package apiService

import (
	api "balance/gen"
	"balance/services/slb"
	randomSelector "balance/services/slb/selectors/random"
	"balance/services/slb/selectors/roundRobin"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

const DefaultApiPort = "443"
const DefaultCertsDirectory = "/etc/certs/"
const DefaultCAPath = DefaultCertsDirectory + "ca-cert.pem"
const DefaultServiceCertPath = DefaultCertsDirectory + "service-cert.pem"
const DefaultServiceKeyPath = DefaultCertsDirectory + "service-key.pem"

var ErrNotConfigured = fmt.Errorf("slb not configured correctly")

type balanceServer struct {
	api.UnimplementedBalanceServer
	selector slb.Selector
	slb      *slb.Slb
}

func (b *balanceServer) Configuration(ctx context.Context, _ *emptypb.Empty) (*api.Config, error) {
	if b.slb == nil {
		return nil, ErrNotConfigured
	}
	cfg := b.slb.Configuration()
	endpoints := []*api.Server{}
	for _, endpoint := range cfg.Endpoints {
		endpoints = append(endpoints, &api.Server{
			Address: endpoint.Addr,
		})
	}
	strategy := api.SelectorStrategy_SELECTOR_STRATEGY_UNSPECIFIED
	t := reflect.TypeOf(b.selector)
	if strings.Contains(api.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN.String(), t.Name()) {
		strategy = api.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN
	}
	if strings.Contains(api.SelectorStrategy_SELECTOR_STRATEGY_RANDOM.String(), t.Name()) {
		strategy = api.SelectorStrategy_SELECTOR_STRATEGY_RANDOM
	}
	if strategy == api.SelectorStrategy_SELECTOR_STRATEGY_UNSPECIFIED {
		slog.Warn("No strategy configured")
	}

	return &api.Config{
		Endpoints:     endpoints,
		ListenPort:    cfg.ListenPort,
		ListenAddress: cfg.ListenAddress,
		HandlePostfix: cfg.HandlePostfix,
		Strategy:      strategy,
	}, nil
}

func (b *balanceServer) Configure(ctx context.Context, config *api.Config) (*emptypb.Empty, error) {
	if b.slb != nil {
		slog.Info(fmt.Sprintf("Stopping server with previous configuration %v", b.slb.Configuration()))
		if err := b.slb.Stop(); err != nil {
			return &emptypb.Empty{}, err
		}
	}

	slog.Info(fmt.Sprintf("Setting new configuration: %v", config))
	newConfig := slb.Config{
		Endpoints:     make([]*http.Server, 0),
		ListenAddress: config.ListenAddress,
		ListenPort:    config.ListenPort,
		HandlePostfix: config.HandlePostfix,
	}
	for _, server := range config.Endpoints {
		newConfig.Endpoints = append(newConfig.Endpoints, &http.Server{Addr: server.Address})
	}

	switch config.Strategy {
	case api.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN:
		b.selector = roundRobin.New()
	case api.SelectorStrategy_SELECTOR_STRATEGY_RANDOM:
		b.selector = randomSelector.New()
	default:
		b.selector = roundRobin.New()
	}

	slb, err := slb.New(newConfig, b.selector)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	b.slb = slb
	return &emptypb.Empty{}, nil
}

func (b *balanceServer) Run(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if b.slb == nil {
		return nil, ErrNotConfigured
	}
	go b.slb.Run()
	slog.Info("Running")
	return req, nil
}

func (b *balanceServer) Stop(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if b.slb == nil {
		return nil, ErrNotConfigured
	}
	return req, b.slb.Stop()
}

func (b *balanceServer) Add(ctx context.Context, server *api.Server) (*emptypb.Empty, error) {
	if b.selector == nil {
		return nil, ErrNotConfigured
	}
	s := &http.Server{Addr: server.Address}
	return &emptypb.Empty{}, b.selector.Add(s)
}

func (b *balanceServer) Remove(ctx context.Context, server *api.Server) (*emptypb.Empty, error) {
	if b.selector == nil {
		return nil, ErrNotConfigured
	}
	s := &http.Server{Addr: server.Address}
	return &emptypb.Empty{}, b.selector.Remove(s)
}

func getTlsCredentials(caPath string, serviceCertPath string, serviceKeyPath string) (credentials.TransportCredentials, error) {
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate:" + err.Error())
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to cert pool")
	}

	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(serviceCertPath, serviceKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate and key:" + err.Error())
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert, // RequireAndVerifyFor mTLS
	}

	// Create gRPC server with TLS credentials
	creds := credentials.NewTLS(tlsConfig)
	return creds, nil
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

func NewApiServer() *ApiServer {
	creds, err := getTlsCredentials(DefaultCAPath, DefaultServiceCertPath, DefaultServiceKeyPath)
	if err != nil {
		panic(err)
	}
	return &ApiServer{
		Server: NewGrpcServer(creds),
		Port:   DefaultApiPort,
	}
}

func NewGrpcServer(creds credentials.TransportCredentials) *grpc.Server {
	s := grpc.NewServer(grpc.Creds(creds))
	slbServer := &balanceServer{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	// Configure with default values
	slbServer.Configure(ctx, &api.Config{
		Strategy:      api.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN,
		ListenAddress: slb.DefaultListenAddress,
		ListenPort:    slb.DefaultListenPort,
		HandlePostfix: slb.DefaultHandlePostfix,
	})
	api.RegisterBalanceServer(s, slbServer)
	reflection.Register(s)
	return s
}
