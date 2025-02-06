package apiService

import (
	"balance/gen"
	"context"
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	localAddress = "127.0.0.1"
	defaultPort  = "8080"
	serverPort   = ":8081"
)

type mockClient struct {
	gen.BalanceClient
	config *gen.Config
}

func (c *mockClient) Configuration(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*gen.Config, error) {
	return c.config, nil
}

func (c *mockClient) Configure(ctx context.Context, in *gen.Config, opts ...grpc.CallOption) (*empty.Empty, error) {
	c.config = in
	return nil, nil
}

func (c *mockClient) Run(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, nil
}

func (c *mockClient) Stop(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, nil
}
func (c *mockClient) Add(ctx context.Context, in *gen.Server, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, nil
}
func (c *mockClient) Remove(ctx context.Context, in *gen.Server, opts ...grpc.CallOption) (*empty.Empty, error) {
	return nil, nil
}

func setupServer(client gen.BalanceClient) (*grpc.Server, *BalanceServer) {
	grpcServer := grpc.NewServer()
	balanceServer := &BalanceServer{Client: client}
	gen.RegisterBalanceServer(grpcServer, balanceServer)
	return grpcServer, balanceServer
}

func TestConfigureShouldSetNewConfig(t *testing.T) {
	_, apiService := setupServer(&mockClient{})
	newConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    "9090",
		Endpoints:     []*gen.Server{{Address: localAddress}},
	}

	_, err := apiService.Configure(context.Background(), newConfig)
	require.NoError(t, err)
}

func TestRunWithSLBShouldRunSuccessfully(t *testing.T) {
	_, apiService := setupServer(&mockClient{})
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Endpoints:     []*gen.Server{{Address: localAddress}},
	}

	apiService.Configure(context.Background(), slbConfig)
	_, err := apiService.Run(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
}

func TestRunWithSLBShouldFailRunWithoutEndpoints(t *testing.T) {
	_, apiService := setupServer(&mockClient{})
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Endpoints:     []*gen.Server{},
	}

	apiService.Configure(context.Background(), slbConfig)
	_, err := apiService.Run(context.Background(), &emptypb.Empty{})
	require.Error(t, err)
}

func TestStopWithSLBShouldStopSuccessfully(t *testing.T) {
	_, apiService := setupServer(&mockClient{})
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Endpoints:     []*gen.Server{{Address: localAddress}},
	}

	apiService.Configure(context.Background(), slbConfig)
	_, err := apiService.Stop(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
}

func TestAddWithSelectorShouldAddServer(t *testing.T) {
	_, apiService := setupServer(&mockClient{})
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Strategy:      gen.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN,
	}

	apiService.Configure(context.Background(), slbConfig)
	_, err := apiService.Add(context.Background(), &gen.Server{Address: localAddress + serverPort})
	require.NoError(t, err)
}

func TestRemoveWithSelectorShouldRemoveServer(t *testing.T) {
	_, apiService := setupServer(&mockClient{})
	slbConfig := &gen.Config{
		ListenAddress: localAddress,
		ListenPort:    defaultPort,
		Strategy:      gen.SelectorStrategy_SELECTOR_STRATEGY_RANDOM,
	}

	apiService.Configure(context.Background(), slbConfig)
	apiService.Add(context.Background(), &gen.Server{Address: localAddress + serverPort})
	_, err := apiService.Remove(context.Background(), &gen.Server{Address: localAddress + serverPort})
	require.NoError(t, err)
}
