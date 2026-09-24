// Design: docs/architecture/plugin/rib-storage-design.md -- authenticated RTR cache sessions.
// Related: rtr_session.go -- connection, handshake and cache publication.
package rpki

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/pki"
)

// TestRTRTLSAuthenticatedCache transfers actual RTR v2 records over each TLS
// version, checking both the router's published state and the cache's peer chain.
func TestRTRTLSAuthenticatedCache(t *testing.T) {
	// RFC requirement: RFC8210-9.2-11 positive -- an authenticated DNS-ID cache completes the TLS handshake and supplies usable VRPs and ASPAs.
	// RFC requirement: RFC8210-3-1 positive -- the named trust anchor and mutually authenticated transport precede cache use.
	// RFC requirement: RFC8210-9-3 positive -- a configured protected transport carries the actual RTR exchange.
	// RFC requirement: RFC8210-9.2-1 positive -- the cache authenticates the configured router leaf and intermediate chain.
	// RFC requirement: RFC8210-9.2-2 positive -- the presented router certificate carries its source address as an iPAddress SAN.
	// RFC requirement: RFC8210-9.2-4 positive -- a trusted certificate's matching dNSName authenticates the cache.
	// RFC requirement: RFC8210-9.2-5 positive -- a differing Common Name does not override a correct DNS-ID.
	// RFC requirement: RFC8210-9.2-10 positive -- authentication uses the matching DNS-ID, not the differing Common Name.
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		t.Run(tls.VersionName(version), func(t *testing.T) {
			fixture := newRTRTLSFixture(t)
			installRTRTLSPKI(t, fixture.store)
			server := fixture.serverConfig(version)
			peer := startRTRTLSPeer(t, server)
			session := parsedRTRTLSSession(t, "127.0.0.1", peer.port, rtrTLSConfigLeaves(), make(chan struct{}))
			if err := session.syncOnce(); err != nil {
				t.Fatalf("sync authenticated cache: %v", err)
			}
			observed := peer.stop()
			if len(observed) != 1 {
				t.Fatalf("connections = %d, want one authenticated exchange", len(observed))
			}
			if observed[0].err != nil {
				t.Fatalf("cache exchange: %v", observed[0].err)
			}
			if observed[0].state.Version != version {
				t.Errorf("negotiated TLS version = %x, want %x", observed[0].state.Version, version)
			}
			if observed[0].queryType != pduResetQuery {
				t.Errorf("a new cache session opened with query type %d", observed[0].queryType)
			}
			if observed[0].state.ServerName != "cache.rtr.test" {
				t.Errorf("cache received SNI %q", observed[0].state.ServerName)
			}
			chain := observed[0].state.PeerCertificates
			want := fixture.store.Certificates["router"]
			if len(chain) != 1+len(want.RawIntermediates) {
				t.Fatalf("cache received %d certificates, want leaf and both intermediates", len(chain))
			}
			if !bytes.Equal(chain[0].Raw, want.Raw) {
				t.Error("cache authenticated a different router certificate")
			}
			for index, intermediate := range want.RawIntermediates {
				if !bytes.Equal(chain[index+1].Raw, intermediate) {
					t.Errorf("cache received the wrong intermediate at position %d", index+1)
				}
			}
			if session.cache.Validate("192.0.2.0/24", 64500) != ValidationValid {
				t.Error("authenticated cache VRP was not published")
			}
			if !session.aspaCache.isProvider(64500, 64501) {
				t.Error("authenticated cache ASPA was not published")
			}
		})
	}
}

