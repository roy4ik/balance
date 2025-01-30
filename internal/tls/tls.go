package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

const DefaultCertsDirectory = "/etc/certs/"
const DefaultCAPath = DefaultCertsDirectory + "ca-cert.pem"
const DefaultCaKeyFilePath = DefaultCertsDirectory + "ca-key.pem"
const DefaultServiceCertPath = DefaultCertsDirectory + "service-cert.pem"
const DefaultServiceKeyPath = DefaultCertsDirectory + "service-key.pem"
const DefaultClientCertPath = DefaultCertsDirectory + "client-cert.pem"
const DefaultClientKeyPath = DefaultCertsDirectory + "client-key.pem"

func GetTlsCredentials(caPath string, certPath string, keyPath string) (credentials.TransportCredentials, error) {
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %s", err.Error())
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to cert pool")
	}

	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate and key: %s", err.Error())
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		RootCAs:      caCertPool,
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert, // RequireAndVerifyFor mTLS
	}

	// Create gRPC server with TLS credentials
	creds := credentials.NewTLS(tlsConfig)
	return creds, nil
}
