package slb

import "net/http"

type EndpointsHandler interface {
	Add(*http.Server) error
	Remove(*http.Server) error
}
type Selector interface {
	Select() (*http.Server, error)
	EndpointsHandler
}
