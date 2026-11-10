// Design: docs/guide/gnmi.md -- TLS: the transport serves TLS or nothing
// Related: server.go -- Serve, which loads this before it listens

package gnmi

import (
	"crypto/tls"
	"os"

	"google.golang.org/grpc/credentials"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The fix lines name both spellings of each setting: the config leaf and the
// environment variable that overrides it (hub service_gnmi.go reads both).
// Serve reads the files once, when gNMI starts, so the change takes a restart.
const (
	tlsCertSetting = "environment gnmi tls cert (or ze.gnmi.tls.cert)"
	tlsKeySetting  = "environment gnmi tls key (or ze.gnmi.tls.key)"
	tlsRestart     = ", then run ze signal restart"
	tlsNoClearText = "the config asks gNMI for TLS, and gNMI never serves its token and config in clear text"
)

// loadTLSCredentials reads the certificate and private-key files the config
// names and answers the transport credentials Serve installs.
//
// It refuses, with an operator error naming the file, when only one of the
// two paths is set, when a file cannot be read, or when the pair does not
// parse. Before this, an unreadable file left the PEM empty and the listener
// served clear text over a config that asked for TLS. Serve calls it before
// it listens, so a refusal leaves no socket open.
func loadTLSCredentials(certFile, keyFile string) (credentials.TransportCredentials, error) {
	if certFile == "" {
		return nil, opererr.New(
			"gNMI not started: TLS key file "+keyFile+" is set without a certificate file",
			tlsNoClearText+", and TLS needs both files",
			"set "+tlsCertSetting+" to the certificate of that key"+tlsRestart)
	}
	if keyFile == "" {
		return nil, opererr.New(
			"gNMI not started: TLS certificate file "+certFile+" is set without a key file",
			tlsNoClearText+", and TLS needs both files",
			"set "+tlsKeySetting+" to the private key of that certificate"+tlsRestart)
	}

	certPEM, err := os.ReadFile(certFile) //nolint:gosec // operator-configured certificate path
	if err != nil {
		return nil, opererr.Wrap(err,
			"gNMI not started: TLS certificate file "+certFile+" cannot be read",
			tlsNoClearText,
			"restore "+certFile+" or point "+tlsCertSetting+" at a readable PEM file"+tlsRestart)
	}
	keyPEM, err := os.ReadFile(keyFile) //nolint:gosec // operator-configured private-key path
	if err != nil {
		return nil, opererr.Wrap(err,
			"gNMI not started: TLS key file "+keyFile+" cannot be read",
			tlsNoClearText,
			"restore "+keyFile+" or point "+tlsKeySetting+" at a readable PEM file"+tlsRestart)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, opererr.Wrap(err,
			"gNMI not started: TLS certificate file "+certFile+" and key file "+keyFile+" are not a valid pair",
			tlsNoClearText,
			"replace them with a PEM certificate and its matching private key, or correct "+
				tlsCertSetting+" and "+tlsKeySetting+tlsRestart)
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}), nil
}
