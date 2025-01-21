//go:build integration
// +build integration

package docker

import (
	"balance/internal/apiService"
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSLBDockerStartSanity(t *testing.T) {
	client, err := CreateDockerClient()
	require.NoError(t, err)
	ctx := context.Background()

	imageTags := []string{SlbImageRepo + ImgVersion}

	config := &container.Config{
		Image: imageTags[0],
	}
	containerID, err := CreateContainer(ctx, client, config, nil, strings.ToLower(t.Name()+"-"+uuid.NewString()[:4]))
	require.NoError(t, err)
	require.NoError(t, StartContainer(ctx, client, containerID))

	output, err := GetContainerLogs(ctx, client, containerID)
	t.Log(output)
	require.NoError(t, err)
}

func TestSLBDockerCreateFailedBadImageName(t *testing.T) {
	client, err := CreateDockerClient()
	require.NoError(t, err)
	ctx := context.Background()

	imageTags := []string{""}

	config := &container.Config{
		Image: imageTags[0],
		ExposedPorts: nat.PortSet{
			apiService.DefaultApiPort: struct{}{},
			"5001":                    struct{}{},
		},
	}
	containerID, err := CreateContainer(ctx, client, config, nil, strings.ToLower(t.Name()))
	require.Error(t, err)
	require.Empty(t, containerID)
}

func TestSLBDockerCleanupFailedBadId(t *testing.T) {
	client, err := CreateDockerClient()
	require.NoError(t, err)
	ctx := context.Background()

	id := ""

	err = CleanupContainer(ctx, client, id)
	require.Error(t, err)
	require.Empty(t, id)
}
