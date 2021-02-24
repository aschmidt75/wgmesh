package meshservice

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	ioutil "io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// TLSConfig ...
type TLSConfig struct {
	Cert     tls.Certificate
	CertPool *x509.CertPool
}

// NewTLSConfigFromFiles creates a new TLS config from given files/paths
func NewTLSConfigFromFiles(caCertFile, caPath, certFile, keyFile string) (*TLSConfig, error) {

	tlsConfig := &TLSConfig{}

	var err error

	tlsConfig.CertPool = x509.NewCertPool()
	if caCertFile != "" {
		caCertPEM, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			return nil, err
		}
		if !tlsConfig.CertPool.AppendCertsFromPEM(caCertPEM) {
			return nil, fmt.Errorf("credentials: failed to append certificate")

		}
	}
	if caPath != "" {
		err := filepath.Walk(caPath, func(path string, info os.FileInfo, err error) error {
			certPEM, err := ioutil.ReadFile(path)
			if err == nil {
				if ok := tlsConfig.CertPool.AppendCertsFromPEM(certPEM); ok {
					log.WithField("cafile", path).Debug("Added certificate from -ca-path")
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	tlsConfig.Cert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}
