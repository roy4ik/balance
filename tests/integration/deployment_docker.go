//go:build integration
// +build integration

//go:generate make -C ./../../ balance-docker
//go:generate make -C ./../mock/backend/ backend-docker

package integration

import (
	"balance/internal/apiService"
	tlsConfig "balance/internal/tls"
	"balance/tests/deployment/docker"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
)

type DockerDeployment struct {
	*TestDeployment
	ctx    context.Context
	client *client.Client
}

func (t *DockerDeployment) getIP(id string) (string, error) {
	ipC, errC := make(chan string), make(chan error)
	defer close(ipC)
	defer close(errC)
	ctx, cancelFunc := context.WithTimeout(t.ctx, time.Second*10)
	defer cancelFunc()
	go func() {
		dockerIp, err := docker.GetIP(t.ctx, t.client, id)
		if err != nil {
			errC <- err
		} else if dockerIp != "" {
			ipC <- dockerIp
		}
		<-time.After(time.Microsecond * 30)
	}()
	select {
	case ip := <-ipC:
		return ip, nil
	case err := <-errC:
		return "", fmt.Errorf("container (%s) IP could not be obtained: : %s", id, err)
	case <-ctx.Done():
		return "", fmt.Errorf("container (%s) IP could not be obtained: timeout:", id)
	}
}

func (t DockerDeployment) restart(id string) {
	require.NoError(t, docker.StopContainer(t.ctx, t.client, id))
	require.NoError(t, docker.StartContainer(t.ctx, t.client, id))
}

func (t *DockerDeployment) setup() (string, string) {
	imageTags := []string{docker.SlbImageRepo + docker.ImgVersion}
	testInstanceName := t.Name()
	certDirAbsPath, err := filepath.Abs(certsDirLocalPath + testInstanceName)
	require.NoError(t, err)
	certDir := certDirAbsPath + "/"
	require.NoError(t, os.MkdirAll(certDir, 0755))
	t.Cleanup(func() { os.RemoveAll(certDir) })

	config := &container.Config{
		Image: imageTags[0],
		ExposedPorts: nat.PortSet{
			nat.Port(HostPort): struct{}{},
			backendListenPort:  struct{}{},
		},
		// Due to the certificates needing to be created with the container id which is
		// obtainable only after starting the container a wait script is needed to wait for the certificates
		// before running balance
		Entrypoint: []string{"/bin/sh", "-c", fmt.Sprintf(
			"for file in %s %s %s; do while [ ! -f $file ]; do sleep 0.001 && ls %s; done; done; echo 'certificates created'; %s",
			tlsConfig.DefaultServiceKeyPath,
			tlsConfig.DefaultServiceKeyPath,
			tlsConfig.DefaultCAPath,
			tlsConfig.DefaultCertsDirectory,
			"exec ./balance", // the command is executed with exec to disconnect from sh
		)},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			apiService.DefaultApiPort + "/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: HostPort,
				},
			},
			backendListenPort + "/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: backendListenPort,
				},
			},
		},
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: certDir,
			Target: tlsConfig.DefaultCertsDirectory,
		}},
	}
	containerID, err := docker.CreateContainer(t.ctx, t.client, config, hostConfig, strings.ToLower(testInstanceName)+"-"+"slb")
	t.addContainerCleanup(containerID)
	t.client.ContainerWait(t.ctx, containerID, container.WaitConditionNotRunning)
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	// get local ips
	outboundIP, err := getOutBoundIP()
	require.NoError(t, err)
	require.NotEmpty(t, outboundIP)
	localIP, err := getLocalIP()
	require.NoError(t, err)
	require.NotEmpty(t, localIP)

	// start container and get ip to setup mtls
	require.NoError(t, docker.StartContainer(t.ctx, t.client, containerID))
	ip1, err := t.getIP(containerID[:12])
	setupMtls(t, outboundIP, localIP, ip1, certDir)
	require.NotEmpty(t, ip1)

	// stop and start container to make balance start without delay, waiting for files
	t.restart(containerID)
	ip2, err := t.getIP(containerID[:12])
	require.NotEmpty(t, ip2)
	require.Exactly(t, ip1, ip2)
	return containerID, certDir
}

func (t *DockerDeployment) setupBackends(numBackends int, certDir string) []string {
	// setup backends
	imageTags := []string{docker.BackEndImgName + docker.BackendImgVersion}
	config := &container.Config{
		Image: imageTags[0],
		ExposedPorts: nat.PortSet{
			apiService.DefaultApiPort: struct{}{},
			backendListenPort:         struct{}{},
		},
	}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			backendListenPort + "/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: backendListenPort,
				},
			},
		},
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: certDir,
			Target: tlsConfig.DefaultCertsDirectory,
		}},
	}
	deploy := func(idChan chan string) {
		var wg sync.WaitGroup
		createBackend := func() {
			defer wg.Done()
			containerID, err := docker.CreateContainer(t.ctx, t.client, config, hostConfig, strings.ToLower(t.Name())+"-"+"backend-")
			require.NoError(t, err)
			t.addContainerCleanup(containerID)
			idChan <- containerID
		}
		for i := 0; i < numBackends; i++ {
			wg.Add(1)
			go createBackend()
		}
		wg.Wait()
		close(idChan)
	}

	idChan := make(chan string, numBackends)
	deploy(idChan)

	backendContainers := make([]string, 0)
	for id := range idChan {
		require.NotEmpty(t, id)
		backendContainers = append(backendContainers, id)
		require.NoError(t, docker.StartContainer(t.ctx, t.client, id))
	}
	return backendContainers
}

func (t *DockerDeployment) addContainerCleanup(containerID string) {
	t.Cleanup(func() {
		o, _ := docker.GetContainerLogs(t.ctx, t.client, containerID)
		if t.ctx.Err() != nil {
			t.Log(o)
		}
		if err := docker.StopContainer(t.ctx, t.client, containerID); err != nil {
			t.Error(err)
		}
		if err := docker.CleanupContainer(t.ctx, t.client, containerID); err != nil {
			t.Error(err)
		}
	})
}

func NewDockerDeployment(t *testing.T) Deployment {
	ctx := context.Background()
	client, err := docker.CreateDockerClient()
	require.NoError(t, err)
	return &DockerDeployment{NewTestDeployment(t), ctx, client}
}
