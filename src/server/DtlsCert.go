package server

import (
	"crypto/tls"

	"github.com/pion/dtls/v2"
)

func LoadDtlsConfig(ServerCfg *ServerConfig) (*dtls.Config, error) {

	cert, err := tls.LoadX509KeyPair("/etc/vpn/server-cert.pem", "/etc/vpn/server-key.pem")

	if err != nil {
		return nil, err
	}

	// Configure DTLS
	config := &dtls.Config{
		Certificates:         []tls.Certificate{cert},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	return config, nil

}
