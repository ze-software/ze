// VALIDATES: the leaf a hub acceptor serves is REISSUED before it expires, so a
// daemon that runs longer than one leaf lives keeps completing handshakes, and
// concurrent handshakes that arrive at the renewal instant issue one leaf
// between them rather than one each.
// PREVENTS: the 24-hour cliff -- a certificate issued once at construction,
// with no tls.Config.GetCertificate on the listener, which every plugin
// connect-back and every managed client refuses from the second day with the
// operator's config unchanged and nothing naming the cause.
package plugin_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/sim"
)

// leafLife is the lifetime clockedAuthority stamps on each leaf. It matches the
// 24 hours pki.IssueLeaf uses, so the test moves the same cliff production has.
const leafLife = 24 * time.Hour

// authorityClockSkew backdates NotBefore the way pki.IssueLeaf does, so the
// renewal deadline this test computes is the one production computes.
const authorityClockSkew = 5 * time.Minute

// clockedAuthority is a certificate authority whose leaves are stamped from a
// FAKE clock. The signing is real (a real ECDSA root, a real
// x509.CreateCertificate, a real chain the client verifies with crypto/x509),
// and only the wall clock is controlled: pki.Root reads time.Now directly, so
// a test holding it could not tell a reissued leaf from the first one.
//
// Safe for concurrent use, which the concurrency test depends on: it counts
// issuances from several goroutines at once.
type clockedAuthority struct {
	root    *x509.Certificate
	rootDER []byte
	key     *ecdsa.PrivateKey
	clk     *sim.FakeClock

	mu     sync.Mutex
	issued int
}

func newClockedAuthority(t *testing.T, clk *sim.FakeClock) *clockedAuthority {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the test root key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ze-test-clocked-ca"},
		NotBefore:             clk.Now().Add(-authorityClockSkew),
		NotAfter:              clk.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create the test root: %v", err)
	}
	root, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse the test root: %v", err)
	}
	return &clockedAuthority{root: root, rootDER: der, key: key, clk: clk}
}

// IssueLeaf signs a leaf whose validity window is read from the fake clock.
func (a *clockedAuthority) IssueLeaf(commonName string, hosts []string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	a.mu.Lock()
	a.issued++
	serial := big.NewInt(int64(a.issued + 1))
	a.mu.Unlock()

	now := a.clk.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-authorityClockSkew),
		NotAfter:              now.Add(leafLife),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		template.DNSNames = append(template.DNSNames, host)
	}

	der, err := x509.CreateCertificate(rand.Reader, template, a.root, &key.PublicKey, a.key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

// CertificatePEM answers the root a peer validates the served leaf against.
func (a *clockedAuthority) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.rootDER})
}

func (a *clockedAuthority) issuances() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.issued
}

func (a *clockedAuthority) pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(a.root)
	return pool
}

// handshakeAt completes a real TLS handshake against addr, judging the served
// certificate at the fake clock's instant. tls.Config.Time is what makes the
// certificate's own expiry reachable in a test: crypto/x509 compares NotAfter
// against it, so an expired leaf is refused here exactly as a peer refuses it.
func handshakeAt(ctx context.Context, addr string, pool *x509.CertPool, clk *sim.FakeClock) (*x509.Certificate, error) {
	dialer := &tls.Dialer{Config: &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
		Time:       clk.Now,
	}}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer raw.Close() //nolint:errcheck // the caller judges the handshake, not the close

	conn, ok := raw.(*tls.Conn)
	if !ok {
		return nil, nil
	}
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, nil
	}
	return state.PeerCertificates[0], nil
}

// emptyAuthority answers a certificate with no DER, which completes no
// handshake. crypto/tls reports it as "no certificates configured" at the first
// connection, a process away from the authority that produced it.
type emptyAuthority struct{ *clockedAuthority }

func (emptyAuthority) IssueLeaf(string, []string) (tls.Certificate, error) {
	return tls.Certificate{}, nil
}

