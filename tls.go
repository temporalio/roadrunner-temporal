package rrtemporal

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

func initTLS(cfg *Config) (*tls.Config, error) {
	// simple TLS based on the cert and key
	var err error
	var cert tls.Certificate
	var certPool *x509.CertPool
	var rca []byte

	// if client CA is not empty we combine it with Cert and Key
	if cfg.TLS.RootCA != "" {
		cert, err = tls.LoadX509KeyPair(cfg.TLS.Cert, cfg.TLS.Key)
		if err != nil {
			return nil, err
		}

		certPool, err = x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		if certPool == nil {
			certPool = x509.NewCertPool()
		}

		// we already checked this file in the config.go
		rca, err = os.ReadFile(cfg.TLS.RootCA)
		if err != nil {
			return nil, err
		}

		if ok := certPool.AppendCertsFromPEM(rca); !ok {
			return nil, err
		}

		return &tls.Config{
			MinVersion:   tls.VersionTLS12,
			ClientAuth:   cfg.TLS.auth,
			Certificates: []tls.Certificate{cert},
			ClientCAs:    certPool,
			RootCAs:      certPool,
			ServerName:   cfg.TLS.ServerName,
		}, nil
	}

	cert, err = tls.LoadX509KeyPair(cfg.TLS.Cert, cfg.TLS.Key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		ServerName:   cfg.TLS.ServerName,
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}, nil
}
