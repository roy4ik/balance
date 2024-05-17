package slb

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
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
	started  bool
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

	setServerProxy := func(server *http.Server) error {
		url, err := parseEndpointUrl(server)
		if err != nil {
			return (err)
		}

		proxyHandler := httputil.NewSingleHostReverseProxy(url)
		server.Handler = proxyHandler
		return nil
	}

	for _, server := range s.cfg.Endpoints {
		if err := s.selector.Add(server); err != nil {
			return nil, err
		}
		if err := setServerProxy(server); err != nil {
			return nil, ErrFailedSetProxy(err)
		}
	}

	s.serveMux = http.NewServeMux()
	s.serveMux.Handle(s.cfg.Postfix(), s)
	url, err := s.cfg.Address()
	if err != nil {
		return nil, err
	}
	s.server = &http.Server{Addr: url.String(), Handler: s.serveMux}
	return s, nil
}

// ServeHTTP wraps the endpoint selection and backend ServerHTTP call so that it can be used as a http.HandlerFunc / by server Mux
func (s *Slb) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	server, err := s.selector.Select()
	if err != nil {
		slog.Error(ErrSelectionFailed(err).Error())
	}
	server.Handler.ServeHTTP(rw, r)
}