// TestRTRTLSRejectsUnauthenticatedCaches observes application bytes at real TLS
// peers: neither certificate failures nor one-way TLS may reach the RTR query.
func TestRTRTLSRejectsUnauthenticatedCaches(t *testing.T) {
	// RFC requirement: RFC8210-9.2-11 negative -- a wrong DNS-ID, a CN-only identity and an IP-SAN-only identity never authenticate the cache or publish its records.
	// RFC requirement: RFC8210-3-1 negative -- an issuer outside the named CA and a cache declining client authentication cannot supply trusted routing data.
	// RFC requirement: RFC8210-9.2-4 negative -- DNS mismatch and a certificate without dNSName SAN fail before cache publication.
	// RFC requirement: RFC8210-9.2-5 negative -- a matching CN without DNS-ID cannot authenticate a cache.
	// RFC requirement: RFC8210-9.2-10 negative -- a matching Common Name is never an authentication fallback.
	// RFC requirement: RFC8210-9.2-8 negative -- the configured DNS reference, not the connected IP or another certificate name, controls authentication.
	for _, name := range []string{"wrong DNS", "CN only", "IP SAN only", "untrusted issuer", "no client authentication"} {
		t.Run(name, func(t *testing.T) {
			fixture := newRTRTLSFixture(t)
			server := fixture.serverConfig(tls.VersionTLS13)
			leaves := rtrTLSConfigLeaves()
			switch name {
			case "wrong DNS":
				leaves["server-name"] = "other.rtr.test"
			case "CN only":
				server.Certificates = []tls.Certificate{fixture.issueServer(t, nil, nil)}
			case "IP SAN only":
				server.Certificates = []tls.Certificate{fixture.issueServer(t, nil, []net.IP{net.ParseIP("127.0.0.1")})}
			case "untrusted issuer":
				// This CA is installed too, but it was not named for this cache.
				server.Certificates = []tls.Certificate{fixture.serverUntrusted}
			case "no client authentication":
				server.ClientAuth = tls.NoClientCert
			}
			installRTRTLSPKI(t, fixture.store)
			peer := startRTRTLSPeer(t, server)
			session := parsedRTRTLSSession(t, "127.0.0.1", peer.port, leaves, make(chan struct{}))
			assertRTRTLSRefused(t, session, peer)
			observed := peer.stop()
			if len(observed) != 1 {
				t.Fatalf("connections = %d, want one rejected TLS peer", len(observed))
			}
			if name == "no client authentication" {
				if !observed[0].state.HandshakeComplete {
					t.Error("fixture did not complete the one-way TLS handshake")
				}
			}
		})
	}
}

// TestRTRTLSRejectsUnusableClientIdentity leaves a working alternate identity in
// PKI so a missing or unusable named identity cannot silently select another one.
func TestRTRTLSRejectsUnusableClientIdentity(t *testing.T) {
	// RFC requirement: RFC8210-9.2-1 negative -- absent or unusable named router credentials cannot produce a client-authenticated session.
	// RFC requirement: RFC8210-9.2-2 negative -- a router identity without an iPAddress SAN cannot be used for RTR TLS.
	for _, name := range []string{"missing CA", "missing certificate", "missing private key", "mismatched private key", "wrong extended usage", "missing IP SAN"} {
		t.Run(name, func(t *testing.T) {
			fixture := newRTRTLSFixture(t)
			client := fixture.store.Certificates["router"]
			fixture.store.Certificates["alternate-router"] = client
			leaves := rtrTLSConfigLeaves()
			switch name {
			case "missing CA":
				leaves["ca-certificate"] = "absent-ca"
			case "missing certificate":
				leaves["certificate"] = "absent-router"
			case "missing private key":
				entry := *client
				entry.PrivateKey = nil
				fixture.store.Certificates["router"] = &entry
			case "mismatched private key":
				entry := *client
				entry.PrivateKey = fixture.issuerKey
				fixture.store.Certificates["router"] = &entry
			case "wrong extended usage":
				fixture.store.Certificates["router"] = fixture.issueClient(t, x509.ExtKeyUsageServerAuth, true)
			case "missing IP SAN":
				fixture.store.Certificates["router"] = fixture.issueClient(t, x509.ExtKeyUsageClientAuth, false)
			}
			installRTRTLSPKI(t, fixture.store)
			peer := startRTRTLSPeer(t, fixture.serverConfig(tls.VersionTLS12))
			session := parsedRTRTLSSession(t, "127.0.0.1", peer.port, leaves, make(chan struct{}))
			assertRTRTLSRefused(t, session, peer)
		})
	}
}

func TestRTRTLSMissingCandidateDoesNotUseLivePKI(t *testing.T) {
	fixture := newRTRTLSFixture(t)
	installRTRTLSPKI(t, fixture.store)
	peer := startRTRTLSPeer(t, fixture.serverConfig(tls.VersionTLS13))
	session := parsedRTRTLSSession(t, "127.0.0.1", peer.port, rtrTLSConfigLeaves(), make(chan struct{}))
	session.pkiConfig = nil
	assertRTRTLSRefused(t, session, peer)
}

