//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSLBDockerStartSanity(t *testing.T) {

	cli, err := createDockerClient()
	require.NoError(t, err)

	imageTags := []string{slbImageRepo + imgVersion}
	ctx := context.Background()

	config := &container.Config{
		Image: imageTags[0],
	}
	containerID, err := createAndStartContainer(ctx, cli, config, strings.ToLower(t.Name()+"-"+uuid.NewString()[:4]))
	require.NoError(t, err)
	t.Cleanup(func() { cleanupContainer(context.Background(), cli, containerID) })

	output, err := getContainerLogs(ctx, cli, containerID)
	t.Log(output)
	require.NoError(t, err)
}

func TestSLBDockerCreateFailedBadImageName(t *testing.T) {
	cli, err := createDockerClient()
	require.NoError(t, err)

	imageTags := []string{""}

	config := &container.Config{
		Image: imageTags[0],
		ExposedPorts: nat.PortSet{
			"443":  struct{}{},
			"5001": struct{}{},
		},
	}
	id, err := createAndStartContainer(context.Background(), cli, config, strings.ToLower(t.Name()))
	require.Error(t, err)
	require.Empty(t, id)
}

func TestSLBDockerCleanupFailedBadId(t *testing.T) {
	cli, err := createDockerClient()
	require.NoError(t, err)

	id := ""

	err = cleanupContainer(context.Background(), cli, id)
	require.Error(t, err)
	require.Empty(t, id)
}

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
