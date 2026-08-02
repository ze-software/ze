// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// RFC: rfc/short/rfc7296.md -- Hash and URL certificate encodings (Section 3.6)

package engine

import (
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.6 mandates SHA-1 as the hash-and-url object identifier
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// cuaStalledServer returns a server that accepts a request and never answers it. The
// cleanup releases every held request so the test's workers can exit.
func cuaStalledServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	return srv
}

// cuaPeer builds an SA configured for hash-and-url with the loopback allowance httptest
// needs, and the CERT payload naming one object on srv.
func cuaPeer(t *testing.T, srvURL string, hash []byte) (*SA, *wire.PayloadCERT) {
	t.Helper()
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth = wpcAuth()
	sa.PeerCfg.Auth.HashAndURL = true
	sa.PeerCfg.Auth.CertificateURL = "http://pki.example/device.der"
	sa.PeerCfg.Auth.CertificateURLAllow = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	return sa, &wire.PayloadCERT{
		CertEncoding: wire.CertEncodingHashURL,
		CertData:     wpcHashAndURL(hash, srvURL),
	}
}

// cuaCountingServer returns a server that refuses every request, and a function reporting
// how many requests it has answered.
func cuaCountingServer(t *testing.T) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv, func() int {
		mu.Lock()
		defer mu.Unlock()
		return hits
	}
}

// cuaWaitIdle waits for every background lookup to finish.
func cuaWaitIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if certURLFetches.inFlight() == 0 {
			return
		}
		// Poll interval; the loop returns as soon as the pending set empties.
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the background hash-and-url lookups never finished")
}

// VALIDATES: a hash-and-url lookup that FAILED is not retried for every retransmission of
// the message that named it. The peer's repeats cost one outbound GET, not one each.
//
// PREVENTS: the defect this test exists for. On failure nothing is cached and the worker
// removed its own pending key, so each retransmitted IKE_AUTH re-entered start and launched
// a fresh GET to a host the UNAUTHENTICATED peer chose. The peer controlled the destination
// and, through the retransmit schedule, the rate. Before the fetch moved off the dispatch
// goroutine the SA died after one attempt, so this repeat did not exist; the bound restores
// that ceiling without putting the goroutine stall back.
func TestCuaFailedLookupIsNotRetriedOnEveryRetransmission(t *testing.T) {
	srv, hits := cuaCountingServer(t)

	certURLFetches.mu.Lock()
	clear(certURLFetches.failed)
	certURLFetches.mu.Unlock()

	hash := sha1.Sum([]byte("retransmitted-object")) //nolint:gosec // RFC 7296 Section 3.6 object id
	sa, cert := cuaPeer(t, srv.URL, hash[:])

	// The first delivery starts a worker, which fails against the refusing server.
	certURLFetches.start(sa.PeerName, cert.CertData, sa.PeerCfg.Auth.CertificateURLAllow, slogutil.DiscardLogger())
	cuaWaitIdle(t)
	if got := hits(); got != 1 {
		t.Fatalf("the first delivery made %d requests, want exactly 1", got)
	}

	// Five retransmissions of the same message. Each re-enters start. Same peer, so the
	// backoff record applies.
	for range 5 {
		certURLFetches.start(sa.PeerName, cert.CertData, sa.PeerCfg.Auth.CertificateURLAllow, slogutil.DiscardLogger())
	}
	cuaWaitIdle(t)

	if got := hits(); got != 1 {
		t.Errorf("five retransmissions after a failed lookup made %d requests in total, want 1; "+
			"the peer chooses both the destination and the repeat rate", got)
	}
}

// VALIDATES: the backoff record cannot itself be grown without bound by a peer naming
// endless distinct hashes.
//
// The key is a hash the PEER chose, so the failure map is attacker-sized exactly like the
// object cache. It is bounded by the same constant, and past it the set is dropped rather
// than evicted one entry at a time: the map is an optimisation, and losing it costs one
// more GET per hash, which is the behavior that existed before it.
func TestCuaFailureRecordIsBounded(t *testing.T) {
	certURLFetches.mu.Lock()
	clear(certURLFetches.failed)
	for i := range certURLCacheMaxEntries * 3 {
		certURLFetches.noteFailureLocked(certURLFailure{peer: "ze", hash: string(rune(i)) + "-distinct-hash"})
	}
	got := len(certURLFetches.failed)
	certURLFetches.mu.Unlock()

	if got > certURLCacheMaxEntries {
		t.Errorf("the failure record holds %d entries after 3x the bound, want at most %d",
			got, certURLCacheMaxEntries)
	}
	if got == 0 {
		t.Error("the failure record is empty, so the backoff above records nothing at all")
	}
}

