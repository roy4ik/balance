package mock

import (
	"fmt"
	"math/rand"
	"net/http"
)

func RandomAddress() string {
	return fmt.Sprintf("%d.%d.%d.%d", rand.Intn(254), rand.Intn(254), rand.Intn(254), rand.Intn(254))
}

func RandomPort() string {
	return fmt.Sprint(1024 + rand.Intn(50000))
}

func newServer(addr string) *http.Server {
	s := &http.Server{
		Addr:    addr,
		Handler: http.NewServeMux(),
	}
	return s
}

func GenerateServers(n int) []*http.Server {
	servers := []*http.Server{}
	for curr := 0; curr < n; curr++ {
		s := newServer(RandomAddress())
		servers = append(servers, s)
	}

	return servers
}

type TransPortResponseFunc func(req *http.Request) (*http.Response, error)

func (fn TransPortResponseFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
