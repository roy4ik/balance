package balanceService

import (
	"balance/gen"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	localAddress = "127.0.0.1"
	defaultPort  = "8080"
	serverPort   = ":8081"
)

func setupServer() (*grpc.Server, *BalanceServer) {
	grpcServer := grpc.NewServer()
	balanceServer := &BalanceServer{}
	gen.RegisterBalanceServer(grpcServer, balanceServer)
	return grpcServer, balanceServer
}

func TestConfigurationNoSLBShouldReturnError(t *testing.T) {
	_, balanceServer := setupServer()

	ctx := context.Background()
	_, err := balanceServer.Configuration(ctx, &emptypb.Empty{})

	require.Error(t, err)
	require.Equal(t, ErrNotConfigured, err)
}

func TestConfigurationWithSLBShouldReturnConfig(t *testing.T) {
	_, balanceServer := setupServer()
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Endpoints:     []*gen.Server{{Address: localAddress}},
		Strategy:      gen.SelectorStrategy_SELECTOR_STRATEGY_RANDOM,
	}
	_, err := balanceServer.Configure(context.Background(), slbConfig)
	require.NoError(t, err)
	config, err := balanceServer.Configuration(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.Equal(t, slbConfig.ListenAddress, config.ListenAddress)
	require.Equal(t, slbConfig.ListenPort, config.ListenPort)
	require.Exactly(t, gen.SelectorStrategy_SELECTOR_STRATEGY_RANDOM, config.Strategy)
}

func TestConfigureShouldSetNewConfig(t *testing.T) {
	_, balanceServer := setupServer()
	newConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    "9090",
		Endpoints:     []*gen.Server{{Address: localAddress}},
	}

	_, err := balanceServer.Configure(context.Background(), newConfig)
	require.NoError(t, err)
}

func TestRunNoSLBShouldReturnError(t *testing.T) {
	_, balanceServer := setupServer()

	ctx := context.Background()
	_, err := balanceServer.Run(ctx, &emptypb.Empty{})

	require.Error(t, err)
	require.Equal(t, ErrNotConfigured, err)
}

func TestRunWithSLBShouldRunSuccessfully(t *testing.T) {
	_, balanceServer := setupServer()
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Endpoints:     []*gen.Server{{Address: localAddress}},
	}

	balanceServer.Configure(context.Background(), slbConfig)
	_, err := balanceServer.Run(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
}

func TestStopNoSLBShouldReturnError(t *testing.T) {
	_, balanceServer := setupServer()

	ctx := context.Background()
	_, err := balanceServer.Stop(ctx, &emptypb.Empty{})

	require.Error(t, err)
	require.Equal(t, ErrNotConfigured, err)
}

func TestStopWithSLBShouldStopSuccessfully(t *testing.T) {
	_, balanceServer := setupServer()
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Endpoints:     []*gen.Server{{Address: localAddress}},
	}

	balanceServer.Configure(context.Background(), slbConfig)
	_, err := balanceServer.Stop(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
}

func TestAddNoSelectorShouldReturnError(t *testing.T) {
	_, balanceServer := setupServer()

	ctx := context.Background()
	_, err := balanceServer.Add(ctx, &gen.Server{Address: localAddress + serverPort})

	require.Error(t, err)
	require.Equal(t, ErrNotConfigured, err)
}

func TestAddWithSelectorShouldAddServer(t *testing.T) {
	_, balanceServer := setupServer()
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Strategy:      gen.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN,
	}

	balanceServer.Configure(context.Background(), slbConfig)
	_, err := balanceServer.Add(context.Background(), &gen.Server{Address: localAddress + serverPort})
	require.NoError(t, err)
}

func TestRemoveNoSelectorShouldReturnError(t *testing.T) {
	_, balanceServer := setupServer()

	ctx := context.Background()
	_, err := balanceServer.Remove(ctx, &gen.Server{Address: localAddress + serverPort})

	require.Error(t, err)
	require.Equal(t, ErrNotConfigured, err)
}

func TestRemoveWithSelectorShouldRemoveServer(t *testing.T) {
	_, balanceServer := setupServer()
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Strategy:      gen.SelectorStrategy_SELECTOR_STRATEGY_RANDOM,
	}

	balanceServer.Configure(context.Background(), slbConfig)
	balanceServer.Add(context.Background(), &gen.Server{Address: localAddress + serverPort})
	_, err := balanceServer.Remove(context.Background(), &gen.Server{Address: localAddress + serverPort})
	require.NoError(t, err)
}
