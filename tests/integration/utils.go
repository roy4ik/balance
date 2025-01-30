//go:build integration
// +build integration

package integration

import (
	api "balance/gen"
	"balance/internal/apiService"
	tlsConfig "balance/internal/tls"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func fileNameFromPath(p string) string {
	_, fileName := path.Split(p)
	return fileName
}

var (
	certsDirLocalPath         = "../../tmp/certs/"
	caCertFileName     string = fileNameFromPath(tlsConfig.DefaultCAPath)
	caKeyFileName      string = fileNameFromPath(tlsConfig.DefaultCaKeyFilePath)
	serverCertFileName string = fileNameFromPath(tlsConfig.DefaultServiceCertPath)
	serverKeyFileName  string = fileNameFromPath(tlsConfig.DefaultServiceKeyPath)
	clientCertFileName string = fileNameFromPath(tlsConfig.DefaultClientCertPath)
	clientKeyFileName  string = fileNameFromPath(tlsConfig.DefaultClientKeyPath)
)

const (
	HostPort          = apiService.DefaultApiPort
	backendListenPort = "8080"
)

func newApiClient(certDir string, serverAddress string, port string) (api.BalanceClient, error) {
	creds, err := tlsConfig.GetTlsCredentials(certDir+caCertFileName, certDir+clientCertFileName, certDir+clientKeyFileName)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(serverAddress+":"+port, grpc.WithTransportCredentials(creds))
	return api.NewBalanceClient(conn), err
}

func setupMtls(t require.TestingT, outboundIP net.IP, localIP net.IP, ip string, localCertsDir string) {
	// Create the CA

	// Both the server and the client must be in the SANs
	CN := "balance"
	dnsNames := []string{"localhost", CN}

	caCert, caKey, caCertPEM, caKeyPEM, err := generateCA(CN, []net.IP{}, dnsNames)
	require.NoError(t, err)

	// Generate server and client certificates signed by the CA
	serverCertPEM, serverKeyPEM, err := generateCertificate(caCert, caKey, CN, []net.IP{net.ParseIP(ip)}, dnsNames)
	require.NoError(t, err)
	clientCertPEM, clientKeyPEM, err := generateCertificate(caCert, caKey, CN, []net.IP{net.IP(localIP), outboundIP}, dnsNames)
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
