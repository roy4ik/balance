//go:build integration
// +build integration

package integration

import (
	api "balance/gen"
	apiService "balance/services/api_service"
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newApiClient(serverAddress string, port string) (api.BalanceClient, error) {
	conn, err := grpc.NewClient(serverAddress+":"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	return api.NewBalanceClient(conn), err
}

func setup(t *testing.T, name string) (context.Context, *client.Client, string) {
	ctx := context.Background()

	cli, err := createDockerClient()
	require.NoError(t, err)

	imageTags := []string{slbImageRepo + imgVersion}
	config := &container.Config{
		Image: imageTags[0],
		ExposedPorts: nat.PortSet{
			nat.Port(HostPort): struct{}{},
			backendListenPort:  struct{}{},
		},
	}
	containerID, err := createAndStartContainer(ctx, cli, config, strings.ToLower(t.Name())+"-"+strings.ToLower(name))
	require.NoError(t, err)

	return ctx, cli, containerID
}

func setupSlbWithBackends(t *testing.T, numBackends int) (context.Context, *client.Client, string, []string) {
	// setup slb
	ctx, cli, containerID := setup(t, "slb"+"-"+uuid.NewString()[:4])
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
				apiService.DefaultApiPort: struct{}{},
				backendListenPort:         struct{}{},
			},
		}
		backendContainerID, err := createAndStartContainer(ctx, cli, config, strings.ToLower(t.Name())+"-"+"backend-"+uuid.NewString()[:4])
		require.NoError(t, err)
		// required is the shortened id here as the backend provides the full id.
		backendContainers = append(backendContainers, backendContainerID[:11])
	}
	return ctx, cli, containerID, backendContainers
}
