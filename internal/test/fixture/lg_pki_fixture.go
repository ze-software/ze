// Design: docs/architecture/pki/tls-listeners.md -- the looking-glass PKI certificate fixtures

// The three drivers here run a real daemon and a real crypto/tls client. Four
// acceptance criteria are what they prove:
//
//   - AC-1, the chain the listener serves.
//   - AC-11, the bearer gate over that same chain.
//   - AC-6, a reload that rotates the certificate and keeps the socket.
//   - AC-5, a reload that names a certificate the store does not define.
//
// The assertions live here rather than in the .ci files. The observer sandbox
// cannot drive a TLS handshake. `http=get` carries only insecure-tls=true, which
// accepts a chain that it never reads. So compiled code that dials crypto/tls
// itself is the only place the served chain is observable.
//
// register_lg_pki.go registers all three.

package fixture

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// lgPKIServerName is the name every generated leaf carries in its SAN and
	// every client here verifies against. The dial goes to 127.0.0.1, so the
	// name is supplied explicitly rather than derived from the address.
	lgPKIServerName = "localhost"

	// lgPKIToken is the bearer token AC-11 sets beside the certificate.
	lgPKIToken = "lg-pki-s3cret-token" //nolint:gosec // fixture credential, never a deployment value

	// lgPKIStatusPath answers from the daemon's own state and reads no RIB. It
	// therefore says whether the listener served the request.
	lgPKIStatusPath = "/api/looking-glass/status"

	// lgPKIReloadDone and lgPKIReloadFailed are the two stable lines a finished
	// SIGHUP reload prints (cmd/ze/hub/main_reload.go). These fixtures wait for a
	// NEW occurrence of one of them. No duration stands in for that report.
	lgPKIReloadDone   = "sighup reload complete"
	lgPKIReloadFailed = "reload error: "

	// lgPKIListenBanner is logged once for each listener bind. Its count is what
	// proves a rotation did not rebind the socket (AC-6).
	lgPKIListenBanner = "looking glass listening on"

	lgPKIPollAttempts = 150
	lgPKIPollDelay    = 200 * time.Millisecond
)

// lgPKILeaf is one device certificate the pki container can carry: base64 DER
// for both halves, which is the encoding every pki leaf takes.
type lgPKILeaf struct {
	CommonName string
	CertB64    string
	KeyB64     string
}

// lgPKIChain is a generated trust chain. It holds one root CA and one leaf for
// each requested common name. When the caller asks for one, an intermediate sits
// between them, and the store carries it beside the leaf.
//
// Each run generates its own chain. No fixture certificate can expire that way,
// and each leaf's common name is readable in the source. An embedded blob is
// neither.
//
// An empty InterB64 means the root issued the leaves. The config then carries no
// `intermediate` leaf. Only AC-1 asks for the deeper chain. AC-5 and AC-6 are
// about which certificate a reload installs, not about the depth of the store.
type lgPKIChain struct {
	Root     *x509.Certificate
	RootB64  string
	InterCN  string
	InterB64 string
	Leaves   []lgPKILeaf
	RootPool *x509.CertPool
}

// served answers how many certificates a listener presenting this chain sends:
// the leaf, plus the intermediate when the store holds one.
func (c *lgPKIChain) served() int {
	if c.InterB64 == "" {
		return 1
	}
	return 2
}

// newLGPKIChain builds the chain, one leaf for each common name. withIntermediate
// puts an intermediate CA between the root and the leaves.
func newLGPKIChain(withIntermediate bool, leafCommonNames ...string) (*lgPKIChain, error) {
	rootCert, rootKey, rootDER, err := lgPKIIssueCA("ze looking glass root", 1, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("root CA: %w", err)
	}

	chain := &lgPKIChain{
		Root:     rootCert,
		RootB64:  base64.StdEncoding.EncodeToString(rootDER),
		RootPool: x509.NewCertPool(),
	}
	chain.RootPool.AddCert(rootCert)

	issuer, issuerKey := rootCert, rootKey
	if withIntermediate {
		const interCN = "ze looking glass intermediate"
		interCert, interKey, interDER, interErr := lgPKIIssueCA(interCN, 2, rootCert, rootKey)
		if interErr != nil {
			return nil, fmt.Errorf("intermediate CA: %w", interErr)
		}
		chain.InterCN = interCN
		chain.InterB64 = base64.StdEncoding.EncodeToString(interDER)
		issuer, issuerKey = interCert, interKey
	}

	for index, commonName := range leafCommonNames {
		leaf, err := lgPKIIssueLeaf(commonName, int64(index+3), issuer, issuerKey)
		if err != nil {
			return nil, fmt.Errorf("leaf %q: %w", commonName, err)
		}
		chain.Leaves = append(chain.Leaves, leaf)
	}
	return chain, nil
}

