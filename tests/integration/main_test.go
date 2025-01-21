//go:build integration
// +build integration

package integration

import (
	"os"
	"os/exec"
	"testing"
)

// This runs generate on integration tets, so that the test preconditions
// are run when the test is run
func TestMain(m *testing.M) {
	// Run setup tasks here, e.g., go generate
	if err := runGenerate(); err != nil {
		os.Exit(1)
	}
	// Run tests
	exitVal := m.Run()
	os.Exit(exitVal)
}

func runGenerate() error {
	cmd := exec.Command("go", "generate", "-tags=integration", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
