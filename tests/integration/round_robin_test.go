//go:build integration
// +build integration

package integration

import (
	api "balance/gen"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

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