// VALIDATES: a hash-and-url lookup an unauthenticated peer names performs NO network I/O on
// the goroutine that processes its IKE_AUTH. The delivery reports the object as pending and
// returns at once, even when the server the peer chose never answers.
// PREVENTS: the defect this machinery exists to remove. routeInbound (register.go) drives a
// half-open handshake INLINE on the one dispatch goroutine that serves every IKE session,
// because the owner-loop hand-off needs ps.ownedSA and runEstablished is its only writer.
// The lookup used to run there, before verifyRemoteAuth. One unauthenticated peer named up
// to certificate-count URLs at certURLTimeout apiece and stopped every other peer's IKE for
// the product. Deferring past authentication is not available -- verifyRemoteAuth needs the
// certificate -- so the fetch moves to a worker instead.
func TestCuaStalledLookupDoesNotHoldTheCallerGoroutine(t *testing.T) {
	wpcFreshCache(t)
	chain := wpcChain(t, 0)
	sum := sha1.Sum(chain[0]) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1

	srv := cuaStalledServer(t)
	sa, payload := cuaPeer(t, srv.URL, sum[:])

	start := time.Now()
	err := storeRemoteCerts(sa, []*wire.PayloadCERT{payload}, wpcQuietLogger())
	elapsed := time.Since(start)

	if !errors.Is(err, errCertURLPending) {
		t.Fatalf("a stalled lookup returned %v, want errCertURLPending; anything else means "+
			"the caller waited on the network", err)
	}
	// The inline fetcher took exactly certURLTimeout against this server. The bound below
	// is half of that. The call performs no I/O at all now, so the margin is very large
	// and the assertion is not a race with host load.
	if elapsed >= certURLTimeout/2 {
		t.Errorf("the delivery took %v against a server that never answers; the lookup is "+
			"back on the caller's goroutine and every other IKE session waits for it", elapsed)
	}
	if len(sa.RemoteCertRaw) != 0 {
		t.Error("a pending lookup stored a peer certificate")
	}
}

// VALIDATES: a retransmission storm cannot multiply background workers. One object gets one
// worker however many times it is named, and the pending set never exceeds its bound.
// PREVENTS: the goroutine-growth primitive the asynchronous hand-off would otherwise be. An
// unauthenticated peer chooses the hashes, and every dropped IKE_AUTH is sent again. An
// unbounded pending set trades one blocked goroutine for an unbounded number of them
// (ai/rules/fail-closed-guards.md).
func TestCuaPendingLookupsAreDedupedAndBounded(t *testing.T) {
	wpcFreshCache(t)
	chain := wpcChain(t, 0)
	sum := sha1.Sum(chain[0]) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1

	srv := cuaStalledServer(t)
	sa, payload := cuaPeer(t, srv.URL, sum[:])

	// The same object delivered many times. Every delivery reports pending. Only the first
	// starts a worker.
	for range 20 {
		if err := storeRemoteCerts(sa, []*wire.PayloadCERT{payload}, wpcQuietLogger()); !errors.Is(err, errCertURLPending) {
			t.Fatalf("a repeat delivery returned %v, want errCertURLPending", err)
		}
	}
	if got := certURLFetches.inFlight(); got != 1 {
		t.Errorf("20 deliveries of ONE object left %d workers running, want 1; a retransmission "+
			"storm multiplies goroutines", got)
	}

	// Distinct objects, far more than the bound. Each is a different hash, so the dedupe
	// above cannot be what holds the number down.
	for i := range certURLMaxPending * 4 {
		distinct := sum
		distinct[0] = byte(i + 1)
		_, p := cuaPeer(t, srv.URL, distinct[:])
		if err := storeRemoteCerts(sa, []*wire.PayloadCERT{p}, wpcQuietLogger()); !errors.Is(err, errCertURLPending) {
			t.Fatalf("delivery %d returned %v, want errCertURLPending", i, err)
		}
	}
	if got := certURLFetches.inFlight(); got > certURLMaxPending {
		t.Errorf("%d workers are running and the bound is %d", got, certURLMaxPending)
	}
}