// lgPKIIssueCA signs one CA certificate. A nil parent makes it self-signed,
// which is the root case.
func lgPKIIssueCA(commonName string, serial int64, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	issuer := template
	signingKey := key
	if parent != nil {
		issuer = parent
		signingKey = parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, &key.PublicKey, signingKey)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}
	return cert, key, der, nil
}

// lgPKIIssueLeaf signs one server certificate under the intermediate and encodes
// both halves the way the pki container takes them.
func lgPKIIssueLeaf(commonName string, serial int64, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey) (lgPKILeaf, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return lgPKILeaf{}, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{lgPKIServerName},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		return lgPKILeaf{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return lgPKILeaf{}, err
	}
	return lgPKILeaf{
		CommonName: commonName,
		CertB64:    base64.StdEncoding.EncodeToString(der),
		KeyB64:     base64.StdEncoding.EncodeToString(keyDER),
	}, nil
}

// lgPKIConfig renders one whole daemon config: the trust chain in the pki
// container under the given store names, and a looking glass pointed at
// certificate. An empty token leaves the looking glass open.
//
// storeNames is index-aligned with the chain's leaves. A caller can therefore
// give one leaf a different name in a second config, and watch what the daemon
// does with the reference.
func lgPKIConfig(chain *lgPKIChain, storeNames []string, certificate, token string, port int) string {
	var tb textbuf.Buffer
	tb.Str("plugin {\n}\n\npki {\n\tca lg-root {\n\t\tcertificate ").Str(chain.RootB64).Str("\n\t}\n")
	for index, leaf := range chain.Leaves {
		tb.Str("\tcertificate ").Str(storeNames[index]).Str(" {\n")
		tb.Str("\t\tcertificate ").Str(leaf.CertB64).Byte('\n')
		if chain.InterB64 != "" {
			tb.Str("\t\tintermediate ").Str(chain.InterB64).Byte('\n')
		}
		tb.Str("\t\tprivate {\n\t\t\tkey ").Str(leaf.KeyB64).Str("\n\t\t}\n\t}\n")
	}
	tb.Str("}\n\nenvironment {\n\tlooking-glass {\n\t\tenabled true\n")
	tb.Str("\t\tcertificate ").Str(certificate).Byte('\n')
	if token != "" {
		tb.Str("\t\ttoken ").Str(token).Byte('\n')
	}
	tb.Str("\t\tserver main {\n\t\t\tip 127.0.0.1;\n\t\t\tport ").Int(int64(port)).Str(";\n\t\t}\n\t}\n}\n")
	return tb.String()
}

// lgPKIDaemon is one running ze these fixtures signal and read.
//
// Its output is captured rather than inherited, because every fence here waits
// on a line the daemon printed: the listener banner, and the reload verdict. A
// fixture that cannot read the daemon's own report has nothing to wait on but
// elapsed time.
type lgPKIDaemon struct {
	command   *exec.Cmd
	done      chan error
	output    *lockedBuffer
	path      string
	port      int
	configDir string
}