// TestHubAcceptorRefusesAnEmptyCertificate: the guard fires where the caller can
// be named, at construction, rather than at the first peer.
//
// MUTATION: drop the len(cert.Certificate) check in issueLocked and the
// acceptor starts, then refuses every connection with no cause named here.
func TestHubAcceptorRefusesAnEmptyCertificate(t *testing.T) {
	clk := sim.NewFakeClock(time.Now())
	ca := newClockedAuthority(t, clk)

	acceptor, err := plugin.NewHubAcceptor(nil, emptyAuthority{ca}, clk)
	if err == nil {
		acceptor.Stop()
		t.Fatal("an authority that issues an empty certificate must be an error at construction")
	}
	if acceptor != nil {
		t.Fatal("a failed NewHubAcceptor must return no acceptor")
	}
}

func TestHubAcceptorReissuesBeforeTheLeafExpires(t *testing.T) {
	clk := sim.NewFakeClock(time.Now())
	ca := newClockedAuthority(t, clk)

	acceptor, err := plugin.NewHubAcceptor(nil, ca, clk)
	if err != nil {
		t.Fatalf("NewHubAcceptor: %v", err)
	}
	t.Cleanup(acceptor.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := acceptor.Addr().String()
	first, err := handshakeAt(ctx, addr, ca.pool(), clk)
	if err != nil {
		t.Fatalf("the first handshake failed: %v", err)
	}
	if first == nil {
		t.Fatal("the acceptor presented no certificate")
	}

	// One hour past the leaf's life. A daemon at this age has not restarted and
	// the operator has changed nothing, so what it presents is the whole test.
	clk.Add(leafLife + time.Hour)

	second, err := handshakeAt(ctx, addr, ca.pool(), clk)
	if err != nil {
		t.Fatalf("a handshake %v after start failed, so the acceptor served an expired leaf: %v",
			leafLife+time.Hour, err)
	}
	if second == nil {
		t.Fatal("the acceptor presented no certificate after the leaf's life")
	}
	if second.SerialNumber.Cmp(first.SerialNumber) == 0 {
		t.Fatal("the acceptor served the same leaf after it expired, rather than reissuing")
	}
	if !clk.Now().Before(second.NotAfter) {
		t.Fatalf("the reissued leaf is already expired: NotAfter %v, now %v", second.NotAfter, clk.Now())
	}
	if ca.issuances() != 2 {
		t.Fatalf("issuances = %d, want 2: one at start and one renewal", ca.issuances())
	}
}

// TestHubAcceptorIssuesOneLeafForConcurrentHandshakes: several peers reconnect
// at once when a hub comes back, so the renewal instant is exactly when the
// listener is busiest. One leaf is owed between them, not one each.
func TestHubAcceptorIssuesOneLeafForConcurrentHandshakes(t *testing.T) {
	clk := sim.NewFakeClock(time.Now())
	ca := newClockedAuthority(t, clk)

	acceptor, err := plugin.NewHubAcceptor(nil, ca, clk)
	if err != nil {
		t.Fatalf("NewHubAcceptor: %v", err)
	}
	t.Cleanup(acceptor.Stop)

	clk.Add(leafLife + time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const peers = 16
	addr := acceptor.Addr().String()
	pool := ca.pool()

	var wg sync.WaitGroup
	errs := make(chan error, peers)
	for range peers {
		wg.Go(func() {
			if _, hErr := handshakeAt(ctx, addr, pool, clk); hErr != nil {
				errs <- hErr
			}
		})
	}
	wg.Wait()
	close(errs)

	for hErr := range errs {
		t.Fatalf("a concurrent handshake failed: %v", hErr)
	}
	if ca.issuances() != 2 {
		t.Fatalf("issuances = %d, want 2: %d concurrent handshakes must share one renewal", ca.issuances(), peers)
	}
}
