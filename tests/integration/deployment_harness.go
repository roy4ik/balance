//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/google/uuid"
)

type Deployment interface {
	// Setup generally deploys an slb, that should be exposed to
	// the localhost(testing machine).
	// It returns the identifier for the slb (e.g containerID) and the certificate dir path
	// for locally created certificates that can be used to connect to the slb.
	// !!!Note any setup should implement test cleanup for the resources it creates
	setup() (string, string)
	// setupBackends deploys backends that the slb is connected to.
	// The backends should be exposed only to the slb.
	// it returns a list of identifiers for the backends (e.g containerID)
	setupBackends(numBackends int, certDir string) []string
	// receives the deployment identifier (e.g containerID) and returns the IP and error.
	getIP(id string) (string, error)
	// restart deployment instance with identifier (e.g containerID)
	restart(id string)
}
type TestDeployment struct {
	*testing.T
}

func (t *TestDeployment) Name() string {
	return t.T.Name() + "-" + uuid.NewString()[:4]
}

func NewTestDeployment(t *testing.T) *TestDeployment {
	return &TestDeployment{t}
}