// TestRTRTLSNoPlaintextFallback offers an RTR-speaking TCP server at the TLS
// address. It records queries on every connection, including any retry.
func TestRTRTLSNoPlaintextFallback(t *testing.T) {
	// RFC requirement: RFC8210-9-3 negative -- a selected protected transport never falls back to an offered plaintext RTR stream.
	fixture := newRTRTLSFixture(t)
	installRTRTLSPKI(t, fixture.store)
	peer := startRTRTLSPeer(t, nil)
	session := parsedRTRTLSSession(t, "127.0.0.1", peer.port, rtrTLSConfigLeaves(), make(chan struct{}))
	assertRTRTLSRefused(t, session, peer)
}

// TestRTRTLSConfigRejectsUnsafeTransport exercises the parser used by the BGP
// plugin, rather than constructing invalid TLS settings directly.
func TestRTRTLSConfigRejectsUnsafeTransport(t *testing.T) {
	for _, server := range []string{
		`{}`,
		`{"trusted-network":"false"}`,
		`{"trusted-network":"true","port":"0"}`,
		`{"trusted-network":"true","port":"65536"}`,
		`{"trusted-network":"true","port":"invalid"}`,
		`{"trusted-network":"true","port":true}`,
		`{"tls":{}}`,
		`{"tls":{"ca-certificate":"cache-ca","server-name":"cache.rtr.test"}}`,
		`{"tls":{"certificate":"router","server-name":"cache.rtr.test"}}`,
		`{"tls":{"ca-certificate":"cache-ca","certificate":"router"}}`,
		`{"tls":{"ca-certificate":"cache-ca","certificate":"router","server-name":"127.0.0.1"}}`,
		`{"tls":{"ca-certificate":"cache-ca","certificate":"router","server-name":"*.rtr.test"}}`,
		`{"tls":{"ca-certificate":"cache-ca","certificate":"router","server-name":"cache..test"}}`,
		`{"tls":{"ca-certificate":"cache-ca","certificate":"router","server-name":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.test"}}`,
	} {
		t.Run(server, func(t *testing.T) {
			config := `{"rpki":{"cache-server":{"127.0.0.1":` + server + `}}}`
			if _, err := parseRPKIConfig(config); err == nil {
				t.Fatal("unsafe cache transport configuration was accepted")
			}
		})
	}
}

// TestRTRTLSDNSAddressIsTheReference checks the implicit reference on an actual
// handshake; localhost resolves locally and the listener uses an ephemeral port.
func TestRTRTLSDNSAddressIsTheReference(t *testing.T) {
	// RFC requirement: RFC8210-9.2-8 positive -- an omitted server-name uses the configured cache DNS name as the actual TLS reference.
	fixture := newRTRTLSFixture(t)
	installRTRTLSPKI(t, fixture.store)
	server := fixture.serverConfig(tls.VersionTLS13)
	server.Certificates = []tls.Certificate{fixture.issueServer(t, []string{"localhost"}, nil)}
	peer := startRTRTLSPeer(t, server)
	leaves := rtrTLSConfigLeaves()
	delete(leaves, "server-name")
	session := parsedRTRTLSSession(t, "localhost", peer.port, leaves, make(chan struct{}))
	if err := session.syncOnce(); err != nil {
		t.Fatalf("sync with address as DNS reference: %v", err)
	}
	observed := peer.stop()
	if len(observed) != 1 {
		t.Fatalf("connections = %d, want one", len(observed))
	}
	if observed[0].state.ServerName != "localhost" {
		t.Errorf("cache received SNI %q, want localhost", observed[0].state.ServerName)
	}
	if session.cache.Validate("192.0.2.0/24", 64500) != ValidationValid {
		t.Error("cache authenticated with its address did not publish a VRP")
	}
}

// TestRTRTLSStopCancelsHandshake stops after the server has received ClientHello
// but before it sends ServerHello. No scheduler sleep decides the interleaving.
func TestRTRTLSStopCancelsHandshake(t *testing.T) {
	fixture := newRTRTLSFixture(t)
	installRTRTLSPKI(t, fixture.store)
	peer := startRTRTLSPeer(t, nil)
	peerHello := make(chan struct{})
	release := make(chan struct{})
	peer.handshakeGate <- &rtrTLSHandshakeGate{hello: peerHello, release: release}
	stop := make(chan struct{})
	session := parsedRTRTLSSession(t, "127.0.0.1", peer.port, rtrTLSConfigLeaves(), stop)
	done := make(chan error, 1)
	go func() { done <- session.syncOnce() }()
	t.Cleanup(func() {
		close(release)
		peer.stop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("session worker failed to exit after socket cleanup")
		}
	})
	select {
	case <-peerHello:
	case <-time.After(5 * time.Second):
		close(stop)
		t.Fatal("cache never received ClientHello")
	}
	close(stop)
	select {
	case err := <-done:
		// Preserve the joined result for the unconditional cleanup join.
		done <- err
		if err == nil {
			t.Error("stopped TLS handshake reported a successful sync")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not cancel the pending TLS handshake")
	}
	assertRTRTLSUnpublished(t, session)
}

