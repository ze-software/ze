// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// RFC: rfc/short/rfc7296.md -- Hash and URL certificate encodings (Section 3.6)

package engine

import (
	"crypto/sha1" //nolint:gosec // the cache key is the RFC 7296 Section 3.6 object hash
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// VALIDATES: the certificate cache is bounded in entries and in bytes, and evicting
// keeps the newest.
// PREVENTS: repeated handshakes growing the heap without limit. The key is a hash the
// PEER chose and an entry is stored before AUTH is verified, so an unauthenticated peer
// decides how many entries exist and how much they weigh.
func TestCertURLCacheIsBounded(t *testing.T) {
	t.Run("entry bound", func(t *testing.T) {
		resetCertURLCache()
		t.Cleanup(resetCertURLCache)

		total := certURLCacheMaxEntries * 3
		for i := range total {
			certURLCache.Store("key-"+strconv.Itoa(i), []byte{byte(i)})
		}
		certURLCache.mu.Lock()
		entries := len(certURLCache.items)
		order := len(certURLCache.order)
		certURLCache.mu.Unlock()

		if entries > certURLCacheMaxEntries {
			t.Fatalf("the cache holds %d entries and the bound is %d; an unauthenticated "+
				"peer chooses the key, so an unbounded cache is a heap-growth primitive",
				entries, certURLCacheMaxEntries)
		}
		if order != entries {
			t.Errorf("the eviction order holds %d keys and the map holds %d; the two must "+
				"not drift or the byte accounting leaks", order, entries)
		}
		// Eviction is oldest-first, so the last writes survive and the first do not.
		if _, ok := certURLCache.Load("key-0"); ok {
			t.Error("the oldest entry survived, so eviction is not insertion-ordered")
		}
		if _, ok := certURLCache.Load("key-" + strconv.Itoa(total-1)); !ok {
			t.Error("the newest entry was evicted, so a fresh fetch is discarded at once")
		}
	})

	t.Run("byte bound", func(t *testing.T) {
		resetCertURLCache()
		t.Cleanup(resetCertURLCache)

		// Objects at the body cap, enough of them to pass the byte budget well before
		// the entry count would.
		big := make([]byte, certURLMaxBytes)
		for i := range certURLCacheMaxEntries * 2 {
			certURLCache.Store("big-"+strconv.Itoa(i), big)
		}
		certURLCache.mu.Lock()
		total := certURLCache.bytes
		certURLCache.mu.Unlock()
		if total > certURLCacheMaxBytes {
			t.Fatalf("the cache holds %d bytes and the bound is %d", total, certURLCacheMaxBytes)
		}
	})

	t.Run("accounting survives a repeat of the same key", func(t *testing.T) {
		resetCertURLCache()
		t.Cleanup(resetCertURLCache)

		for range 10 {
			certURLCache.Store("same", []byte("abcd"))
		}
		certURLCache.mu.Lock()
		entries, cached := len(certURLCache.items), certURLCache.bytes
		certURLCache.mu.Unlock()
		if entries != 1 || cached != 4 {
			t.Fatalf("a repeated key produced %d entries and %d bytes, want 1 and 4; "+
				"double counting would evict live entries early", entries, cached)
		}
	})
}

// VALIDATES: the fetcher refuses a response whose HEADER exceeds its own budget.
// PREVENTS: the body cap being taken for the whole bound. Go's default header budget is
// 10 MiB, 160 times certURLMaxBytes, and certURLMaxInFlight multiplies whatever one
// response costs.
func TestCertURLRefusesOversizedResponseHeader(t *testing.T) {
	resetCertURLCache()
	t.Cleanup(resetCertURLCache)

	body := []byte("certificate-der")
	sum := sha1.Sum(body) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1 as the object id

	filler := make([]byte, 1024)
	for i := range filler {
		filler[i] = 'x'
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Well past certURLMaxHeaderBytes and far below Go's 10 MiB default, so this
		// server is refused only because the budget was narrowed.
		for i := range 64 {
			w.Header().Set("X-Filler-"+strconv.Itoa(i), string(filler))
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(body); err != nil {
			return
		}
	}))
	t.Cleanup(srv.Close)

	got, err := fetchHashAndURL(t.Context(),
		curlPayloadWithHash(sum[:], srv.URL), curlLoopbackAllow())
	if err == nil {
		t.Fatal("a response with a header past the budget was accepted")
	}
	if got != nil {
		t.Error("bytes were returned despite the refusal")
	}
	if _, cached := certURLCache.Load(string(sum[:])); cached {
		t.Error("a refused response was cached")
	}
}
