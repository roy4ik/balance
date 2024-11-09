//go:generate protoc --go_out=.. --go-grpc_out=.. ./api/slb.proto
package slb

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"
)

var (
	ErrSelectionFailed = func(err error) error { return fmt.Errorf("could not select server: %s", err) }
	ErrFailedSetProxy  = func(err error) error { return fmt.Errorf("failed to set server proxy: %s", err) }
	ErrNoSelector      = func() error { return fmt.Errorf("no selector provided") }
)

// SoftwareLoadBalance implements the http.Handler interface,
// so that it itself can be used as a handler with http.Server
// example: s := &http.Server{Handle: New(<cfg>, <selector>)}
type SoftwareLoadBalancer interface {
	http.Handler
}

type Slb struct {
	cfg      Config
	selector Selector
	serveMux *http.ServeMux
	server   *http.Server
	SoftwareLoadBalancer
}

// Returns a new SoftwareLoadBalancer with validated configuration and set endpoints (and error)
func New(config Config, selector Selector) (*Slb, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if selector == nil {
		return nil, ErrNoSelector()
	}

	s := &Slb{
		cfg: config,
	}
	s.selector = selector

	setServerProxy := func(server *http.Server, listenPort string) error {
		url, err := resolveAddress(server.Addr, listenPort)
		if err != nil {
			return err
		}
		server.Addr = url.String()

		proxyHandler := httputil.NewSingleHostReverseProxy(url)
		server.Handler = proxyHandler
		return nil
	}

	for _, server := range s.cfg.Endpoints {
		if err := s.selector.Add(server); err != nil {
			return nil, err
		}
		if err := setServerProxy(server, config.ListenPort); err != nil {
			return nil, ErrFailedSetProxy(err)
		}
	}

	s.serveMux = http.NewServeMux()
	s.serveMux.Handle(s.cfg.Postfix(), s)
	s.server = &http.Server{Addr: s.cfg.Address(), Handler: s.serveMux}
	return s, nil
}

// Runs the SLB with a server that listens to requests on the ListenAddress, and ListenPort.
// The server is proxying the requests to the backend servers.
func (s *Slb) Run() error {
	defer s.server.Close()

	slog.Info("SLB started at: " + s.server.Addr + s.cfg.Postfix())

	err := s.server.ListenAndServe()
	if err != nil {
		slog.Error(err.Error())
	}
	return err
}

// ServeHTTP wraps the endpoint selection and backend ServerHTTP call so that it can be used as a http.HandlerFunc / by server Mux
func (s *Slb) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	server, err := s.selector.Select()
	if err != nil {
		slog.Error(ErrSelectionFailed(err).Error())
	}
	server.Handler.ServeHTTP(rw, r)
}

// Gracefully stops the SLB server, if it cannot gracefully shut down, it will stop it immediately
func (s *Slb) Stop() error {
	defer s.server.Close()
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	slog.Info("SLB stopping")
	return s.server.Shutdown(ctx)
}

// returns the current configuration of the SLB with updated endpoints
func (s *Slb) Configuration() Config {
	cfg := s.cfg
	var err error
	cfg.Endpoints, err = s.selector.EndPoints()
	if err != nil {
		slog.Error("could not update endpoints list")
	}
	return s.cfg
}