func rtrTLSConfigLeaves() map[string]string {
	return map[string]string{
		"ca-certificate": "cache-ca",
		"certificate":    "router",
		"server-name":    "cache.rtr.test",
	}
}

func parsedRTRTLSSession(t *testing.T, address string, port uint16, leaves map[string]string, stop <-chan struct{}) *RTRSession {
	t.Helper()
	config, err := json.Marshal(map[string]any{"rpki": map[string]any{
		"cache-server": map[string]any{address: map[string]any{
			"port": strconv.Itoa(int(port)), "tls": leaves,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseRPKIConfig(string(config))
	if err != nil {
		t.Fatalf("parse TLS cache configuration: %v", err)
	}
	if len(parsed.CacheServers) != 1 {
		t.Fatalf("configured caches = %d, want one", len(parsed.CacheServers))
	}
	cache := parsed.CacheServers[0]
	session := newRTRSession(cache.Address, cache.Port, cache.Preference, cache.SourceAddress, newROACache(), newASPACache(), stop)
	session.tlsSettings = cache.TLS
	session.pkiConfig = pki.Snapshot()
	t.Cleanup(func() {
		session.close()
		if session.dataLease != nil {
			session.dataLease.stop()
		}
	})
	return session
}

func assertRTRTLSRefused(t *testing.T, session *RTRSession, peer *rtrTLSPeer) {
	t.Helper()
	if err := session.syncOnce(); err == nil {
		t.Error("unauthenticated cache sync succeeded")
	}
	observeds := peer.stop()
	for index := range observeds {
		observed := &observeds[index]
		if observed.queryBytes != 0 {
			t.Errorf("unauthenticated cache received %d RTR query bytes", observed.queryBytes)
		}
		if timeout, ok := errors.AsType[net.Error](observed.err); ok {
			if timeout.Timeout() {
				t.Errorf("cache timed out instead of observing transport refusal: %v", observed.err)
			}
		}
	}
	assertRTRTLSUnpublished(t, session)
}

func assertRTRTLSUnpublished(t *testing.T, session *RTRSession) {
	t.Helper()
	if session.cache.Validate("192.0.2.0/24", 64500) != ValidationNotFound {
		t.Error("unauthenticated cache changed origin validation")
	}
	if session.aspaCache.hasRecord(64500) {
		t.Error("unauthenticated cache published an ASPA")
	}
	if session.synced {
		t.Error("unauthenticated cache marked the session synchronized")
	}
}

// installRTRTLSPKI is deliberately nonparallel: PKI is process-wide. Cleanup
// reinstalls the exact previous snapshot after the peer and session are joined.
func installRTRTLSPKI(t *testing.T, config *pki.PKIConfig) {
	t.Helper()
	previous := pki.Snapshot()
	if err := pki.Load(config); err != nil {
		t.Fatalf("install test PKI: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(previous); err != nil {
			t.Errorf("restore PKI: %v", err)
		}
	})
}

type rtrTLSFixture struct {
	store           *pki.PKIConfig
	issuer          *x509.Certificate
	issuerKey       *ecdsa.PrivateKey
	intermediates   []*x509.Certificate
	server          tls.Certificate
	serverUntrusted tls.Certificate
}

func newRTRTLSFixture(t *testing.T) *rtrTLSFixture {
	t.Helper()
	root, rootKey := issueRTRTLSCertificate(t, 1, nil, nil, true, nil, nil, x509.ExtKeyUsageAny)
	upper, upperKey := issueRTRTLSCertificate(t, 2, root, rootKey, true, nil, nil, x509.ExtKeyUsageAny)
	lower, lowerKey := issueRTRTLSCertificate(t, 3, upper, upperKey, true, nil, nil, x509.ExtKeyUsageAny)
	unrelated, unrelatedKey := issueRTRTLSCertificate(t, 4, nil, nil, true, nil, nil, x509.ExtKeyUsageAny)
	fixture := &rtrTLSFixture{
		store: &pki.PKIConfig{
			CACerts: map[string]*pki.CACertEntry{
				"cache-ca":     {Name: "cache-ca", Certificate: root, Raw: root.Raw},
				"unrelated-ca": {Name: "unrelated-ca", Certificate: unrelated, Raw: unrelated.Raw},
			},
			Certificates: make(map[string]*pki.CertificateEntry),
		},
		issuer: lower, issuerKey: lowerKey,
		intermediates: []*x509.Certificate{lower, upper},
	}
	fixture.store.Certificates["router"] = fixture.issueClient(t, x509.ExtKeyUsageClientAuth, true)
	fixture.server = fixture.issueServer(t, []string{"cache.rtr.test"}, nil)
	untrusted, untrustedKey := issueRTRTLSCertificate(t, 7, unrelated, unrelatedKey, false, []string{"cache.rtr.test"}, nil, x509.ExtKeyUsageServerAuth)
	fixture.serverUntrusted = tls.Certificate{
		Certificate: [][]byte{untrusted.Raw}, PrivateKey: untrustedKey, Leaf: untrusted,
	}
	return fixture
}

func issueRTRTLSCertificate(t *testing.T, serial int64, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey, ca bool, dns []string, ips []net.IP, usage x509.ExtKeyUsage) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "cache.rtr.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
		BasicConstraintsValid: true, IsCA: ca, DNSNames: dns, IPAddresses: ips,
	}
	if len(dns) != 0 {
		template.Subject.CommonName = "not-the-cache-reference.invalid"
	}
	if ca {
		template.Subject.CommonName = fmt.Sprintf("RTR test CA %d", serial)
		template.KeyUsage = x509.KeyUsageCertSign
		template.ExtKeyUsage = nil
	}
	if issuer == nil {
		issuer, issuerKey = template, key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, key
}

func (f *rtrTLSFixture) issueClient(t *testing.T, usage x509.ExtKeyUsage, withIP bool) *pki.CertificateEntry {
	t.Helper()
	var ips []net.IP
	if withIP {
		ips = []net.IP{net.ParseIP("127.0.0.1")}
	}
	cert, key := issueRTRTLSCertificate(t, 5, f.issuer, f.issuerKey, false, nil, ips, usage)
	return &pki.CertificateEntry{
		Name: "router", Certificate: cert, Raw: cert.Raw, PrivateKey: key,
		Intermediates:    f.intermediates,
		RawIntermediates: [][]byte{f.intermediates[0].Raw, f.intermediates[1].Raw},
	}
}

func (f *rtrTLSFixture) issueServer(t *testing.T, dns []string, ips []net.IP) tls.Certificate {
	t.Helper()
	cert, key := issueRTRTLSCertificate(t, 6, f.issuer, f.issuerKey, false, dns, ips, x509.ExtKeyUsageServerAuth)
	return tls.Certificate{
		Certificate: [][]byte{cert.Raw, f.intermediates[0].Raw, f.intermediates[1].Raw},
		PrivateKey:  key, Leaf: cert,
	}
}

func (f *rtrTLSFixture) serverConfig(version uint16) *tls.Config {
	roots := x509.NewCertPool()
	roots.AddCert(f.store.CACerts["cache-ca"].Certificate)
	return &tls.Config{
		MinVersion: version, MaxVersion: version,
		Certificates: []tls.Certificate{f.server},
		ClientAuth:   tls.RequireAndVerifyClientCert, ClientCAs: roots,
	}
}

type rtrTLSObservation struct {
	state      tls.ConnectionState
	queryBytes int
	queryType  uint8
	err        error
}

type rtrTLSHandshakeGate struct {
	hello   chan struct{}
	release <-chan struct{}
}

// rtrTLSPeer owns one bounded accept worker. The caller MUST call stop to close
// its listener and active connection and join that worker before reading results.
// Its connection lifecycle is safe for concurrent stop calls.
type rtrTLSPeer struct {
	port          uint16
	listener      net.Listener
	mu            sync.Mutex
	conn          net.Conn
	closed        bool
	done          chan struct{}
	once          sync.Once
	observed      []rtrTLSObservation
	handshakeGate chan *rtrTLSHandshakeGate
}

func startRTRTLSPeer(t *testing.T, config *tls.Config) *rtrTLSPeer {
	t.Helper()
	var listen net.ListenConfig
	listener, err := listen.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is %T, want *net.TCPAddr", listener.Addr())
	}
	peer := &rtrTLSPeer{
		listener: listener, done: make(chan struct{}),
		handshakeGate: make(chan *rtrTLSHandshakeGate, 1),
		port:          uint16(address.Port), //nolint:gosec // TCP port fits uint16.
	}
	go peer.serve(config)
	t.Cleanup(func() { peer.stop() })
	return peer
}

// stop MUST be called after starting a peer; it closes its sockets and joins the
// worker. Repeated calls return the same observations after that join.
func (p *rtrTLSPeer) stop() []rtrTLSObservation {
	p.once.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.listener.Close() //nolint:errcheck // Closing the owned listener wakes Accept; no I/O is pending on it afterward.
		if p.conn != nil {
			p.conn.Close() //nolint:errcheck // Closing the owned connection wakes the worker; its I/O result is recorded.
		}
		p.mu.Unlock()
		<-p.done
	})
	return p.observed
}

