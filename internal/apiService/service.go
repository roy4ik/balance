//go:generate protoc --proto_path=../../api --go_out=.. --go-grpc_out=.. balance.proto
package apiService

import (
	api "balance/gen"
	"context"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

const DefaultAddress = "127.0.0.1"
const DefaultApiPort = "443"

type BalanceServer struct {
	api.UnimplementedBalanceServer
	Client api.BalanceClient
}

func (b *BalanceServer) Configuration(ctx context.Context, in *emptypb.Empty) (*api.Config, error) {
	return b.Client.Configuration(ctx, in)
}

func (b *BalanceServer) Configure(ctx context.Context, config *api.Config) (*emptypb.Empty, error) {
	return b.Client.Configure(ctx, config)
}

func (b *BalanceServer) Run(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	config, err := b.Client.Configuration(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(config.Endpoints) < 1 {
		return nil, fmt.Errorf("%v endpoints configured. Use Add to add endpoints", len(config.Endpoints))
	}
	return b.Client.Run(ctx, req)
}

func (b *BalanceServer) Stop(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return b.Client.Stop(ctx, req)
}

func (b *BalanceServer) Add(ctx context.Context, server *api.Server) (*emptypb.Empty, error) {
	return b.Client.Add(ctx, server)
}

func (b *BalanceServer) Remove(ctx context.Context, server *api.Server) (*emptypb.Empty, error) {
	return b.Client.Remove(ctx, server)
}

type ApiServer struct {
	Server *grpc.Server
	Port   string
}

func (a *ApiServer) Start() {
	apiAddress := "" + ":" + a.Port
	grpCListener, err := net.Listen("tcp", apiAddress)
	if err != nil {
		slog.Error("SLB api service failed to listen at " + apiAddress)
		panic(err)
	}
	slog.Info("SLB api service starting at " + apiAddress)
	if err := a.Server.Serve(grpCListener); err != nil {
		slog.Error("SLB api service failed to start at " + apiAddress)
		panic(err)
	}
}

func (a *ApiServer) Stop() {
	defer a.Server.Stop()
	a.Server.GracefulStop()
}

func NewApiServer(creds credentials.TransportCredentials, slbServer api.BalanceServer, port string) *ApiServer {
	return &ApiServer{
		Server: NewGrpcServer(creds, slbServer),
		Port:   port,
	}
}

func NewApiClient(creds credentials.TransportCredentials, serverAddress string, port string) (api.BalanceClient, error) {
	conn, err := grpc.NewClient(serverAddress+":"+port, grpc.WithTransportCredentials(creds))
	return api.NewBalanceClient(conn), err
}

func NewGrpcServer(creds credentials.TransportCredentials, slbServer api.BalanceServer) *grpc.Server {
	s := grpc.NewServer(grpc.Creds(creds))
	api.RegisterBalanceServer(s, slbServer)
	reflection.Register(s)
	return s
}
