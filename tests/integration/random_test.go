//go:build integration
// +build integration

package integration

import (
	api "balance/gen"
	"balance/internal/apiService"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestRandomSanity(t *testing.T) {
	deployment := NewDockerDeployment(t)
	slbContainerID, certDir := deployment.setup()
	backendIDs := deployment.setupBackends(4, certDir)
	slbIp, err := deployment.getIP(slbContainerID)
	require.NoError(t, err)
	// get backend ids to ips
	config := &api.Config{
		Strategy:   api.SelectorStrategy_SELECTOR_STRATEGY_RANDOM,
		Endpoints:  make([]*api.Server, 0),
		ListenPort: backendListenPort,
		// ListenAddress: slbIp,
	}
	for _, id := range backendIDs {
		ip, err := deployment.getIP(id)
		require.NoError(t, err)
		require.NotEmpty(t, ip)
		s := &api.Server{Address: ip}
		config.Endpoints = append(config.Endpoints, s)
	}
	apiClient, err := newApiClient(certDir, slbIp, apiService.DefaultApiPort)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*30)
	t.Cleanup(cancel)
	_, err = apiClient.Configure(ctx, config)
	require.NoError(t, err)
	_, err = apiClient.Run(ctx, &emptypb.Empty{})
	require.NoError(t, err)

	var selectedBackendIds []string
	shortenedBackendIDs := make([]string, 0)
	for _, id := range backendIDs {
		shortenedBackendIDs = append(shortenedBackendIDs, id[:12])
	}
	for range backendIDs {
		// send request via slb and retrieve the container id of the backend.
		slbUrl := "http://" + slbIp + ":" + config.ListenPort
		res, err := http.Get(slbUrl)
		require.NoError(t, err)
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		nlbRespID := string(body)
		require.Contains(t, shortenedBackendIDs, nlbRespID)
		selectedBackendIds = append(selectedBackendIds, nlbRespID)
	}

	require.NotEqualValues(t, backendIDs, selectedBackendIds, "Backends selected sequentially")
}
