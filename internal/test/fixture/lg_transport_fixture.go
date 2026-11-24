// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass listener serves HTTPS or HTTP
// Related: register_lg_transport.go -- the registration
// Related: lg_pki_fixture.go -- lgPKIDaemon, the chain, the store and the reload fence this driver reuses

// The driver here answers one question through the real daemon: does each
// looking-glass `server` entry serve the protocol its own `tls` leaf names, side
// by side on one daemon, and does a SIGHUP reload that flips one entry's leaf
// switch that listener alone, on the socket it already had? A unit test drives
// transportListener and planLGTransports in the test process, with the hub's
// start and reload wiring replaced; only `ze start`, the config file and
// SIGHUP reach buildLGService, checkLGTransport and updateLGTransport together.
// Which protocol a socket speaks is observable only from a client that speaks
// both, so the checks dial crypto/tls and plain TCP themselves: the observer
// sandbox cannot drive a handshake.

package fixture

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// lgTransportHost is the address every listener of this driver binds.
	lgTransportHost = "127.0.0.1"

	// lgTransportStoreName is the pki certificate the looking glass names. A
	// named certificate serves TLS store or no store, so which protocol an
	// entry gets depends on its tls leaf alone, never on whether this daemon
	// created a database store.
	lgTransportStoreName = "lg-transport"

	// lgTransportLeafCN is the common name of that certificate's leaf.
	lgTransportLeafCN = "ze looking glass transport leaf"

	// lgTransportRFC8522Path is an RFC 8522 function every looking glass serves
	// over TLS with 200: the command list reads no RIB and dispatches nothing.
	// The bare /.well-known/looking-glass/ prefix answers 400 over TLS too, as
	// an unknown path, so only a path TLS accepts can tell the refusal apart.
	lgTransportRFC8522Path = "/.well-known/looking-glass/v1/cmd"

	// lgTransportRFC8522Refusal is the message handleRFC8522 refuses a
	// connection that is not TLS with.
	lgTransportRFC8522Refusal = "HTTPS is required for the RFC 8522 Looking Glass API"

	// lgTransportDowngradeWarning starts the line buildLGService prints for an
	// entry that serves HTTP only for want of a certificate. An entry that says
	// `tls false` asked for HTTP and owes no such warning.
	lgTransportDowngradeWarning = "serves plain HTTP: its tls leaf is unset"

	// lgTransportBodyOctetsMax bounds the body one probe reads.
	lgTransportBodyOctetsMax = 64 << 10

	// lgTransportBannerAttempts bounds the wait for the second listener's
	// banner. Both banners are printed by one loop once the server is ready,
	// so the second follows the first within the same write sequence.
	lgTransportBannerAttempts = 10
)

// lgTransportLeaf is what one server entry writes in its tls leaf.
type lgTransportLeaf uint8

const (
	// lgTransportLeafUnspecified is the zero value. No entry is built with it.
	lgTransportLeafUnspecified lgTransportLeaf = iota
	// lgTransportLeafDefault writes no tls leaf, so the YANG default, true.
	lgTransportLeafDefault
	// lgTransportLeafTrue writes `tls true`.
	lgTransportLeafTrue
	// lgTransportLeafFalse writes `tls false`.
	lgTransportLeafFalse
)

// lgTransportEntry is one looking-glass server entry of the config.
type lgTransportEntry struct {
	name string // the entry's key, and the report label
	port int    // a port the runner leased
	leaf lgTransportLeaf
}

// servesTLS answers the protocol the entry asks for. The looking glass names a
// certificate, so every entry but a `tls false` one has one to present.
func (e lgTransportEntry) servesTLS() bool {
	switch e.leaf {
	case lgTransportLeafDefault, lgTransportLeafTrue:
		return true
	case lgTransportLeafFalse:
		return false
	case lgTransportLeafUnspecified:
	}
	panic("BUG: looking-glass transport entry " + e.name + " built with no tls leaf")
}

// protocol names what the entry serves, for the report.
func (e lgTransportEntry) protocol() string {
	if e.servesTLS() {
		return "HTTPS"
	}
	return "HTTP"
}

// lgTransportConfig renders the daemon config: the pki container holding the
// chain's one leaf, and a looking glass that names it with one server entry for
// each of entries.
func lgTransportConfig(chain *lgPKIChain, entries []lgTransportEntry) string {
	var tb textbuf.Buffer
	writeLGPKIStore(&tb, chain, []string{lgTransportStoreName})
	tb.Str("\nenvironment {\n\tlooking-glass {\n\t\tenabled true\n")
	tb.Str("\t\tcertificate ").Str(lgTransportStoreName).Byte('\n')
	for _, entry := range entries {
		tb.Str("\t\tserver ").Str(entry.name).Str(" {\n\t\t\tip ").Str(lgTransportHost)
		tb.Str(";\n\t\t\tport ").Int(int64(entry.port)).Str(";\n")
		switch entry.leaf {
		case lgTransportLeafDefault:
		case lgTransportLeafTrue:
			tb.Str("\t\t\ttls true;\n")
		case lgTransportLeafFalse:
			tb.Str("\t\t\ttls false;\n")
		case lgTransportLeafUnspecified:
			panic("BUG: looking-glass transport entry " + entry.name + " built with no tls leaf")
		}
		tb.Str("\t\t}\n")
	}
	tb.Str("\t}\n}\n")
	return tb.String()
}

