package slb

import (
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
)

const (
	DefaultListenPort    = "80"
	DefaultHandlePostfix = "/"
	DefaultListenAddress = "localhost"
)

var (
	// Config errros
	ErrConfigNoEnpoints       = func() error { return fmt.Errorf("no endpoints provided") }
	ErrFailedToParseServerUrl = func(err error) error { return fmt.Errorf("failed to parse server url: %s", err) }
)

type Config struct {
	// Load balancer backend endpoints to use
	Endpoints []*http.Server `json:"endpoints"`
	// Network port that the frontend server listens on
	ListenPort string `json:"listenPort,omitempty"`
	// Frontend address (without port)
	ListenAddress string `json:"listenAddress,omitempty"`
	// The address postfix for which the slb forwards requests
	HandlePostfix string `json:"handlePostfix,omitempty"`
}

// Returns the full address with port.
// If no listenPort or ListenAdress are provided in the configuration,
// it will use default values "localhost:80"
func (c *Config) Address() (*url.URL, error) {
	listenPort := c.ListenPort
	if c.ListenPort == "" {
		listenPort = DefaultListenPort
	}
	listenAddress := c.ListenAddress
	if c.ListenAddress == "" {
		listenAddress = DefaultListenAddress
	}
	url, err := url.Parse(listenAddress + ":" + listenPort)
	if err != nil {
		return nil, ErrFailedToParseServerUrl(err)
	}
	return url, nil
}

// Returns the HandlePostix or default "/" if none was provided
func (c *Config) Postfix() string {
	if c.HandlePostfix == "" {
		return DefaultHandlePostfix
	}
	return c.HandlePostfix
}

// Validates the configuration
func (c *Config) Validate() error {
	if !c.hasEndpoints() {
		return ErrConfigNoEnpoints()
	}
	for _, server := range c.Endpoints {
		if _, err := parseEndpointUrl(server); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) hasEndpoints() bool {
	return 0 < len(c.Endpoints)
}

func parseEndpointUrl(server *http.Server) (*url.URL, error) {
	a, err := netip.ParseAddr(server.Addr)
	if err != nil {
		return nil, ErrFailedToParseServerUrl(err)
	}
	url, err := url.Parse(a.String())
	if err != nil {
		return nil, ErrFailedToParseServerUrl(err)
	}
	return url, nil
}