// VALIDATES: one delivery starts a worker for EVERY uncached object the message names, and
// the whole chain resolves on the peer's first retransmission.
// PREVENTS: one worker per delivery. A four-certificate chain would then need four
// retransmissions, and a responder's half-open timeout (30s) can expire before an initiator
// backing off exponentially gets through them.
func TestCuaEveryUncachedObjectGetsAWorkerFromOneDelivery(t *testing.T) {
	wpcFreshCache(t)
	chain := wpcChain(t, 2) // leaf plus two intermediates

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth = wpcAuth()
	sa.PeerCfg.Auth.HashAndURL = true
	sa.PeerCfg.Auth.CertificateURL = "http://pki.example/device.der"
	sa.PeerCfg.Auth.CertificateURLAllow = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

	payloads := make([]*wire.PayloadCERT, 0, len(chain))
	hashes := make([][]byte, 0, len(chain))
	for _, der := range chain {
		srv, _ := wpcServer(t, der)
		sum := sha1.Sum(der) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
		h := make([]byte, len(sum))
		copy(h, sum[:])
		hashes = append(hashes, h)
		payloads = append(payloads, &wire.PayloadCERT{
			CertEncoding: wire.CertEncodingHashURL,
			CertData:     wpcHashAndURL(h, srv.URL),
		})
	}
	if len(payloads) < 3 {
		t.Fatalf("the fixture built %d payloads; the multi-object claim needs at least 3", len(payloads))
	}

	if err := storeRemoteCerts(sa, payloads, wpcQuietLogger()); !errors.Is(err, errCertURLPending) {
		t.Fatalf("the first delivery returned %v, want errCertURLPending", err)
	}

	// ONE delivery, and every object is cached after it. A loop that returned at the first
	// miss would leave the second and third untouched.
	for i, h := range hashes {
		wpcAwaitCached(t, h)
		if _, ok := lookupHashAndURL(h); !ok {
			t.Fatalf("object %d was not fetched by the first delivery, so the chain needs one "+
				"retransmission per certificate", i)
		}
	}

	// The retransmission resolves the whole chain at once.
	if err := storeRemoteCerts(sa, payloads, wpcQuietLogger()); err != nil {
		t.Fatalf("the retransmission did not resolve the chain: %v", err)
	}
	if len(sa.RemoteCertChainRaw) != len(chain)-1 {
		t.Errorf("the resolved chain holds %d intermediates, want %d",
			len(sa.RemoteCertChainRaw), len(chain)-1)
	}
}

// VALIDATES: a Hash and URL payload too short to carry its 20-octet hash is REFUSED, and is
// not treated as an object awaiting a lookup.
// PREVENTS: a malformed payload becoming an unkillable retransmission loop. It can never
// become cached, so reporting it as pending would have the peer resend until the half-open
// reaper fires rather than refusing the message outright.
func TestCuaMalformedPayloadIsRefusedNotPended(t *testing.T) {
	wpcFreshCache(t)
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth = wpcAuth()
	sa.PeerCfg.Auth.HashAndURL = true

	short := &wire.PayloadCERT{
		CertEncoding: wire.CertEncodingHashURL,
		CertData:     make([]byte, wire.CertHashURLHashLen),
	}
	err := storeRemoteCerts(sa, []*wire.PayloadCERT{short}, wpcQuietLogger())
	if errors.Is(err, errCertURLPending) {
		t.Fatal("a payload that cannot carry a hash was reported as pending, so the peer " +
			"retransmits it until the handshake is reaped")
	}
	if !errors.Is(err, errCertURLShortData) {
		t.Fatalf("a short payload returned %v, want errCertURLShortData", err)
	}
	if certURLFetches.inFlight() != 0 {
		t.Error("a malformed payload started a background lookup")
	}
}
