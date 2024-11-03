//go:build integration
// +build integration

package integration

import (
	"archive/tar"
	api "balance/gen"
	apiService "balance/services/api_service"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const certsDirLocalPath = "../../gen/certs/"
const clientCertFileName = "client-cert.pem"
const clientKeyFileName = "client-key.pem"

func newApiClient(serverAddress string, port string) (api.BalanceClient, error) {
	// Load the CA certificate
	_, caCertFileName := path.Split(apiService.DefaultCAPath)
	caCert, err := os.ReadFile(certsDirLocalPath + caCertFileName)
	if err != nil {
		return nil, fmt.Errorf("Failed to read CA certificate maybe you need to generate them (make certs): %v", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("Failed to append CA certificate")
	}

	// Load the client's certificate and private key
	clientCert, err := tls.LoadX509KeyPair(certsDirLocalPath+clientCertFileName, certsDirLocalPath+clientKeyFileName)
	if err != nil {
		log.Fatalf("Failed to load client certificate and key: %v", err)
	}

	// Configure TLS for the gRPC client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}
	creds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.NewClient(serverAddress+":"+port, grpc.WithTransportCredentials(creds))
	return api.NewBalanceClient(conn), err
}

func setup(t *testing.T, name string) (context.Context, *client.Client, string) {
	ctx := context.Background()

	cli, err := createDockerClient()
	require.NoError(t, err)

	imageTags := []string{slbImageRepo + imgVersion}
	config := &container.Config{
		Image: imageTags[0],
		ExposedPorts: nat.PortSet{
			nat.Port(HostPort): struct{}{},
			backendListenPort:  struct{}{},
		},
	}
	containerID, err := createAndStartContainer(ctx, cli, config, strings.ToLower(t.Name())+"-"+strings.ToLower(name))
	t.Cleanup(func() {
		o, _ := getContainerLogs(ctx, cli, containerID)
		t.Log(o)
		cleanupContainer(context.Background(), cli, containerID)
	})

	// Create the CA

	// Generate server and client certificates signed by the CA
	getLocalIP := func() (net.IP, error) {
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

	getOutBoundIP := func() (net.IP, error) {
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
	outboundIP, err := getOutBoundIP()
	require.NoError(t, err)
	ip, err := getContainerIP(containerID)
	require.NoError(t, err)
	localIP, err := getLocalIP()
	require.NoError(t, err)

	// Both the server and the client must be in the SANs
	ipAddresses := []net.IP{net.ParseIP(ip), localIP, outboundIP}
	dnsNames := []string{"localhost", "balance"}

	caCert, caKey, caCertPEM, caKeyPEM, err := generateCA("balance", ipAddresses, dnsNames)
	require.NoError(t, err)
	certsDirPath, caCertFileName := path.Split(apiService.DefaultCAPath)
	os.WriteFile(certsDirLocalPath+caCertFileName, caCertPEM, 0644)
	os.WriteFile(certsDirLocalPath+"ca-key.pem", caKeyPEM, 0600)

	serverCertPEM, serverKeyPEM, err := generateCertificate(caCert, caKey, "balance", ipAddresses, dnsNames)
	require.NoError(t, err)
	clientCertPEM, clientKeyPEM, err := generateCertificate(caCert, caKey, "client", ipAddresses, dnsNames)
	require.NoError(t, err)

	// Save the certificates to files
	_, serverCertFileName := path.Split(apiService.DefaultServiceCertPath)
	localServerCertPath := certsDirLocalPath + serverCertFileName
	require.NoError(t, os.WriteFile(localServerCertPath, serverCertPEM, 0644))
	_, serverKeyFileName := path.Split(apiService.DefaultServiceKeyPath)
	require.NoError(t, os.WriteFile(certsDirLocalPath+serverKeyFileName, serverKeyPEM, 0600))
	localClientCertPath := certsDirLocalPath + clientCertFileName
	require.NoError(t, os.WriteFile(localClientCertPath, clientCertPEM, 0644))
	require.NoError(t, os.WriteFile(certsDirLocalPath+clientKeyFileName, clientKeyPEM, 0600))

	// Copy certificates to Docker containers
	serverCert, err := os.Open(localServerCertPath)
	defer serverCert.Close()
	require.NoError(t, err)
	require.NoError(t, copyToDocker(cli, ctx, containerID, apiService.DefaultServiceCertPath, serverCert))

	clientCert, err := os.Open(localClientCertPath)
	require.NoError(t, err)
	defer clientCert.Close()
	require.NoError(t, copyToDocker(cli, ctx, containerID, certsDirPath+clientCertFileName, clientCert))

	caCertFile, err := os.Open(certsDirLocalPath + caCertFileName)
	require.NoError(t, err)
	defer caCertFile.Close()
	require.NoError(t, copyToDocker(cli, ctx, containerID, certsDirPath+caCertFileName, caCertFile))
	return ctx, cli, containerID
}

func setupSlbWithBackends(t *testing.T, numBackends int) (context.Context, *client.Client, string, []string) {
	// setup slb
	ctx, cli, containerID := setup(t, "slb"+"-"+uuid.NewString()[:4])
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
				"443":             struct{}{},
				backendListenPort: struct{}{},
			},
		}
		backendContainerID, err := createAndStartContainer(ctx, cli, config, strings.ToLower(t.Name())+"-"+"backend-"+uuid.NewString()[:4])
		require.NoError(t, err)
		// required is the shortened id here as the backend provides the full id.
		backendContainers = append(backendContainers, backendContainerID[:11])
	}
	return ctx, cli, containerID, backendContainers
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

func createTar(fileName string, reader io.Reader) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buffer)
	defer tarWriter.Close()

	// Read the entire content into a temporary buffer to get the size
	contentBuffer := new(bytes.Buffer)
	size, err := io.Copy(contentBuffer, reader)
	if err != nil {
		return nil, err
	}

	// Create a tar header with the correct size
	header := &tar.Header{
		Name: fileName,
		Mode: 0600,
		Size: size,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return nil, err
	}

	// Write the content from the buffer to the tar writer
	if _, err := io.Copy(tarWriter, contentBuffer); err != nil {
		return nil, err
	}

	return buffer, nil
}

func copyToDocker(cli *client.Client, ctx context.Context, containerID string, destPath string, content io.Reader) error {
	dirPath, fileName := path.Split(destPath)
	tar, err := createTar(fileName, content)
	if err != nil {
		return err
	}
	return cli.CopyToContainer(ctx, containerID, dirPath, tar, types.CopyToContainerOptions{})
}
