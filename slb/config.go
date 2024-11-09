package slb

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
)

const (
	DefaultListenPort    = "80"
	DefaultHandlePostfix = "/"
	DefaultListenAddress = "0.0.0.0"
	DefaultScheme        = "http://"
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
func (c *Config) Address() string {
	addr := c.ListenAddress
	if c.ListenAddress == "" {
		addr = DefaultListenAddress
	}
	addr = addr + ":" + resolvePort(c.ListenPort)
	return addr
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
		if _, err := resolveAddress(server.Addr, c.ListenPort); err != nil {
			return err
		}
	}
	if _, err := resolveAddress(c.ListenAddress, c.ListenPort); err != nil {
		return err
	}
	return nil
}

func (c *Config) hasEndpoints() bool {
	return 0 < len(c.Endpoints)
}

func resolveAddress(addr string, listenPort string) (*url.URL, error) {
	// Resolve the TCP address
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr+":"+resolvePort(listenPort))
	if err != nil {
		return nil, ErrFailedToParseServerUrl(fmt.Errorf("failed to resolve address: %s", err))
	}

	parsedURL, err := url.Parse(DefaultScheme + tcpAddr.IP.String() + ":" + resolvePort(listenPort))
	if err != nil {
		return nil, ErrFailedToParseServerUrl(err)
	}
	// Return the parsed URL if everything is successful
	return parsedURL, nil
}

func resolvePort(listenPort string) string {
	if listenPort == "" {
		listenPort = DefaultListenPort
	}
	return listenPort
}
