package roundRobin

import (
	"fmt"
	"net/http"
	"sync"
)

// RoundRobin Selects targets sequentially (in a structured order)
type RoundRobin struct {
	mu        *sync.Mutex
	endpoints []*http.Server
	currIdx   int
}

func New() *RoundRobin {
	return &RoundRobin{endpoints: make([]*http.Server, 0), currIdx: 0, mu: &sync.Mutex{}}
}

func (r *RoundRobin) Select() (*http.Server, error) {

	endpoints, err := r.EndPoints()
	if err != nil {
		return nil, err
	}
	if len(endpoints) <= 0 {
		return nil, fmt.Errorf("selector has no endpoints to select")
	}

	// manage index within defer to avoid index change at access
	defer func([]*http.Server) {

		if r.currIdx >= len(r.endpoints)-1 {
			r.currIdx = 0 // reset index
		} else {
			r.currIdx++
		}
	}(endpoints)

	return endpoints[r.currIdx], nil
}

func (r *RoundRobin) EndPoints() ([]*http.Server, error) {
	return r.endpoints, nil
}

func (r *RoundRobin) Add(server *http.Server) error {
	defer r.mu.Unlock()
	r.mu.Lock()
	r.endpoints = append(r.endpoints, server)
	return nil
}

func (r *RoundRobin) Remove(server *http.Server) error {
	defer r.mu.Unlock()
	r.mu.Lock()
	if found, idx := r.findInPool(server, r.endpoints); found {
		r.endpoints = append(r.endpoints[:idx], r.endpoints[idx+1:]...)
		return nil
	}
	return fmt.Errorf("could not find server to delete %+v", server)
}

func (r *RoundRobin) findInPool(server *http.Server, endpoints []*http.Server) (bool, int) {
	found := false
	for i, s := range endpoints {
		if s == server {
			return true, i
		}
	}
	return found, -1
}