func (p *rtrTLSPeer) serve(config *tls.Config) {
	defer close(p.done)
	// Four connections cover every RTR version downgrade as well as an illicit
	// plaintext retry; the listener is closed by stop even if none arrive.
	for range 4 {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			conn.Close() //nolint:errcheck // Cleanup won the race with Accept.
			return
		}
		p.conn = conn
		p.mu.Unlock()
		p.observed = append(p.observed, p.exchange(conn, config))
		conn.Close() //nolint:errcheck // The exchange result has already recorded any I/O failure.
		p.mu.Lock()
		p.conn = nil
		p.mu.Unlock()
	}
}

func (p *rtrTLSPeer) exchange(conn net.Conn, config *tls.Config) rtrTLSObservation {
	var result rtrTLSObservation
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		result.err = err
		return result
	}
	if config != nil {
		secure := tls.Server(conn, config)
		if err := secure.HandshakeContext(context.Background()); err != nil {
			result.err = err
			return result
		}
		result.state = secure.ConnectionState()
		conn = secure
	}
	var query [pduResetQueryLen]byte
	result.queryBytes, result.err = io.ReadFull(conn, query[:])
	if config == nil {
		if query[0] == 22 {
			// Consume the complete first TLS record so a plaintext query sent
			// after the failed handshake cannot hide behind ClientHello bytes.
			result.queryBytes = 0
			remaining := int64(binary.BigEndian.Uint16(query[3:5])) - 3
			if remaining < 0 {
				result.err = fmt.Errorf("short ClientHello record: %x", query)
				return result
			}
			if _, err := io.CopyN(io.Discard, conn, remaining); err != nil {
				result.err = err
				return result
			}
			select {
			case gate := <-p.handshakeGate:
				close(gate.hello)
				<-gate.release
				return result
			default:
				if _, err := io.WriteString(conn, "not TLS\n"); err != nil {
					result.err = err
					return result
				}
			}
			// The TLS client must close, not restart RTR on the same socket.
			result.queryBytes, result.err = io.ReadFull(conn, query[:])
		}
	}
	if result.err != nil {
		return result
	}
	result.queryType = query[1]
	if query[0] != 2 || (query[1] != pduResetQuery && query[1] != pduSerialQuery) {
		result.err = fmt.Errorf("unexpected opening RTR PDU %x", query)
		return result
	}
	if query[1] == pduSerialQuery {
		var serial [4]byte
		var n int
		n, result.err = io.ReadFull(conn, serial[:])
		result.queryBytes += n
		if result.err != nil {
			return result
		}
		if binary.BigEndian.Uint16(query[2:4]) != 0 || binary.BigEndian.Uint32(serial[:]) != 1 {
			result.err = fmt.Errorf("serial query has no known cache base: %x %x", query, serial)
			return result
		}
	}
	response, end := cacheResponsePDU(), endOfDataPDU()
	response[0], end[0] = 2, 2
	binary.BigEndian.PutUint32(end[8:12], 1)
	prefix := []byte{2, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0, 192, 0, 2, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(prefix[16:20], 64500)
	pdus := [][]byte{response}
	if query[1] == pduResetQuery {
		pdus = append(pdus, prefix, aspaPDU(64500, 64501))
	}
	pdus = append(pdus, end)
	for _, pdu := range pdus {
		if _, err := conn.Write(pdu); err != nil {
			result.err = err
			return result
		}
	}
	return result
}
