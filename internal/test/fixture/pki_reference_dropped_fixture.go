// Design: docs/architecture/pki/tls-listeners.md -- a reload that drops the certificate name
// Related: lg_pki_fixture.go -- lgPKIDaemon, the chain and the handshake this driver reuses
// Related: register_lg_pki.go -- the registrations

// The two drivers here answer one question for each hub TLS listener: after a
// SIGHUP reload removes the listener's certificate leaf, which certificate does
// the next handshake present? The config then names no certificate, so a
// restart would serve the self-signed pair, and the reload must too. The answer
// is only observable from a crypto/tls client, because the observer sandbox
// cannot drive a handshake and `http=get` never reads the chain it accepts.

package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// pkiDroppedListener is the hub TLS listener one driver drops the name of.
type pkiDroppedListener struct {
	container string // the environment container holding the certificate leaf
	banner    string // the line the daemon prints when the listener binds, before the scheme
	web       bool   // the web server serves only once somebody can log in
}

// webEnvironmentContainer is the environment container that configures the web
// listener, and so the container its certificate leaf sits in.
const webEnvironmentContainer = "web"

var (
	pkiDroppedWeb = pkiDroppedListener{container: webEnvironmentContainer, banner: "web server listening on", web: true}
	pkiDroppedLG  = pkiDroppedListener{container: "looking-glass", banner: lgPKIListenBanner}
)

// pkiDroppedConfig renders the daemon config: the pki container holding the
// chain's one leaf as "dropped-cert", and the listener on port naming it when
// named is true. The pki container stays when the name goes, so the reload
// changes the reference alone.
func pkiDroppedConfig(listener pkiDroppedListener, chain *lgPKIChain, named bool, port int) string {
	var tb textbuf.Buffer
	if listener.web {
		writeWebAdminSystem(&tb)
		tb.Byte('\n')
	}
	writeLGPKIStore(&tb, chain, []string{"dropped-cert"})
	tb.Str("\nenvironment {\n\t").Str(listener.container).Str(" {\n\t\tenabled true\n")
	if named {
		tb.Str("\t\tcertificate dropped-cert\n")
	}
	tb.Str("\t\tserver main {\n\t\t\tip 127.0.0.1;\n\t\t\tport ").Int(int64(port)).Str(";\n\t\t}\n\t}\n}\n")
	return tb.String()
}

// pkiDroppedPresented handshakes without verifying and answers what the
// listener presented, leaf first. Verification is off because the certificate
// expected after the reload is self-signed; the caller judges the chain.
func pkiDroppedPresented(ctx context.Context, port int) ([]*x509.Certificate, error) {
	dialer := &tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}} //nolint:gosec // the driver inspects the presented chain rather than trusting it
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("TLS handshake on port %d: %w", port, err)
	}
	defer conn.Close() //nolint:errcheck // test client teardown
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("dialed connection is %T, not TLS", conn)
	}
	presented := tlsConn.ConnectionState().PeerCertificates
	if len(presented) == 0 {
		return nil, fmt.Errorf("the listener on port %d presented no certificate", port)
	}
	return presented, nil
}

// pkiReferenceDropped is `reload/web-pki-reference-dropped` and
// `reload/lg-pki-reference-dropped`.
//
// Method: start the listener on a named store certificate and confirm a client
// trusting only the operator's root verifies it. Rewrite the config without the
// certificate leaf and SIGHUP. The reload must be accepted, the next handshake
// must present a self-signed certificate that is not the store leaf and that
// the operator's root did not issue, and the listener must not have rebound.
func pkiReferenceDropped(listener pkiDroppedListener) Driver {
	return func(ctx context.Context, args []string) error {
		// The port is the one the runner leased: a port the fixture probed for
		// itself can be taken by another test between the probe and the bind.
		if len(args) != 1 {
			return fmt.Errorf("usage: the fixture takes one argument, the port the runner leased; got %d", len(args))
		}
		port, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("port %q: %w", args[0], err)
		}
		root, err := os.MkdirTemp("", "ze-pki-dropped-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

		const leafCN = "ze dropped store leaf"
		chain, err := newLGPKIChain(false, leafCN)
		if err != nil {
			return err
		}
		daemon, err := startTLSFixtureDaemon(ctx, filepath.Join(root, "hub.conf"),
			pkiDroppedConfig(listener, chain, true, port), port, listener.banner)
		if err != nil {
			return err
		}
		defer daemon.stop()

		named, err := dialLGPKI(ctx, port, chain.RootPool)
		if err != nil {
			return fmt.Errorf("%s before the reload: %w\n%s", listener.container, err, daemon.output.String())
		}
		err = lgPKIAssertServedChain(named, chain, leafCN)
		named.close()
		if err != nil {
			return fmt.Errorf("%s before the reload: %w", listener.container, err)
		}
		lgPKIOK(listener.container+" serves the named store leaf, verified against the operator root:", leafCN)
		bindsBefore := strings.Count(daemon.output.String(), listener.banner)

		accepted, err := daemon.reload(ctx, pkiDroppedConfig(listener, chain, false, port))
		if err != nil {
			return err
		}
		if !accepted {
			return fmt.Errorf("the reload removing environment %s certificate was refused:\n%s", listener.container, daemon.output.String())
		}

		presented, err := pkiDroppedPresented(ctx, port)
		if err != nil {
			return fmt.Errorf("%s after the reload: %w\n%s", listener.container, err, daemon.output.String())
		}
		leaf := presented[0]
		if leaf.Subject.CommonName == leafCN {
			return fmt.Errorf("%s still presents the store leaf %q the config no longer names", listener.container, leafCN)
		}
		if err := leaf.CheckSignatureFrom(chain.Root); err == nil {
			return fmt.Errorf("%s presents %q, issued by the operator root: a store certificate the config no longer names", listener.container, leaf.Subject.CommonName)
		}
		// The self-signed leaf is not a CA, so CheckSignatureFrom refuses it as
		// a parent. Self-signed is its two facts: it names itself as issuer,
		// and its own key verifies its signature.
		if !bytes.Equal(leaf.RawIssuer, leaf.RawSubject) {
			return fmt.Errorf("%s presents %q, issued by %q: not self-signed", listener.container, leaf.Subject.CommonName, leaf.Issuer)
		}
		if err := leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature); err != nil {
			return fmt.Errorf("%s presents %q, which its own key did not sign: %w", listener.container, leaf.Subject.CommonName, err)
		}
		lgPKIOK(listener.container+" presents its self-signed certificate once the name is dropped:", leaf.Subject.CommonName)

		if binds := strings.Count(daemon.output.String(), listener.banner); binds != bindsBefore {
			return fmt.Errorf("%s rebound its listener: %d banners before the reload, %d after:\n%s",
				listener.container, bindsBefore, binds, daemon.output.String())
		}
		lgPKIOK(listener.container+" kept its socket: the certificate changed without a rebind", "")
		fmt.Fprint(os.Stderr, daemon.output.String())
		return nil
	}
}
