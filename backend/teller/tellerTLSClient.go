package teller

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"yohanc3/steer/config"
)

type TellerMTLSClient struct {
	Client *http.Client
}

// Creates an mTLS client for Teller.io  
func (client *TellerMTLSClient) NewTellerMTLS() (*http.Client, error) {

	// Loads certificates
	cert, err := tls.LoadX509KeyPair(config.Cfg.TellerCertPem, config.Cfg.TellerKeyPem)

	if err != nil {
		return nil, fmt.Errorf("Error when loading X509 Key Pair for Teller TLS: %w", err)

	}
	
	// Generate bot the tls and the transport config before creating the client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &http.Client{Transport: transport}, nil

}
