package randomSelector

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"balance/pkg/mock"
)

type RRTest struct {
	name        string
	t           *testing.T
	endpoints   []*http.Server
	nSelections int
}

func (r *RRTest) Run() {
	selector := New()

	eList, err := selector.EndPoints()
	require.NoError(r.t, err)
	require.Empty(r.t, eList)
	_, err = selector.Select()
	require.Error(r.t, err)
	for _, e := range r.endpoints {
		require.NoError(r.t, selector.Add(e))
	}
	eList, err = selector.EndPoints()
	require.NoError(r.t, err)
	require.NotEmpty(r.t, eList)

	var selected []*http.Server
	for nSelection := 1; nSelection <= r.nSelections; nSelection++ {
		s, err := selector.Select()
		selected = append(selected, s)
		require.NoError(r.t, err)
	}
	require.NotEmpty(r.t, selected)
	require.True(r.t, len(selected) >= 1, "Expected multiple unique items, but got only one")

	var selectedAgain []*http.Server
	for nSelection := 1; nSelection <= r.nSelections; nSelection++ {
		s, err := selector.Select()
		selectedAgain = append(selectedAgain, s)
		require.NoError(r.t, err)
	}
	require.NotEqualValues(r.t, selected, selectedAgain)

	for _, e := range r.endpoints {
		require.NoError(r.t, selector.Remove(e))
	}
	require.Error(r.t, selector.Remove(&http.Server{}))
}

func TestRandom(t *testing.T) {
	scenarios := []*RRTest{}
	servers := mock.GenerateServers(20)
	scenarioRandomSanity := &RRTest{"Random Sanity", t, servers, 30}

	scenarios = append(scenarios, scenarioRandomSanity)
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) { scenario.Run() })
	}
}
