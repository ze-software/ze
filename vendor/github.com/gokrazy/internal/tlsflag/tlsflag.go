package tlsflag

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gokrazy/internal/config"
)

type ErrNotYetCreated struct {
	HostConfigPath string
	CertPath       string
	KeyPath        string
}

func (e *ErrNotYetCreated) Error() string {
	return "self-signed certificate not yet created"
}

func CertificatePathsFor(useTLS, hostname string) (certPath string, keyPath string, _ error) {
	hostConfigPath := config.HostnameSpecific(hostname)
	certPath = filepath.Join(string(hostConfigPath), "cert.pem")
	keyPath = filepath.Join(string(hostConfigPath), "key.pem")
	exist := true
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		exist = false
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		exist = false
	}

	switch useTLS {
	case "self-signed":
		// If the user set -tls=self-signed, treat non-existing certificates as
		// an error.

		if !exist {
			return "", "", &ErrNotYetCreated{
				HostConfigPath: string(hostConfigPath),
				CertPath:       certPath,
				KeyPath:        keyPath,
			}
		}

		// TODO: Check validity dates of existing certificate

	case "off":
		// User specified -tls=off explicitly.
		return "", "", nil

	case "":
		// If the user did not set -tls, return the cert/key path locations only
		// if they exist.

		if !exist {
			return "", "", nil
		}

	default:
		parts := strings.Split(useTLS, ",")
		certPath = parts[0]
		if len(parts) > 1 {
			keyPath = parts[1]
		} else {
			return "", "", fmt.Errorf("no private key supplied")
		}
		// TODO: Check validity
	}
	return certPath, keyPath, nil
}
