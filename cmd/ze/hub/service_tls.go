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
// Related: listener_migrate.go -- updateWebCertificate and updateLGTransport
// Related: lg_transport.go -- which looking-glass listeners serve TLS at all

package hub

import (
	"context"
	"io"
	"log/slog"

	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// errSelfSignedNeedsStore refuses the self-signed branch for a listener that has
// no store to keep the pair in. Startup never reaches it: the web server is
// disabled without a store, and the looking glass decides HTTP or a server
// left down for each of its entries before it asks (planLGEndpoints). A reload of a config
// that describes the looking-glass listeners never reaches it either, for the
// same reason. It is reached when a reloaded config no longer starts a looking
// glass and names no certificate, while the running one serves TLS on a named
// certificate without a store: its listeners stay, and they have no
// self-signed pair to return to.
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

// webCertificateProblem answers the operator error of a web server whose named
// certificate cannot be presented, resolved against the installed pki store as
// the listener resolves it. The second result is false when certName resolves,
// and the error is then nil. Callers MUST pass a name that is not empty: an
// empty name is the self-signed pair and names nothing to resolve.
//
// The start path reads it before it builds any service. A web server that
// cannot present the certificate its config names is not started, and ze runs:
// the operator reaches the daemon over its other surfaces and corrects the
// name. Serving the self-signed pair in its place would read as a working
// deployment (docs/architecture/pki/tls-listeners.md, "a named certificate
// fails closed").
func webCertificateProblem(certName string) (problem *opererr.Error, unpresentable bool) {
	_, _, err := zepki.ServerTLSMaterial(certName)
	if err == nil {
		return nil, false
	}
	return opererr.Wrap(err,
		"The web server cannot serve TLS: the certificate "+certName+" cannot be presented.",
		"`environment web certificate` or ze.web.certificate names it, and ze never serves a self-signed certificate in place of a named certificate",
		"Define it with `edit pki certificate "+certName+"`, or delete `environment web certificate`, then restart ze.",
	), true
}

// webNotStartedEffect is what a start does with a web server it cannot serve.
// The log line and the terminal line both carry it, so the operator reads that
// ze itself is running.
const webNotStartedEffect = "the web server is not started; ze and its other services keep running"

// reportWebNotStarted tells the operator that a start runs without the web
// server: an error line on log under the doctor code the web certificate
// reference check reports the same name with (internal/component/web), and the
// same text on console.
func reportWebNotStarted(log *slog.Logger, console io.Writer, problem *opererr.Error) {
	log.LogAttrs(context.Background(), slog.LevelError, problem.Problem(),
		problem.LogAttrs(slog.String("code", diagnostic.CodeDoctorTLSReference), slog.String("effect", webNotStartedEffect))...)
	var line textbuf.Buffer
	line.Str("error: ").Str(webNotStartedEffect).Str(": ").Str(problem.Error()).Byte('\n')
	if _, err := console.Write(line.Bytes()); err != nil {
		log.Warn("the web server report could not be written to the terminal",
			"code", diagnostic.CodeDoctorTLSReference, "error", err)
	}
}