// startLGPKIDaemon writes the config, starts ze, and returns once the looking
// glass has announced its HTTPS listener.
//
// No `ze init` runs, so the deployment has no blob storage. A named certificate
// must serve there anyway, because its material comes from the pki container.
func startLGPKIDaemon(ctx context.Context, path, config string, port int) (*lgPKIDaemon, error) {
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return nil, err
	}
	configDir, err := os.MkdirTemp("", "lg-pki-daemon-")
	if err != nil {
		return nil, err
	}
	daemon := &lgPKIDaemon{output: &lockedBuffer{}, path: path, port: port, configDir: configDir}
	command := exec.CommandContext(ctx, "ze", actionStart, path) //nolint:gosec // the fixture chooses the program and its arguments
	command.Env = miscEnvironment(map[string]string{envConfigDir: configDir})
	command.Stdout = daemon.output
	command.Stderr = daemon.output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		_ = os.RemoveAll(configDir) //nolint:errcheck // the start already failed
		return nil, err
	}
	daemon.command = command
	daemon.done = make(chan error, 1)
	// Lifecycle goroutine (one-time Process.Wait bridge): Wait must be called
	// exactly once, and stop() reads the channel it sends to.
	go daemon.reap()

	var banner textbuf.Buffer
	banner.Str(lgPKIListenBanner).Str(" https://127.0.0.1:").Int(int64(port)).Byte('/')
	needle := banner.String()
	ready := Poll(ctx, lgPKIPollAttempts, lgPKIPollDelay, func() bool {
		return strings.Contains(daemon.output.String(), needle)
	})
	if !ready {
		daemon.stop()
		return nil, fmt.Errorf("the looking glass never announced %q:\n%s", needle, daemon.output.String())
	}
	return daemon, nil
}

// reap is the body of the lifecycle goroutine startLGPKIDaemon starts. It calls
// Wait once and publishes the status on the channel stop() reads.
func (d *lgPKIDaemon) reap() {
	d.done <- d.command.Wait()
}

// stop stops the daemon's process group and releases its config directory.
func (d *lgPKIDaemon) stop() {
	if d.command != nil {
		stopManagedProcess(d.command, d.done)
	}
	if d.configDir != "" {
		_ = os.RemoveAll(d.configDir) //nolint:errcheck // fixture cleanup
	}
}

// reload rewrites the config file, sends SIGHUP, and returns once the daemon has
// printed a NEW verdict line. accepted says which verdict arrived, so a caller
// can assert the refusal as precisely as the success.
//
// Counting the verdicts already printed is what makes this a fence. A substring
// search alone cannot tell the second reload of a run from the first.
func (d *lgPKIDaemon) reload(ctx context.Context, config string) (accepted bool, err error) {
	acceptedBefore := strings.Count(d.output.String(), lgPKIReloadDone)
	verdictsBefore := d.verdicts()
	if err := os.WriteFile(d.path, []byte(config), 0o600); err != nil {
		return false, err
	}
	if err := syscall.Kill(d.command.Process.Pid, syscall.SIGHUP); err != nil {
		return false, err
	}
	settled := Poll(ctx, lgPKIPollAttempts, lgPKIPollDelay, func() bool { return d.verdicts() > verdictsBefore })
	if !settled {
		return false, fmt.Errorf("the daemon printed no reload verdict:\n%s", d.output.String())
	}
	return strings.Count(d.output.String(), lgPKIReloadDone) > acceptedBefore, nil
}

// verdicts counts the reload verdicts printed so far, of either kind.
func (d *lgPKIDaemon) verdicts() int {
	text := d.output.String()
	return strings.Count(text, lgPKIReloadDone) + strings.Count(text, lgPKIReloadFailed)
}

// binds counts the listener banners printed so far. A rotation must not add one:
// the new chain goes onto the running server and the socket is kept.
func (d *lgPKIDaemon) binds() int {
	return strings.Count(d.output.String(), lgPKIListenBanner)
}

// lgPKIConn is one TLS connection to the looking glass, with the reader that
// owns whatever the last response left buffered. One reader for each connection
// is what lets a caller hold a connection open across a reload and use it again.
type lgPKIConn struct {
	conn   *tls.Conn
	reader *bufio.Reader
}

// dialLGPKI handshakes with the looking glass as a client that trusts only the
// operator's root. The handshake therefore VERIFIES. It succeeds only if the
// listener sent enough of the chain for the client to build a path. That is what
// an operator publishes a looking glass to get.
func dialLGPKI(ctx context.Context, port int, roots *x509.CertPool) (*lgPKIConn, error) {
	dialer := &tls.Dialer{Config: &tls.Config{
		RootCAs:    roots,
		ServerName: lgPKIServerName,
		MinVersion: tls.VersionTLS12,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("TLS handshake with the looking glass on port %d: %w", port, err)
	}
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		_ = conn.Close() //nolint:errcheck // the type is wrong, so the close result cannot help
		return nil, fmt.Errorf("dialed connection is %T, not TLS", conn)
	}
	return &lgPKIConn{conn: tlsConn, reader: bufio.NewReader(tlsConn)}, nil
}

