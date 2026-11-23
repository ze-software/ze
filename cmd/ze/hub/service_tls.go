// Design: docs/architecture/pki/tls-listeners.md -- one precedence rule for every hub TLS listener
//
// Always on, although its startup consumers are tagged. service_web.go (ze_web)
// and service_lg.go (ze_lg) ask this file for the material their listener
// serves at start, and the reload hooks in listener_migrate.go, which are
// always-on hub code, ask it for the material a reloaded config describes. One
// function answers both, so a restart and a reload cannot disagree about which
// certificate one config asks for.
//
// Related: cert_store.go -- the live store the self-signed branch persists into
// Related: listener_migrate.go -- updateWebCertificate and updateLGCertificate

package hub

import (
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// errSelfSignedNeedsStore refuses the self-signed branch for a listener that has
// no store to keep the pair in. Startup never reaches it: the web server is
// disabled without a store, and the looking glass decides plaintext or an error
// before it asks (service_lg.go). A reload does, when it drops the certificate
// name of a looking glass that started on a named certificate without a store.
var errSelfSignedNeedsStore = opererr.New(
	"The listener cannot return to its self-signed certificate.",
	"No certificate is named, and the self-signed certificate is kept in the database store, which this daemon runs without.",
	"Name a pki certificate in the listener's certificate leaf, or run `ze init` and restart ze.",
)

// listenerTLSMaterial returns the PEM material a hub TLS listener serves.
//
// It is the one place the precedence between a named certificate and the
// self-signed pair is decided. The web and API listener and the looking glass
// both call it at start, and both reload hooks call it, so neither two surfaces
// nor a start and a reload can disagree about that rule.
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
// require the daemon's live store. Named certificates do not use that store. A
// nil certStore with an empty name is refused rather than answered with no
// material: the listener has no certificate the config describes.
func listenerTLSMaterial(certName string, certStore selfcert.CertStore, listenAddr string) (certPEM, keyPEM []byte, err error) {
	if certName != "" {
		return zepki.ServerTLSMaterial(certName)
	}
	if certStore == nil {
		return nil, nil, errSelfSignedNeedsStore
	}
	return selfcert.LoadOrGenerateCert(certStore, listenAddr)
}
