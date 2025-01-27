//go:build integration
// +build integration

package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const (
	ImgVersion        = ":latest"
	SlbImageRepo      = "balance"
	BackendImgVersion = ":latest"
	BackEndImgName    = "backend"
)

func ContainerShortID(id string) string {
	// Ensure the container ID is at least 12 characters long
	if len(id) >= 12 {
		return id[:12]
	}
	return id // If it's shorter than 12 characters, return it as is
}

func CreateDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

func CreateContainer(ctx context.Context, client *client.Client, config *container.Config, hostConfig *container.HostConfig, containerName string) (string, error) {
	// Create host configuration with port mapping
	resp, err := client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("error creating Docker container: %v", err)
	}
	return resp.ID, err
}

func StartContainer(ctx context.Context, client *client.Client, containerID string) error {
	waitRunning := func(running chan bool) {
		defer close(running)
		waitCtx, cancel := context.WithTimeout(ctx, time.Second*3)
		defer cancel()
		for {
			select {
			case <-waitCtx.Done():
				running <- false
				return
			default:
				inspect, _ := client.ContainerInspect(ctx, containerID)
				if inspect.State.Running {
					running <- true
					return
				}
				<-time.After(time.Millisecond * 30)
			}
		}

	}
	if err := client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("error starting Docker container: %v", err)
	}
	running := make(chan bool)
	go waitRunning(running)
	if running := <-running; !running {
		return fmt.Errorf("")
	}

	return nil
}

func StopContainer(ctx context.Context, client *client.Client, containerID string) error {
	if err := client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		slog.Error(fmt.Sprintf("error removing Docker container: %v", err))
		return err
	}
	client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	return nil
}

func CleanupContainer(ctx context.Context, client *client.Client, containerID string) error {
	if err := client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
		slog.Error(fmt.Sprintf("error removing Docker container: %v", err))
		return err
	}
	client.ContainerWait(ctx, containerID, container.WaitConditionRemoved)
	return nil
}

func GetContainerLogs(ctx context.Context, client *client.Client, containerID string) (string, error) {
	out, err := client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
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

func GetIP(ctx context.Context, client *client.Client, containerID string) (string, error) {
	inspect, err := client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	return inspect.NetworkSettings.IPAddress, nil
}

func ListImages(ctx context.Context, client *client.Client, imageTags []string) ([]types.ImageSummary, error) {
	args := filters.NewArgs()
	for _, tag := range imageTags {
		args.Add("reference", tag)
	}

	images, err := client.ImageList(ctx, types.ImageListOptions{
		Filters: args,
	})
	if err != nil {
		return images, fmt.Errorf("error listing Docker images: %v", err)
	}
	return images, err
}

func WaitForImage(ctx context.Context, client *client.Client, imageTags []string, timeout time.Duration) error {
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
			images, err := ListImages(ctx, client, imageTags)
			if err != nil {
				return err
			}
			if len(images) > 0 {
				return nil
			}

		}
	}
}
