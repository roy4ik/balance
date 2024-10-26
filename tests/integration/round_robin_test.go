//go:build integration
// +build integration

package integration

import (
	api "balance/gen"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestRoundRobinSanity(t *testing.T) {
	ctx, cli, slbContainerID, backendIDs := setupSlbWithBackends(t, 4)
	t.Cleanup(func() {
		time.Sleep(1)
		o, _ := getContainerLogs(ctx, cli, slbContainerID)
		t.Log(o)
		cleanupContainer(context.Background(), cli, slbContainerID)
		for _, backend := range backendIDs {
			cleanupContainer(context.Background(), cli, backend)
		}
	})
	slbIp, err := getContainerIP(slbContainerID)
	require.NoError(t, err)
	// get backend ids to ips
	config := &api.Config{
		Endpoints:  make([]*api.Server, 0),
		ListenPort: backendListenPort,
		// ListenAddress: slbIp,
	}
	for _, id := range backendIDs {
		ip, err := getContainerIP(id)
		require.NoError(t, err)
		require.NotEmpty(t, ip)
		s := &api.Server{Address: ip}
		config.Endpoints = append(config.Endpoints, s)
	}
	apiClient, err := newApiClient(slbIp, "443")
	require.NoError(t, err)
	_, err = apiClient.Configure(ctx, config)
	require.NoError(t, err)
	_, err = apiClient.Run(ctx, &emptypb.Empty{})
	require.NoError(t, err)

	for _, id := range backendIDs {
		// send request to backend to verify that the backend responds
		backEndIp, _ := getContainerIP(id)
		backEndUrl := "http://" + backEndIp + ":" + config.ListenPort
		res, err := http.Get(backEndUrl)
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		backendRespId := string(body[:11])
		require.Exactly(t, id, backendRespId)

		// send request via slb and retrieve the container id of the backend.
		slbUrl := "http://" + slbIp + ":" + config.ListenPort
		res, err = http.Get(slbUrl)
		require.NoError(t, err)
		body, err = io.ReadAll(res.Body)
		require.NoError(t, err)
		nlbRespID := string(body[:11])
		require.Exactly(t, id, nlbRespID)
		require.Exactly(t, backendRespId, nlbRespID)
	}
}

const (
	BackEndImgName    = "backend:"
	BackendImgVersion = "latest"
)

func setupSlbWithBackends(t *testing.T, numBackends int) (context.Context, *client.Client, string, []string) {
	// setup slb
	ctx, cli, containerID := setup(t, "round-robin"+"-"+uuid.NewString()[:4])
	// setup backends
	backendContainers := make([]string, 0)
	for i := 0; i < numBackends; i++ {
		ctx := context.Background()

		cli, err := createDockerClient()
		require.NoError(t, err)

		imageTags := []string{BackEndImgName + BackendImgVersion}
		config := &container.Config{
			Image: imageTags[0],
			ExposedPorts: nat.PortSet{
				"443":             struct{}{},
				backendListenPort: struct{}{},
			},
		}
		backendContainerID, err := createAndStartContainer(ctx, cli, config, strings.ToLower(t.Name())+"-"+"backend-"+uuid.NewString()[:4])
		require.NoError(t, err)
		// required is the shortened id here as the backend provides the full id.
		backendContainers = append(backendContainers, backendContainerID[:11])
	}
	return ctx, cli, containerID, backendContainers
}
