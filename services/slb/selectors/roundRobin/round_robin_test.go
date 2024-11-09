package roundRobin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"balance/internal/mock"
)

type RRTest struct {
	name        string
	t           *testing.T
	endpoints   []*http.Server
	nSelections int
	toSelect    *http.Server
}

func (r *RRTest) Run() {
	selector := New()
	selected, err := selector.Select()
	require.Error(r.t, err, "expected no endpoints to select")
	for _, e := range r.endpoints {
		require.NoError(r.t, selector.Add(e))
	}

	for nSelection := 1; nSelection <= r.nSelections; nSelection++ {
		selected, err = selector.Select()
		require.NoError(r.t, err)
	}
	if r.toSelect != nil {
		require.Exactly(r.t, selected, r.toSelect)
	}

	for _, e := range r.endpoints {
		require.NoError(r.t, selector.Remove(e))
	}
	require.Error(r.t, selector.Remove(selected), "all endoints removed, expected error")
}

func TestRoundRobin(t *testing.T) {
	scenarios := []*RRTest{}
	servers := mock.GenerateServers(3)
	scenarioWithinRange := &RRTest{"test selection within endpoint range", t, servers, 1, servers[0]}
	scenarioOutSideRange := &RRTest{"selection outside of endpoint range", t, servers, 5, servers[1]}

	scenarios = append(scenarios, scenarioWithinRange, scenarioOutSideRange)
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) { scenario.Run() })
	}
}
