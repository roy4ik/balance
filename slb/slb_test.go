package slb

import (
	"balance/internal/mock"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

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
		slog.Info("backend received request")
		return &http.Response{
			StatusCode: s.expectedResponse.StatusCode,
			Body:       s.expectedResponse.Body}, nil
	})
	proxyHandler := httputil.NewSingleHostReverseProxy(url)
	proxyHandler.Transport = transport
	return &http.Server{Handler: proxyHandler}, s.err
}

func (s slbTest) Run() {
	s.testFunc(s.t)
}

const badParseString = "Http:// ="

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
				listenAddress := "localhost"

				expectedRespBody := "OK"
				expectedRespStatus := http.StatusOK

				// initiate SLB with localhost as listen address as this is a local test
				slb, err := New(
					Config{Endpoints: mockServers, ListenPort: listenPort, ListenAddress: listenAddress},
					&SelectorMock{
						expectedResponse: http.Response{
							StatusCode: expectedRespStatus,
							Body:       io.NopCloser(strings.NewReader(expectedRespBody))},
					})
				require.NoError(t, err)

				// run SLB
				var runErr = make(chan error)
				go func(t *testing.T, slb *Slb) {
					defer close(runErr)
					runErr <- slb.Run()
				}(t, slb)
				checkRunError := func() {
					deadline, _ := t.Deadline()
					ctx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
					defer cancel()
					for {
						select {
						case err := <-runErr:
							require.NoError(t, err)
							return
						case <-ctx.Done():
							return
						default:
							<-time.After(time.Millisecond * 10)
						}
					}
				}
				go checkRunError()
				// tiny wait for the slb to start
				<-time.After(time.Millisecond * 250)
				// send request to SLB
				sendRequest := func() {
					targetUrl := "http://" + string(listenAddress) + ":" + listenPort + "/food"
					slog.Info("sending request to " + targetUrl)
					resp, err := http.Get(targetUrl)
					require.NoError(t, err)
					defer resp.Body.Close()

					// validate response
					body, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					require.Equal(t, expectedRespStatus, resp.StatusCode)
					require.Equal(t, expectedRespBody, string(body))
				}

				sendRequest()

			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) { scenario.Run() })
	}
}
