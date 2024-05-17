package slb

import (
	"balance/pkg/mock"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type slbTest struct {
	name     string
	t        *testing.T
	testFunc func(t *testing.T)
}

type SelectorMock struct {
	Selector
	expectedResponse http.Response
	err              error
}

func (s *SelectorMock) Add(*http.Server) error {
	return nil
}

func (s *SelectorMock) Remove(*http.Server) error {
	return nil
}

func (s *SelectorMock) Select() (*http.Server, error) {
	url := &url.URL{}
	transport := mock.TransPortResponseFunc(func(req *http.Request) (*http.Response, error) {
		r := &s.expectedResponse
		defer r.Body.Close()
		slog.Info("backend received request")
		return r, nil
	})
	proxyHandler := httputil.NewSingleHostReverseProxy(url)
	proxyHandler.Transport = transport
	return &http.Server{Handler: proxyHandler}, s.err
}

func (s slbTest) Run() {
	s.testFunc(s.t)
}

const badParseString = " ="

func TestSLB(t *testing.T) {
	scenarios := []*slbTest{
		{
			name: "Config: empty config",
			t:    t,
			testFunc: func(t *testing.T) {
				_, err := New(Config{}, &SelectorMock{})
				require.Error(t, err)
				require.Exactly(t, err, ErrConfigNoEnpoints())
			},
		},
		{
			name: "New: No Selector",
			t:    t,
			testFunc: func(t *testing.T) {
				mockServers := mock.GenerateServers(3)
				_, err := New(Config{Endpoints: mockServers}, nil)
				require.Error(t, err)
				require.Exactly(t, err, ErrNoSelector())
			},
		},
		{
			name: "Config: Bad Listen Address",
			t:    t,
			testFunc: func(t *testing.T) {
				mockServers := mock.GenerateServers(1)
				_, err := New(Config{Endpoints: mockServers, ListenAddress: badParseString}, &SelectorMock{})
				require.Error(t, err)
				require.Containsf(t, err.Error(), ErrFailedToParseServerUrl(fmt.Errorf("")).Error(), "")
			},
		},
		{
			name: "Config: Bad Backened Address",
			t:    t,
			testFunc: func(t *testing.T) {
				mockServers := mock.GenerateServers(1)
				for _, m := range mockServers {
					m.Addr = badParseString
				}
				_, err := New(Config{Endpoints: mockServers}, &SelectorMock{})
				require.Error(t, err)
				require.Containsf(t, err.Error(), ErrFailedToParseServerUrl(fmt.Errorf("")).Error(), "")
			},
		},
		{
			name: "Run Happy Flow",
			t:    t,
			testFunc: func(t *testing.T) {
				// Generate mock servers
				mockServers := mock.GenerateServers(3)
				// Chose a random but realistic port range
				listenPort := mock.RandomPort()
				listenAddress := &net.IPAddr{IP: []byte("localhost")}

				expectedRespBody := "OK"
				expectedRespStatus := http.StatusOK
				// initiate SLB
				slb, err := New(
					Config{Endpoints: mockServers, ListenPort: listenPort}, &SelectorMock{
						expectedResponse: http.Response{
							StatusCode: expectedRespStatus,
							Body:       io.NopCloser(strings.NewReader(expectedRespBody))},
					})
				require.NoError(t, err)

				// run SLB
				var runErr = make(chan error, 1)
				go func(t *testing.T, slb *Slb) {
					defer close(runErr)
					runErr <- slb.Run()
					require.NoError(t, err)
				}(t, slb)

				// send request to SLB
				targetUrl := "http://" + string(listenAddress.IP) + ":" + listenPort + "/food"
				slog.Info("sending request to " + targetUrl)
				resp, err := http.Get(targetUrl)
				require.NoError(t, err)
				defer resp.Body.Close()

				// validate response
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, string(body), expectedRespBody)
				require.Equal(t, resp.StatusCode, expectedRespStatus)
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) { scenario.Run() })
	}
}