// lgTransportConn is one connection to a looking-glass listener, plain or TLS,
// with the reader that owns whatever the response left buffered.
type lgTransportConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

// dialLGTransport opens one connection to port. A nil config leaves it plain
// TCP; any other config handshakes TLS with it first. The dial and the
// handshake share one bound, so a listener that accepts and never answers
// fails the probe rather than hanging it.
func dialLGTransport(ctx context.Context, port int, config *tls.Config) (*lgTransportConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, listenerMigrationIOTimeout)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(lgTransportHost, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("dial port %d: %w", port, err)
	}
	if config != nil {
		client := tls.Client(conn, config)
		if err := client.HandshakeContext(dialCtx); err != nil {
			_ = conn.Close() //nolint:errcheck // the handshake already failed
			return nil, fmt.Errorf("TLS handshake on port %d: %w", port, err)
		}
		conn = client
	}
	return &lgTransportConn{conn: conn, reader: bufio.NewReader(conn)}, nil
}

// get sends one GET for path and answers the status and the body.
func (c *lgTransportConn) get(path string) (int, string, error) {
	if err := c.conn.SetDeadline(time.Now().Add(listenerMigrationIOTimeout)); err != nil {
		return 0, "", err
	}
	request, err := http.NewRequest(http.MethodGet, "http://"+lgTransportHost+path, http.NoBody) //nolint:noctx // the deadline belongs to the connection
	if err != nil {
		return 0, "", err
	}
	request.Close = true
	if err := request.Write(c.conn); err != nil {
		return 0, "", fmt.Errorf("write %s: %w", path, err)
	}
	response, err := http.ReadResponse(c.reader, request)
	if err != nil {
		return 0, "", fmt.Errorf("read %s: %w", path, err)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, lgTransportBodyOctetsMax))
	if err != nil {
		_ = response.Body.Close() //nolint:errcheck // the read already failed
		return 0, "", fmt.Errorf("read %s body: %w", path, err)
	}
	if err := response.Body.Close(); err != nil {
		return 0, "", err
	}
	return response.StatusCode, string(body), nil
}

func (c *lgTransportConn) close() {
	_ = c.conn.Close() //nolint:errcheck // fixture teardown
}

// lgTransportGet opens one connection, sends one GET for path, and closes it.
func lgTransportGet(ctx context.Context, port int, config *tls.Config, path string) (int, string, error) {
	conn, err := dialLGTransport(ctx, port, config)
	if err != nil {
		return 0, "", err
	}
	defer conn.close()
	return conn.get(path)
}

// lgTransportVerified is the client a visitor of an HTTPS entry is: it trusts
// the operator's root and nothing else, so a handshake that succeeds proves the
// looking glass presented the named chain.
func lgTransportVerified(chain *lgPKIChain) *tls.Config {
	return &tls.Config{RootCAs: chain.RootPool, ServerName: lgPKIServerName, MinVersion: tls.VersionTLS12}
}

// lgTransportAnyTLS is the probe of an HTTP entry. It accepts any certificate,
// so its handshake can only fail because nothing on the port speaks TLS.
func lgTransportAnyTLS() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12} //nolint:gosec // the probe asks whether TLS answers at all, not who answers
}

