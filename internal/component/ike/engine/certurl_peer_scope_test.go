package engine

import (
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.6 mandates SHA-1 as the hash-and-url object identifier
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// cpsAllow is the loopback allowance the httptest servers below need. The fetcher's deny
// list refuses 127.0.0.0/8 outright, so without it no test here reaches a server at all.
var cpsAllow = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

// cpsClearFailures empties the fetcher's negative cache for one test. It is process-global
// and outlives any SA, so an entry an earlier test wrote would answer this one.
func cpsClearFailures(t *testing.T) {
	t.Helper()
	clear := func() {
		certURLFetches.mu.Lock()
		defer certURLFetches.mu.Unlock()
		for k := range certURLFetches.failed {
			delete(certURLFetches.failed, k)
		}
	}
	clear()
	t.Cleanup(clear)
}

// VALIDATES: one peer's FAILED hash-and-url lookup does not suppress another peer's lookup
// of the same object.
//
// PREVENTS: cross-peer poisoning of the negative cache. certURLFetcher.start keyed its
// failure record on the SHA-1 alone, while the destination allow-list that can refuse a
// fetch is per-peer. Any peer that completed IKE_SA_INIT could therefore name a VICTIM's
// certificate hash beside a URL its own allow-list rejects, record the failure, and
// suppress the victim's legitimate lookup for certURLFailBackoff -- renewably, so
// indefinitely. The victim's IKE_AUTH is dropped for as long as it lasts.
//
// The measurement is the victim's object reaching the cache. An assertion on the map's keys
// would pass against any keying that merely LOOKS scoped.
func TestCpsOnePeersFailureDoesNotSuppressAnothersLookup(t *testing.T) {
	wpcFreshCache(t)
	cpsClearFailures(t)

	der := wpcChain(t, 0)[0]
	sum := sha1.Sum(der) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
	hash := sum[:]

	// The attacker names the victim's object at a server that refuses it.
	bad, _ := cuaCountingServer(t)
	certURLFetches.start("attacker", wpcHashAndURL(hash, bad.URL), cpsAllow, slogutil.DiscardLogger())
	cuaWaitIdle(t)
	if _, cached := lookupHashAndURL(hash); cached {
		t.Fatal("the refusing server cached the object, so there is no failure to poison with")
	}

	// The victim names the same object at the server that really serves it.
	good, hits := wpcServer(t, der)
	certURLFetches.start("victim", wpcHashAndURL(hash, good.URL), cpsAllow, slogutil.DiscardLogger())
	cuaWaitIdle(t)

	if _, cached := lookupHashAndURL(hash); !cached {
		t.Errorf("after another peer recorded a failure for the same hash, the victim's lookup "+
			"fetched nothing (%d requests reached its server); its IKE_AUTH is dropped for "+
			"certURLFailBackoff (%v), renewably", *hits, certURLFailBackoff)
	}
}

// VALIDATES: the SAME peer repeating a failed lookup is still held off.
//
// This is the discriminator, and it is what stops the fix from being "delete the negative
// cache". The backoff exists because a miss drops the IKE_AUTH and the peer retransmits it
// (RFC 7296 Section 2.1), so without a record every retransmission is another outbound GET
// to a destination that peer chose. Scoping the record to the peer must not remove it.
//
// TestCuaFailedLookupIsNotRetriedOnEveryRetransmission covers the same property through the
// full retransmission sequence; this one states it beside the cross-peer half so a future
// edit of the key sees both at once.
func TestCpsTheSamePeersRepeatIsStillHeldOff(t *testing.T) {
	wpcFreshCache(t)
	cpsClearFailures(t)

	sum := sha1.Sum([]byte("cps-same-peer-object")) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
	bad, hits := cuaCountingServer(t)
	payload := wpcHashAndURL(sum[:], bad.URL)

	certURLFetches.start("peer-one", payload, cpsAllow, slogutil.DiscardLogger())
	cuaWaitIdle(t)
	if got := hits(); got != 1 {
		t.Fatalf("the first lookup made %d requests, want exactly 1", got)
	}

	certURLFetches.start("peer-one", payload, cpsAllow, slogutil.DiscardLogger())
	cuaWaitIdle(t)
	if got := hits(); got != 1 {
		t.Errorf("the same peer's repeat made the request count %d, want it held at 1; the peer "+
			"chooses both the destination and the repeat rate", got)
	}
}
