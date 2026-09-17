//go:build ze_lg || ze_web

// Design: docs/architecture/pki/tls-listeners.md -- one precedence rule for every hub TLS listener
//
// The build constraint is the DISJUNCTION of its two consumers'. service_web.go
// (ze_web) and service_lg.go (ze_lg) each ask this file for the material their
// listener serves, and nothing else does. A daemon with neither listener serves
// no TLS from the hub.
//
// Related: cert_store.go -- the live store the self-signed branch persists into

package hub

import (
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// listenerTLSMaterial returns the PEM material a hub TLS listener serves.
//
// It is the one place the precedence between a named certificate and the
// self-signed pair is decided. The web and API listener and the looking glass
// both call it, so the two surfaces cannot disagree about that rule.
//
// certName is the operator's certificate leaf. The web listener reads
// environment.web.certificate. The looking glass reads
// environment.looking-glass.certificate.
//
// A set name takes its material from the PKI store, with the full chain.
// certStore is not read on that branch, because the pki container already holds
// the certificate and its key. An empty name loads the established self-signed
// pair from certStore, or generates it into certStore. The SAN hint comes from
// the first endpoint. GenerateWebCertWithAddr already fans out to all interface
// IPs when the host is 0.0.0.0.
//
// It FAILS CLOSED. A configured name that does not resolve returns an error and
// no material, and never the self-signed path. A self-signed certificate served
// while the config names a real one reads as a working deployment. Only a
// client's rejection shows the difference
// (docs/architecture/pki/tls-listeners.md, "a named certificate fails closed").
//
// The caller decides whether certStore is reachable. Self-signed certificates
// require the daemon's live store. Named certificates do not use that store.
func listenerTLSMaterial(certName string, certStore selfcert.CertStore, listenAddr string) (certPEM, keyPEM []byte, err error) {
	if certName != "" {
		return zepki.ServerTLSMaterial(certName)
	}
	return selfcert.LoadOrGenerateCert(certStore, listenAddr)
}
