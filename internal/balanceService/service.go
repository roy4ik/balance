//go:generate protoc --proto_path=../../api --go_out=.. --go-grpc_out=.. balance.proto
package balanceService

import (
	api "balance/gen"
	randomSelector "balance/internal/selectors/random"
	"balance/internal/selectors/roundRobin"
	"balance/slb"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"

	"google.golang.org/protobuf/types/known/emptypb"
)

const DefaultAddress = "0.0.0.0"
const DefaultPort = "8080"

var ErrNotConfigured = fmt.Errorf("slb not configured correctly")

type BalanceServer struct {
	api.UnimplementedBalanceServer
	selector slb.Selector
	slb      *slb.Slb
}

func (b *BalanceServer) Configuration(ctx context.Context, _ *emptypb.Empty) (*api.Config, error) {
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
	if reflect.TypeOf(&roundRobin.RoundRobin{}) == t {
		strategy = api.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN
	}
	if reflect.TypeOf(&randomSelector.Random{}) == t {
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

func (b *BalanceServer) Configure(ctx context.Context, config *api.Config) (*emptypb.Empty, error) {
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

func (b *BalanceServer) Run(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if b.slb == nil {
		return nil, ErrNotConfigured
	}
	go b.slb.Run()
	slog.Info("Running ")
	return req, nil
}

func (b *BalanceServer) Stop(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if b.slb == nil {
		return nil, ErrNotConfigured
	}
	return req, b.slb.Stop()
}

func (b *BalanceServer) Add(ctx context.Context, server *api.Server) (*emptypb.Empty, error) {
	if b.selector == nil {
		return nil, ErrNotConfigured
	}
	s := &http.Server{Addr: server.Address}
	return &emptypb.Empty{}, b.selector.Add(s)
}

func (b *BalanceServer) Remove(ctx context.Context, server *api.Server) (*emptypb.Empty, error) {
	if b.selector == nil {
		return nil, ErrNotConfigured
	}
	s := &http.Server{Addr: server.Address}
	return &emptypb.Empty{}, b.selector.Remove(s)
}

func NewBalanceService() *BalanceServer {
	slbServer := &BalanceServer{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	// Configure with default values
	slbServer.Configure(ctx, &api.Config{
		Strategy:      api.SelectorStrategy_SELECTOR_STRATEGY_ROUND_ROBIN,
		ListenAddress: DefaultAddress,
		ListenPort:    DefaultPort,
		HandlePostfix: slb.DefaultHandlePostfix,
	})
	return slbServer
}