// close releases the connection.
func (c *lgPKIConn) close() {
	_ = c.conn.Close() //nolint:errcheck // test client teardown
}

// chain answers the certificates the listener presented, leaf first.
func (c *lgPKIConn) chain() []*x509.Certificate {
	return c.conn.ConnectionState().PeerCertificates
}

// get sends one keep-alive request over this connection and answers the status.
// The body is drained so the connection stays usable for the next request.
func (c *lgPKIConn) get(port int, path, authorization string) (int, error) {
	var url textbuf.Buffer
	url.Str("https://127.0.0.1:").Int(int64(port)).Str(path)
	request, err := http.NewRequest(http.MethodGet, url.String(), http.NoBody) //nolint:noctx // the deadline belongs to the connection the caller holds
	if err != nil {
		return 0, err
	}
	request.Header.Set("Connection", "keep-alive")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if err := request.Write(c.conn); err != nil {
		return 0, fmt.Errorf("write %s: %w", path, err)
	}
	response, err := http.ReadResponse(c.reader, request)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		_ = response.Body.Close() //nolint:errcheck // the read already failed
		return 0, err
	}
	if err := response.Body.Close(); err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

// lgPKIAssertServedChain checks that one connection carries the store leaf
// first. When the store holds an intermediate, that same intermediate must come
// second, and it must be the certificate that signed the leaf.
func lgPKIAssertServedChain(conn *lgPKIConn, chain *lgPKIChain, leafCN string) error {
	presented := conn.chain()
	if len(presented) != chain.served() {
		return fmt.Errorf("the looking glass served %d certificates, want %d", len(presented), chain.served())
	}
	if presented[0].Subject.CommonName != leafCN {
		return fmt.Errorf("the served leaf CN is %q, want the store leaf %q", presented[0].Subject.CommonName, leafCN)
	}
	if chain.InterB64 == "" {
		return nil
	}
	if presented[1].Subject.CommonName != chain.InterCN {
		return fmt.Errorf("the second served certificate CN is %q, want the store intermediate %q", presented[1].Subject.CommonName, chain.InterCN)
	}
	if err := presented[0].CheckSignatureFrom(presented[1]); err != nil {
		return fmt.Errorf("the served leaf is not signed by the certificate sent after it: %w", err)
	}
	return nil
}

// lgPKIOK prints one passing assertion, naming the value it checked.
func lgPKIOK(text, value string) {
	var tb textbuf.Buffer
	tb.Str("OK: ").Str(text)
	if value != "" {
		tb.Byte(' ').Quoted(value)
	}
	tb.Byte('\n')
	_ = tb.StdErr() //nolint:errcheck // a failed write to stderr has nowhere left to be reported
}

