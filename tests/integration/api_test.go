//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	api "balance/gen"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestGRPCSanityNotConfigured(t *testing.T) {
	deployment := NewDockerDeployment(t)
	containerID, certDir := deployment.setup()
	require.NotEmpty(t, containerID)

	ip, err := deployment.getIP(containerID)
	require.NoError(t, err)
	apiClient, err := newApiClient(certDir, ip, HostPort)
	require.NoError(t, err)

	apiCtx, cancelFunc := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancelFunc)
	_, err = apiClient.Configuration(apiCtx, &emptypb.Empty{})
	require.ErrorContains(t, err, "not configured")
}

func TestGrpcConfigureNegativeNoEndpoints(t *testing.T) {
	deployment := NewDockerDeployment(t)
	containerID, certDir := deployment.setup()

	ip, err := deployment.getIP(containerID)
	require.NoError(t, err)
	apiClient, err := newApiClient(certDir, ip, HostPort)
	require.NoError(t, err)

	config := &api.Config{}

	apiCtx, cancelFunc := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancelFunc)
	_, err = apiClient.Configure(apiCtx, config)
	require.ErrorContains(t, err, "no endpoints provided")
}

func TestGrpcConfigureNegativeEndpoints(t *testing.T) {
	deployment := NewDockerDeployment(t)
	containerID, certDir := deployment.setup()

	ip, err := deployment.getIP(containerID)
	require.NoError(t, err)
	apiClient, err := newApiClient(certDir, ip, HostPort)
	require.NoError(t, err)

	endpoints := []*api.Server{{Address: " ="}}
	config := &api.Config{Endpoints: endpoints}

	apiCtx, cancelFunc := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancelFunc)
	_, err = apiClient.Configure(apiCtx, config)
	require.ErrorContains(t, err, "failed to parse server url")
}

func TestGrpcConfigureEndpoints(t *testing.T) {
	deployment := NewDockerDeployment(t)
	containerID, certDir := deployment.setup()

	ip, err := deployment.getIP(containerID)
	require.NoError(t, err)
	apiClient, err := newApiClient(certDir, ip, HostPort)
	require.NoError(t, err)

	// setting this endpoint so the slb itself will be an endpoint asm this address refers to all of its nics
	endpoints := []*api.Server{{Address: "0.0.0.0"}}
	config := &api.Config{Endpoints: endpoints}

	apiCtx, cancelFunc := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancelFunc)
	_, err = apiClient.Configure(apiCtx, config)
	require.NoError(t, err)
}

func TestGrpcConfigureRunStopNoLoad(t *testing.T) {
	deployment := NewDockerDeployment(t)
	containerID, certDir := deployment.setup()

	ip, err := deployment.getIP(containerID)
	require.NoError(t, err)
	apiClient, err := newApiClient(certDir, ip, HostPort)
	require.NoError(t, err)
	endpoints := []*api.Server{{Address: "0.0.0.0"}}
	config := &api.Config{Endpoints: endpoints}

	apiCtx, cancelFunc := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancelFunc)
	_, err = apiClient.Configure(apiCtx, config)
	require.NoError(t, err)

	apiCtx, cancelFunc = context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancelFunc)
	_, err = apiClient.Run(apiCtx, &emptypb.Empty{})
	require.NoError(t, err)

	apiCtx, cancelFunc = context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancelFunc)
	_, err = apiClient.Stop(apiCtx, &emptypb.Empty{})
	require.NoError(t, err)
}
