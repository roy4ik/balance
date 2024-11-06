//go:build integration
// +build integration

//go:generate make -C ../.. balance-docker

package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	apiService "balance/services/api_service"
	backendServer "balance/tests/integration/mock/backend/server"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	imgVersion        = ":latest"
	slbImageRepo      = "balance"
	BackendImgVersion = ":latest"
	BackEndImgName    = "backend"
)

var buildContextPaths = []string{"../../"}

func createDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}


func createContainer(ctx context.Context, cli *client.Client, config *container.Config, containerName string) (string, error) {
	// Create host configuration with port mapping
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			apiService.DefaultApiPort + "/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: HostPort,
				},
			},
			"8080/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: backendServer.ListenPort,
				},
			},
		},
	}
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("error creating Docker container: %v", err)
	}
	return resp.ID, nil
}

func startContainer(cli *client.Client, ctx context.Context, containerID string) error {
	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("error starting Docker container: %v", err)
	}
	return nil
}

func stopContainer(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		slog.Error(fmt.Sprintf("error removing Docker container: %v", err))
		return err
	}
	cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	return nil
}

func cleanupContainer(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
		slog.Error(fmt.Sprintf("error removing Docker container: %v", err))
		return err
	}
	cli.ContainerWait(ctx, containerID, container.WaitConditionRemoved)
	return nil
}

func getContainerLogs(ctx context.Context, cli *client.Client, containerID string) (string, error) {
	out, err := cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", fmt.Errorf("error fetching Docker container logs: %v", err)
	}
	defer out.Close()

	stdout, err := io.ReadAll(out)
	if err != nil {
		return "", fmt.Errorf("error reading Docker container logs: %v", err)
	}
	return string(stdout), nil
}

func getContainerIP(containerID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	return inspect.NetworkSettings.IPAddress, nil
}

func listImages(ctx context.Context, cli *client.Client, imageTags []string) ([]types.ImageSummary, error) {
	args := filters.NewArgs()
	for _, tag := range imageTags {
		args.Add("reference", tag)
	}

	images, err := cli.ImageList(ctx, types.ImageListOptions{
		Filters: args,
	})
	if err != nil {
		return images, fmt.Errorf("error listing Docker images: %v", err)
	}
	return images, err
}

func waitForImage(ctx context.Context, cli *client.Client, imageTags []string, timeout time.Duration) error {
	timeoutChan := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	args := filters.NewArgs()
	for _, tag := range imageTags {
		args.Add("reference", tag)
	}

	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("timeout waiting for image %s to be available", imageTags)
		case <-ticker.C:
			images, err := listImages(ctx, cli, imageTags)
			if err != nil {
				return err
			}
			if len(images) > 0 {
				return nil
			}

		}
	}
}
