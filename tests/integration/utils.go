//go:build integration
// +build integration

package integration

import (
	api "balance/gen"
	apiService "balance/services/api_service"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func fileNameFromPath(p string) string {
	_, fileName := path.Split(p)
	return fileName
}

var (
	certsDirLocalPath         = "../../tmp/certs/"
	clientCertFileName        = "client-cert.pem"
	clientKeyFileName         = "client-key.pem"
	caCertFileName     string = fileNameFromPath(apiService.DefaultCAPath)
	serverCertFileName string = fileNameFromPath(apiService.DefaultServiceCertPath)
	serverKeyFileName  string = fileNameFromPath(apiService.DefaultServiceKeyPath)
	caKeyFileName             = "ca-key.pem"
)

const (
	HostPort          = apiService.DefaultApiPort
	backendListenPort = "8080"
)

func newApiClient(certDir string, serverAddress string, port string) (api.BalanceClient, error) {
	creds, err := getClientTlsCreds(certDir)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(serverAddress+":"+port, grpc.WithTransportCredentials(creds))
	return api.NewBalanceClient(conn), err
}

func getClientTlsCreds(certDir string) (credentials.TransportCredentials, error) {
	// Load the CA certificate
	caCert, err := os.ReadFile(certDir + caCertFileName)
	if err != nil {
		return nil, fmt.Errorf("Failed to read CA certificate maybe you need to generate them (make certs): %v", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("Failed to append CA certificate")
	}

	// Load the client's certificate and private key
	clientCert, err := tls.LoadX509KeyPair(certDir+clientCertFileName, certDir+clientKeyFileName)
	if err != nil {
		return nil, fmt.Errorf("Failed to load client certificate and key: %v", err)
	}

	// Configure TLS for the gRPC client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}
	creds := credentials.NewTLS(tlsConfig)
	return creds, nil
}

func setup(t *testing.T) (context.Context, *client.Client, string, string) {
	ctx := context.Background()

	cli, err := createDockerClient()
	require.NoError(t, err)

	imageTags := []string{slbImageRepo + imgVersion}
	testInstanceName := testNameWithUUID(t)
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
			apiService.DefaultServiceKeyPath,
			apiService.DefaultServiceKeyPath,
			apiService.DefaultCAPath,
			apiService.DefaultCertsDirectory,
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
			Target: apiService.DefaultCertsDirectory,
		}},
	}
	containerID, err := createContainer(ctx, cli, config, hostConfig, strings.ToLower(testInstanceName)+"-"+"slb")
	t.Cleanup(func() {
		o, _ := getContainerLogs(ctx, cli, containerID)
		t.Log(o)
		stopContainer(context.Background(), cli, containerID)
		cleanupContainer(context.Background(), cli, containerID)
	})
	require.NoError(t, err)

	require.NoError(t, startContainer(cli, ctx, containerID))
	setupMtls(t, containerID, certDir)

	// stop and start container to make balance start without delay, waiting for files
	require.NoError(t, stopContainer(ctx, cli, containerID))
	require.NoError(t, startContainer(cli, ctx, containerID))

	return ctx, cli, containerID, certDir
}

func setupSlbWithBackends(t *testing.T, numBackends int) (context.Context, *client.Client, string, []string, string) {
	// setup slb
	ctx, cli, containerID, certDir := setup(t)
	// setup backends
	backendContainers := make([]string, 0)
	for i := 0; i < numBackends; i++ {
		ctx := context.Background()

		cli, err := createDockerClient()
		require.NoError(t, err)

		imageTags := []string{BackEndImgName + BackendImgVersion}
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
				Target: apiService.DefaultCertsDirectory,
			}},
		}
		backendContainerID, err := createContainer(ctx, cli, config, hostConfig, strings.ToLower(testNameWithUUID(t))+"-"+"backend-")
		t.Cleanup(func() {
			stopContainer(context.Background(), cli, backendContainerID)
			cleanupContainer(context.Background(), cli, backendContainerID)
		})
		require.NoError(t, err)
		// required is the shortened id here as the backend provides the full id.
		backendContainers = append(backendContainers, backendContainerID[:11])
		require.NoError(t, startContainer(cli, ctx, backendContainerID))
	}
	return ctx, cli, containerID, backendContainers, certDir
}

func setupMtls(t *testing.T, containerID string, localCertsDir string) {
	// get local ips
	outboundIP, err := getOutBoundIP()
	require.NoError(t, err)
	localIP, err := getLocalIP()
	require.NoError(t, err)

	var ip string
	require.Eventually(t, func() bool {
		ip, _ = getContainerIP(containerID)
		return ip != ""
	}, time.Second*10, time.Millisecond*30)

	require.NotEmpty(t, ip)
	require.NoError(t, err)

	// Create the CA

	// Both the server and the client must be in the SANs
	CN := "balance"
	dnsNames := []string{"localhost", CN}

	caCert, caKey, caCertPEM, caKeyPEM, err := generateCA(CN, []net.IP{}, dnsNames)
	require.NoError(t, err)

	// Generate server and client certificates signed by the CA
	serverCertPEM, serverKeyPEM, err := generateCertificate(caCert, caKey, CN, []net.IP{net.ParseIP(ip)}, dnsNames)
	require.NoError(t, err)
	clientCertPEM, clientKeyPEM, err := generateCertificate(caCert, caKey, CN, []net.IP{localIP, outboundIP}, dnsNames)
	require.NoError(t, err)

	// Save Certs
	type certFile struct {
		localpath string
		content   []byte
		mode      os.FileMode
	}
	files := []certFile{
		// ca
		{localCertsDir + caCertFileName, caCertPEM, 0644},
		{localCertsDir + caKeyFileName, caKeyPEM, 0600},
		// server
		{localCertsDir + serverCertFileName, serverCertPEM, 0644},
		{localCertsDir + serverKeyFileName, serverKeyPEM, 0600},
		// client
		{localCertsDir + clientCertFileName, clientCertPEM, 0644},
		{localCertsDir + clientKeyFileName, clientKeyPEM, 0600},
	}
	for _, file := range files {
		require.NoError(t, os.WriteFile(file.localpath, file.content, file.mode))
	}
}

func generateCA(host string, ipAddresses []net.IP, dnsNames []string) (*x509.Certificate, *ecdsa.PrivateKey, []byte, []byte, error) {
	caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{host},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // Valid for 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
		IPAddresses:           ipAddresses,
		DNSNames:              dnsNames,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	ecPriv, err := x509.MarshalECPrivateKey(caPriv)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecPriv})

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return caCert, caPriv, caCertPEM, caKeyPEM, nil
}

// Helper function to generate a certificate and private key with SANs
func generateCertificate(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, host string, ipAddresses []net.IP, dnsNames []string) ([]byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Certificate valid for one year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   host,
			Organization: []string{host},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           ipAddresses,
		DNSNames:              dnsNames,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	ecPriv, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecPriv})

	return certPEM, keyPEM, nil
}

func getLocalIP() (net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue // Skip down or loopback interfaces
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				return ipNet.IP, nil
			}
		}
	}
	return nil, fmt.Errorf("no valid local IP address found")
}

func getOutBoundIP() (net.IP, error) {
	// Create a UDP connection to an external address (Google's public DNS)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Get the local address of the connection and extract the IP
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

func testNameWithUUID(t *testing.T) string {
	return t.Name() + "-" + uuid.NewString()[:4]
}
