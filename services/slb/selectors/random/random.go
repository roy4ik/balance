package randomSelector

import (
	"fmt"
	"net/http"
	"sync"
)

// Random Selects targets sequentially (in a structured order)
type Random struct {
	mu        *sync.Mutex
	endpoints map[string]*http.Server
}

func New() *Random {
	return &Random{endpoints: make(map[string]*http.Server, 0), mu: &sync.Mutex{}}
}

func (r *Random) Select() (*http.Server, error) {
	// first value returns random item, since it retrieves from a hash map
	for _, v := range r.endpoints {
		return v, nil
	}

	return nil, fmt.Errorf("selector has no endpoints to select")
}

func (r *Random) EndPoints() ([]*http.Server, error) {
	eList := make([]*http.Server, 0)
	for _, v := range r.endpoints {
		eList = append(eList, v)
	}
	return eList, nil
}

func (r *Random) Add(server *http.Server) error {
	defer r.mu.Unlock()
	r.mu.Lock()
	r.endpoints[server.Addr] = server
	return nil
}

func (r *Random) Remove(server *http.Server) error {
	defer r.mu.Unlock()
	r.mu.Lock()
	if _, ok := r.endpoints[server.Addr]; ok {
		delete(r.endpoints, server.Addr)
		return nil
	}
	return fmt.Errorf("could not find server to delete %+v", server)
}