// assertLGServesHTTPS checks that entry's listener serves HTTPS and only HTTPS:
// a verified handshake answers the status path with 200, and a plain GET of it
// gets no such answer.
func assertLGServesHTTPS(ctx context.Context, chain *lgPKIChain, entry lgTransportEntry) error {
	status, _, err := lgTransportGet(ctx, entry.port, lgTransportVerified(chain), lgPKIStatusPath)
	if err != nil {
		return fmt.Errorf("server %s on port %d, over verified TLS: %w", entry.name, entry.port, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("server %s on port %d answered %s over TLS with %d, want 200", entry.name, entry.port, lgPKIStatusPath, status)
	}
	// A plain GET reaching a TLS listener fails: the connection closes, or
	// http.Server answers the record it cannot read with its own 400. A 200 is
	// the looking glass answering, over HTTP.
	status, _, err = lgTransportGet(ctx, entry.port, nil, lgPKIStatusPath)
	if err == nil {
		if status == http.StatusOK {
			return fmt.Errorf("server %s on port %d answered a plain HTTP GET of %s with 200: it serves HTTP, not HTTPS", entry.name, entry.port, lgPKIStatusPath)
		}
	}
	return nil
}

// assertLGServesHTTP checks that entry's listener serves plain HTTP and only
// HTTP: a plain GET answers the status path with 200, and a TLS handshake,
// which accepts any certificate, fails.
func assertLGServesHTTP(ctx context.Context, entry lgTransportEntry) error {
	status, _, err := lgTransportGet(ctx, entry.port, nil, lgPKIStatusPath)
	if err != nil {
		return fmt.Errorf("server %s on port %d, over plain HTTP: %w", entry.name, entry.port, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("server %s on port %d answered a plain GET of %s with %d, want 200", entry.name, entry.port, lgPKIStatusPath, status)
	}
	conn, err := dialLGTransport(ctx, entry.port, lgTransportAnyTLS())
	if err == nil {
		conn.close()
		return fmt.Errorf("server %s on port %d completed a TLS handshake: it serves HTTPS, not HTTP", entry.name, entry.port)
	}
	// A handshake cut short by the run's own end is not an answer from the
	// listener.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// assertLGTransports checks every entry serves the protocol its leaf asks for,
// and reports each one it verified.
func assertLGTransports(ctx context.Context, chain *lgPKIChain, entries []lgTransportEntry, phase string) error {
	for _, entry := range entries {
		if entry.servesTLS() {
			if err := assertLGServesHTTPS(ctx, chain, entry); err != nil {
				return fmt.Errorf("%s: %w", phase, err)
			}
			lgPKIOK(phase+", server "+entry.name+" serves HTTPS: a handshake verified against the operator root answers 200 and a plain GET does not", "")
			continue
		}
		if err := assertLGServesHTTP(ctx, entry); err != nil {
			return fmt.Errorf("%s: %w", phase, err)
		}
		lgPKIOK(phase+", server "+entry.name+" serves plain HTTP: a GET answers 200 and a TLS handshake fails", "")
	}
	return nil
}

// assertLGRFC8522 checks the RFC 8522 API answers each entry by the connection
// it arrived on: refused with 400 and the HTTPS-required message on an HTTP
// entry, served with 200 on an HTTPS one of the same looking glass.
func assertLGRFC8522(ctx context.Context, chain *lgPKIChain, entries []lgTransportEntry, phase string) error {
	for _, entry := range entries {
		if entry.servesTLS() {
			status, body, err := lgTransportGet(ctx, entry.port, lgTransportVerified(chain), lgTransportRFC8522Path)
			if err != nil {
				return fmt.Errorf("%s: server %s, %s over TLS: %w", phase, entry.name, lgTransportRFC8522Path, err)
			}
			if status != http.StatusOK {
				return fmt.Errorf("%s: server %s answered %s over TLS with %d, want 200:\n%s", phase, entry.name, lgTransportRFC8522Path, status, body)
			}
			lgPKIOK(phase+", the RFC 8522 API answers 200 on HTTPS server "+entry.name, "")
			continue
		}
		status, body, err := lgTransportGet(ctx, entry.port, nil, lgTransportRFC8522Path)
		if err != nil {
			return fmt.Errorf("%s: server %s, %s over plain HTTP: %w", phase, entry.name, lgTransportRFC8522Path, err)
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("%s: server %s answered %s over plain HTTP with %d, want 400:\n%s", phase, entry.name, lgTransportRFC8522Path, status, body)
		}
		if !strings.Contains(body, lgTransportRFC8522Refusal) {
			return fmt.Errorf("%s: server %s refused %s with 400 but without %q:\n%s", phase, entry.name, lgTransportRFC8522Path, lgTransportRFC8522Refusal, body)
		}
		lgPKIOK(phase+", the RFC 8522 API answers 400, HTTPS required, on HTTP server "+entry.name, "")
	}
	return nil
}

// reloadLGTransport rewrites the config with entries, SIGHUPs the daemon, and
// checks the reload was accepted with no listener rebound.
func reloadLGTransport(ctx context.Context, daemon *lgPKIDaemon, chain *lgPKIChain, entries []lgTransportEntry, flipped lgTransportEntry) error {
	bindsBefore := daemon.binds()
	accepted, err := daemon.reload(ctx, lgTransportConfig(chain, entries))
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("the reload turning server %s to %s was refused:\n%s", flipped.name, flipped.protocol(), daemon.output.String())
	}
	if binds := daemon.binds(); binds != bindsBefore {
		return fmt.Errorf("the reload turning server %s to %s rebound a listener: %d banners before, %d after:\n%s",
			flipped.name, flipped.protocol(), bindsBefore, binds, daemon.output.String())
	}
	lgPKIOK("the reload turned server "+flipped.name+" to "+flipped.protocol()+" with no rebind: listener banners before and after: "+strconv.Itoa(bindsBefore), "")
	return nil
}

// lgTransportPorts reads the two ports the runner leased. A port the fixture
// probed for itself can be taken by another test between the probe and the
// bind.
func lgTransportPorts(args []string) (public, secure int, err error) {
	if len(args) != 2 {
		return 0, 0, fmt.Errorf("usage: the fixture takes two arguments, the ports the runner leased ($PORT $PORT2); got %d", len(args))
	}
	if public, err = strconv.Atoi(args[0]); err != nil {
		return 0, 0, fmt.Errorf("port %q: %w", args[0], err)
	}
	if secure, err = strconv.Atoi(args[1]); err != nil {
		return 0, 0, fmt.Errorf("port %q: %w", args[1], err)
	}
	return public, secure, nil
}

// startLGTransportDaemon starts the daemon and returns once both listeners
// announced their bind, the HTTPS one under https:// and the HTTP one under
// http://. The banner scheme is the plan buildLGService applied.
func startLGTransportDaemon(ctx context.Context, path, config string, public, secure lgTransportEntry) (*lgPKIDaemon, error) {
	daemon, err := startTLSFixtureDaemon(ctx, path, config, secure.port, lgPKIListenBanner)
	if err != nil {
		return nil, err
	}
	var announced textbuf.Buffer
	announced.Str(lgPKIListenBanner).Str(" http://").Str(lgTransportHost).Byte(':').Int(int64(public.port)).Byte('/')
	needle := announced.String()
	ready := Poll(ctx, lgTransportBannerAttempts, lgPKIPollDelay, func() bool {
		return strings.Contains(daemon.output.String(), needle)
	})
	if !ready {
		daemon.stop()
		return nil, fmt.Errorf("the daemon announced the HTTPS listener but never %q:\n%s", needle, daemon.output.String())
	}
	return daemon, nil
}

// lgTransportMixed is `reload/lg-transport-mixed`.
//
// Method: start one daemon whose looking glass names a pki certificate and has
// two server entries, public with `tls false` and secure with the leaf unset.
// Check public serves plain HTTP and refuses a TLS handshake, secure serves
// HTTPS and gives a plain GET no answer, both at once, and the RFC 8522 API
// answers 400 on public and 200 on secure. Then reload twice, each time
// flipping one entry's leaf on an unchanged address: public to `tls true`,
// then secure to `tls false`. After each, the flipped entry serves its new
// protocol, the other keeps its own, and no listener banner was added. The
// RFC 8522 check runs again once the two protocols have traded places.
func lgTransportMixed(ctx context.Context, args []string) error {
	publicPort, securePort, err := lgTransportPorts(args)
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "ze-lg-transport-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

	chain, err := newLGPKIChain(false, lgTransportLeafCN)
	if err != nil {
		return err
	}
	entries := []lgTransportEntry{
		{name: "public", port: publicPort, leaf: lgTransportLeafFalse},
		{name: "secure", port: securePort, leaf: lgTransportLeafDefault},
	}
	daemon, err := startLGTransportDaemon(ctx, filepath.Join(root, "hub.conf"), lgTransportConfig(chain, entries), entries[0], entries[1])
	if err != nil {
		return err
	}
	defer daemon.stop()

	if binds := daemon.binds(); binds != len(entries) {
		return fmt.Errorf("the looking glass printed %d listener banners for %d server entries:\n%s", binds, len(entries), daemon.output.String())
	}
	if strings.Contains(daemon.output.String(), lgTransportDowngradeWarning) {
		return fmt.Errorf("the daemon warned of a downgrade, but every entry got the protocol it asked for:\n%s", daemon.output.String())
	}
	lgPKIOK("the looking glass bound one listener per server entry and warned of no downgrade", "")

	if err := assertLGTransports(ctx, chain, entries, "at start"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.output.String())
	}
	if err := assertLGRFC8522(ctx, chain, entries, "at start"); err != nil {
		return err
	}

	entries[0].leaf = lgTransportLeafTrue
	if err := reloadLGTransport(ctx, daemon, chain, entries, entries[0]); err != nil {
		return err
	}
	if err := assertLGTransports(ctx, chain, entries, "after public turned to tls true"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.output.String())
	}

	entries[1].leaf = lgTransportLeafFalse
	if err := reloadLGTransport(ctx, daemon, chain, entries, entries[1]); err != nil {
		return err
	}
	if err := assertLGTransports(ctx, chain, entries, "after secure turned to tls false"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.output.String())
	}
	if err := assertLGRFC8522(ctx, chain, entries, "after secure turned to tls false"); err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, daemon.output.String())
	return nil
}
