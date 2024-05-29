package rrtemporal

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

func initTLS(cfg *Config) (*tls.Config, error) {
	// simple TLS based on the cert and key
	var err error
	var certPool *x509.CertPool
	var rca []byte

	// if client CA is not empty we combine it with Cert and Key
	if cfg.TLS.RootCA != "" {
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
			MinVersion:           tls.VersionTLS12,
			ClientAuth:           cfg.TLS.auth,
			GetClientCertificate: getClientCertificate(cfg),
			ClientCAs:            certPool,
			RootCAs:              certPool,
			ServerName:           cfg.TLS.ServerName,
		}, nil
	}

	return &tls.Config{
		ServerName:           cfg.TLS.ServerName,
		MinVersion:           tls.VersionTLS12,
		GetClientCertificate: getClientCertificate(cfg),
	}, nil
}

// getClientCertificate is used for tls.Config struct field GetClientCertificate and enables re-fetching the client certificates when they expire
func getClientCertificate(cfg *Config) func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
	return func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.Cert, cfg.TLS.Key)
		if err != nil {
			return nil, err
		}

		return &cert, nil
	}
}