// lgPKICertificateServed is `plugin/lg-pki-certificate`.
//
// One deployment carries both halves of the operator story. The looking glass
// serves the named store certificate with its intermediate (AC-1). The bearer
// gate still refuses an unauthenticated request over that same chain (AC-11).
// They are one scenario because R-7 is the risk that one clears the other.
func lgPKICertificateServed(ctx context.Context, _ []string) error {
	root, err := os.MkdirTemp("", "ze-lg-pki-served-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

	const leafCN = "ze looking glass public leaf"
	chain, err := newLGPKIChain(true, leafCN)
	if err != nil {
		return err
	}
	port, err := availablePort09()
	if err != nil {
		return err
	}
	config := lgPKIConfig(chain, []string{"lg-public"}, "lg-public", lgPKIToken, port)
	daemon, err := startLGPKIDaemon(ctx, filepath.Join(root, "hub.conf"), config, port)
	if err != nil {
		return err
	}
	defer daemon.stop()

	conn, err := dialLGPKI(ctx, port, chain.RootPool)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.output.String())
	}
	defer conn.close()
	if err := lgPKIAssertServedChain(conn, chain, leafCN); err != nil {
		return err
	}
	lgPKIOK("the looking glass serves the named store leaf first and its intermediate second:", leafCN)
	// The handshake above verified against the operator's root and nothing else.
	// A client holding only that root therefore built the whole path from what
	// the listener sent. That is the browser warning this feature removes, and it
	// is stated once here.
	lgPKIOK("a client trusting only the operator root validated the served chain", "")

	for _, path := range []string{lgPKIStatusPath, "/lg/peers"} {
		status, err := conn.get(port, path, "")
		if err != nil {
			return fmt.Errorf("%s without a token: %w", path, err)
		}
		if status != http.StatusUnauthorized {
			return fmt.Errorf("%s without a token answered %d over the named chain, want 401", path, status)
		}
		status, err = conn.get(port, path, "Bearer wrong-token")
		if err != nil {
			return fmt.Errorf("%s with a wrong token: %w", path, err)
		}
		if status != http.StatusUnauthorized {
			return fmt.Errorf("%s with a wrong token answered %d over the named chain, want 401", path, status)
		}
		status, err = conn.get(port, path, "Bearer "+lgPKIToken)
		if err != nil {
			return fmt.Errorf("%s with the configured token: %w", path, err)
		}
		if status == http.StatusUnauthorized {
			return fmt.Errorf("%s refused the configured token over the named chain", path)
		}
	}
	lgPKIOK("the bearer gate refuses an unauthenticated request over the named chain", "")
	fmt.Fprint(os.Stderr, daemon.output.String())
	return nil
}

// lgPKIReferenceReload is `reload/lg-pki-reference-reload` (AC-6).
//
// A viewer holds an open connection while the operator renews the certificate.
// The reload must rotate the served chain, keep the socket, and leave the open
// connection carrying data.
func lgPKIReferenceReload(ctx context.Context, _ []string) error {
	root, err := os.MkdirTemp("", "ze-lg-pki-reload-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

	const (
		firstCN  = "ze looking glass leaf one"
		secondCN = "ze looking glass leaf two"
	)
	chain, err := newLGPKIChain(false, firstCN, secondCN)
	if err != nil {
		return err
	}
	port, err := availablePort09()
	if err != nil {
		return err
	}
	names := []string{"lg-first", "lg-second"}
	daemon, err := startLGPKIDaemon(ctx, filepath.Join(root, "hub.conf"),
		lgPKIConfig(chain, names, "lg-first", "", port), port)
	if err != nil {
		return err
	}
	defer daemon.stop()

	// The viewer whose session the rotation must not drop. It is opened before
	// the reload and used again after it.
	held, err := dialLGPKI(ctx, port, chain.RootPool)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.output.String())
	}
	defer held.close()
	if err := lgPKIAssertServedChain(held, chain, firstCN); err != nil {
		return err
	}
	status, err := held.get(port, lgPKIStatusPath, "")
	if err != nil {
		return fmt.Errorf("the held connection before the reload: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("the held connection answered %d before the reload, want 200", status)
	}
	bindsBefore := daemon.binds()
	lgPKIOK("a viewer holds an open connection to the looking glass, which serves", firstCN)

	accepted, err := daemon.reload(ctx, lgPKIConfig(chain, names, "lg-second", "", port))
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("the rotation reload was refused:\n%s", daemon.output.String())
	}

	fresh, err := dialLGPKI(ctx, port, chain.RootPool)
	if err != nil {
		return fmt.Errorf("handshake after the rotation: %w\n%s", err, daemon.output.String())
	}
	defer fresh.close()
	if err := lgPKIAssertServedChain(fresh, chain, secondCN); err != nil {
		return fmt.Errorf("after the rotation: %w", err)
	}
	lgPKIOK("the next handshake receives the rotated chain, leaf", secondCN)

	if daemon.binds() != bindsBefore {
		return fmt.Errorf("the looking glass rebound its listener: %d banners before the rotation, %d after:\n%s",
			bindsBefore, daemon.binds(), daemon.output.String())
	}
	lgPKIOK("the listener address is unchanged and the socket was never rebound", "")

	status, err = held.get(port, lgPKIStatusPath, "")
	if err != nil {
		return fmt.Errorf("the held connection after the reload: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("the held connection answered %d after the reload, want 200", status)
	}
	lgPKIOK("the connection held across the rotation still carries data", "")
	fmt.Fprint(os.Stderr, daemon.output.String())
	return nil
}

// lgPKIReferenceReloadBroken is `reload/lg-pki-reference-reload-broken` (AC-5).
//
// A reload naming a certificate the store does not define is refused, and the
// looking glass keeps serving the chain it had.
//
// The store restoration itself is not observable from outside the process. Every
// reload re-derives the store from the file. No later config can therefore tell a
// restored store from a re-installed one, and
// `TestReloadRejectsBrokenLGCertificateReference` pins that half.
//
// What this proves is the operator's half. The refusal names the leaf and the
// missing name. The served chain does not move. The daemon still commits the
// corrected config, so the rejection did not wedge it.
func lgPKIReferenceReloadBroken(ctx context.Context, _ []string) error {
	root, err := os.MkdirTemp("", "ze-lg-pki-broken-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root) //nolint:errcheck // fixture cleanup

	const (
		firstCN  = "ze looking glass leaf one"
		secondCN = "ze looking glass leaf two"
	)
	chain, err := newLGPKIChain(false, firstCN, secondCN)
	if err != nil {
		return err
	}
	port, err := availablePort09()
	if err != nil {
		return err
	}
	names := []string{"lg-first", "lg-second"}
	daemon, err := startLGPKIDaemon(ctx, filepath.Join(root, "hub.conf"),
		lgPKIConfig(chain, names, "lg-first", "", port), port)
	if err != nil {
		return err
	}
	defer daemon.stop()

	before, err := dialLGPKI(ctx, port, chain.RootPool)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.output.String())
	}
	defer before.close()
	if err := lgPKIAssertServedChain(before, chain, firstCN); err != nil {
		return err
	}
	lgPKIOK("the looking glass serves this leaf before the mistyped reload:", firstCN)

	accepted, err := daemon.reload(ctx, lgPKIConfig(chain, names, "no-such-certificate", "", port))
	if err != nil {
		return err
	}
	if accepted {
		return fmt.Errorf("the reload naming an undefined certificate was accepted:\n%s", daemon.output.String())
	}
	text := daemon.output.String()
	for _, needle := range []string{"environment.looking-glass.certificate", "no-such-certificate"} {
		if !strings.Contains(text, needle) {
			return fmt.Errorf("the refusal never named %q:\n%s", needle, text)
		}
	}
	lgPKIOK("the reload is refused, naming environment.looking-glass.certificate and the missing name", "")

	after, err := dialLGPKI(ctx, port, chain.RootPool)
	if err != nil {
		return fmt.Errorf("handshake after the refused reload: %w\n%s", err, daemon.output.String())
	}
	defer after.close()
	if err := lgPKIAssertServedChain(after, chain, firstCN); err != nil {
		return fmt.Errorf("after the refused reload: %w", err)
	}
	status, err := after.get(port, lgPKIStatusPath, "")
	if err != nil {
		return fmt.Errorf("the looking glass after the refused reload: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("the looking glass answered %d after the refused reload, want 200", status)
	}
	lgPKIOK("the looking glass keeps serving its previous chain and answering requests:", firstCN)

	// The refusal must leave the daemon able to commit the operator's next
	// attempt, which is what they make once they have read the error.
	accepted, err = daemon.reload(ctx, lgPKIConfig(chain, names, "lg-second", "", port))
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("the corrected reload was refused too:\n%s", daemon.output.String())
	}
	corrected, err := dialLGPKI(ctx, port, chain.RootPool)
	if err != nil {
		return fmt.Errorf("handshake after the corrected reload: %w\n%s", err, daemon.output.String())
	}
	defer corrected.close()
	if err := lgPKIAssertServedChain(corrected, chain, secondCN); err != nil {
		return fmt.Errorf("after the corrected reload: %w", err)
	}
	lgPKIOK("the operator's corrected reload commits and serves", secondCN)
	fmt.Fprint(os.Stderr, daemon.output.String())
	return nil
}
