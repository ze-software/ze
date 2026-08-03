// VALIDATES: the RFC 7296 Section 3.6 hash-and-URL lookup: the http scheme is supported,
// the fetched bytes are verified against the hash the peer sent BEFORE anything parses
// them, and every control bounding a pre-authentication outbound request holds.
// PREVENTS: the fetch becoming a server-side request forgery primitive. A peer that is not
// yet authenticated chooses this URL. A missing scheme check, size cap, timeout, redirect
// refusal or destination deny turns IKE_AUTH into an outbound request generator. The peer
// names the target.
package engine

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // the payload format under test is defined in terms of SHA-1
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// curlServe returns a handler writing body. The write error is handled rather than
// discarded: a client that hung up mid-body is exactly what the size-cap row produces.
func curlServe(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(body); err != nil {
			return
		}
	}
}

// curlPayload builds a Hash and URL payload body: the 20-octet SHA-1 of body, then url.
func curlPayload(body []byte, url string) []byte {
	sum := sha1.Sum(body)
	return append(sum[:], []byte(url)...)
}

// curlPayloadWithHash builds one carrying a hash that is deliberately not the served
// body's, which is the fixture the hash-mismatch row needs.
func curlPayloadWithHash(hash []byte, url string) []byte {
	out := make([]byte, 0, len(hash)+len(url))
	out = append(out, hash...)
	return append(out, []byte(url)...)
}

// curlLoopbackAllow permits the httptest server's loopback address. Loopback is denied by
// default and a test server can only listen there, so every positive test states the
// permission explicitly. The destination rows below drop it, which is what proves the
// default deny is real rather than incidental.
func curlLoopbackAllow() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
	}
}

func curlReset() {
	resetCertURLCache()
}

// VALIDATES: the http scheme is supported for hash-and-URL lookup, and the fetched bytes
// are verified against the hash the peer sent -- RFC 7296 Section 3.6
// (rfc/full/rfc7296.txt:5279-5282). Section 1.7 at :1226-1229 records it as an addition
// over RFC 5996. This is the LOOKUP half of the obligation, and only a real fetch proves
// it.
// PREVENTS: a send-only implementation being mistaken for a conforming one.
//
// NOT YET TAGGED as `RFC requirement: RFC7296-3.6-3`, deliberately. Section 3.6 has three
// Appendix A rows -- 3.6-1 (four X.509 certificates), 3.6-2 (both Hash and URL formats)
// and 3.6-3. Section 3.6 has NO committed id, so the first package to land there sets
// the high-water mark. check_id_allocation (scripts/dev/rfc_requirements.py,
// check_id_allocation) then refuses every ordinal at or below it, so enrolling 3.6-3
// alone would strand 3.6-1 and 3.6-2 permanently. The three must land together. The
// send half and the accept-path wiring 3.6-1 and 3.6-2 need are not implemented yet, so
// this test stands as ordinary coverage until they are.
func TestHashURLLookupUsesHTTPAndVerifiesTheHash(t *testing.T) {
	curlReset()
	der := []byte("pretend-DER-certificate-bytes")
	srv := httptest.NewServer(curlServe(der))
	defer srv.Close()

	got, err := fetchHashAndURL(context.Background(),
		curlPayload(der, srv.URL), curlLoopbackAllow())
	if err != nil {
		t.Fatalf("an http hash-and-url lookup failed: %v", err)
	}
	if !bytes.Equal(got, der) {
		t.Errorf("lookup returned %q, want the served bytes %q", got, der)
	}

	// The cache is keyed by the hash, so a second lookup naming a DEAD url still
	// resolves. That is also what proves the key is the hash and not the URL.
	srv.Close()
	again, err := fetchHashAndURL(context.Background(),
		curlPayload(der, "http://192.0.2.1/gone"), curlLoopbackAllow())
	if err != nil {
		t.Fatalf("the cached object was not returned for the same hash: %v", err)
	}
	if !bytes.Equal(again, der) {
		t.Error("the cache returned different bytes for the same hash")
	}
}

// rfc-test-change-approved: 2026-07-31 NOT a weakening and NOT a strengthening -- the
// `RFC requirement: RFC7296-3.6-3` tag on this function and on the positive above was
// added earlier in THIS uncommitted session. This session withdraws it before any commit.
//
// No committed summary row, no ledger entry and no public claim in
// docs/features/rfc-status.md ever referenced RFC7296-3.6-3. No proof of an advertised
// obligation is therefore removed.
//
// The reason is id allocation, stated in full on the positive. §3.6's three rows must
// enroll as a contiguous block, and two of them are not implemented yet. Every assertion
// below is unchanged. Only the tag lines change.
//
// VALIDATES: every control bounding the lookup holds, one table row per control. Each is
// its own row on purpose: a single "a bad URL is refused" row passes while five of the
// seven controls are missing.
// PREVENTS: the pre-authentication fetch becoming an SSRF primitive.
//
// The hash row is the most important assertion in this package. A fetcher that parses the
// fetched bytes first and compares the hash afterwards passes every other row here. That
// fetcher is still exploitable, because parsing attacker-supplied DER is the actual
// attack surface.
func TestHashURLLookupRefusesEverythingOutsideTheBound(t *testing.T) {
	body := []byte("some-der")

	t.Run("scheme file", func(t *testing.T) {
		curlReset()
		_, err := fetchHashAndURL(context.Background(),
			curlPayload(body, "file:///etc/passwd"), curlLoopbackAllow())
		if !errors.Is(err, errCertURLScheme) {
			t.Fatalf("a file: URL was not refused by the scheme check: %v", err)
		}
	})

	t.Run("scheme ftp", func(t *testing.T) {
		curlReset()
		_, err := fetchHashAndURL(context.Background(),
			curlPayload(body, "ftp://example.com/cert"), curlLoopbackAllow())
		if !errors.Is(err, errCertURLScheme) {
			t.Fatalf("an ftp: URL was not refused by the scheme check: %v", err)
		}
	})

	t.Run("scheme https is not http", func(t *testing.T) {
		curlReset()
		_, err := fetchHashAndURL(context.Background(),
			curlPayload(body, "https://example.com/cert"), curlLoopbackAllow())
		if !errors.Is(err, errCertURLScheme) {
			t.Fatalf("the scheme allowlist admitted a scheme other than http: %v", err)
		}
	})

	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, strengthening only. Mutation testing found
	// the first version of this row did not gate the control it names. Deleting
	// io.LimitReader left the row GREEN, because the post-hoc length check still
	// returned errCertURLTooLarge after the reader buffered the whole body.
	// Refusing the request and
	// BOUNDING the read are different properties, and only the second one stops an
	// attacker-operated server from being a memory exhaustion primitive. The row now
	// asserts the read actually stops, which is what the design specified
	// ("refused, and no more than the cap is read").
	t.Run("size cap", func(t *testing.T) {
		curlReset()
		const total = 8 << 20 // far past the cap, so stopping early is observable
		chunk := make([]byte, 64<<10)
		var written atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			for sent := 0; sent < total; sent += len(chunk) {
				n, err := w.Write(chunk)
				written.Add(int64(n))
				if err != nil {
					// The client stopped reading and closed. That is the pass.
					return
				}
			}
		}))
		defer srv.Close()

		// The hash is irrelevant here: the size check must fire before it.
		_, err := fetchHashAndURL(context.Background(),
			curlPayloadWithHash(make([]byte, 20), srv.URL), curlLoopbackAllow())
		if !errors.Is(err, errCertURLTooLarge) {
			t.Fatalf("an oversized body was not refused by the size cap: %v", err)
		}

		// The discriminating assertion. An unbounded reader consumes all 8 MiB and the
		// server writes every chunk. A bounded one stops, and the server's writes fail.
		// A generous ceiling keeps this robust against socket buffering while still
		// being far below the full body.
		if n := written.Load(); n >= total {
			t.Errorf("the server wrote %d octets of %d; the reader consumed the whole "+
				"body before complaining, so the cap bounds the verdict but not the "+
				"memory", n, total)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		curlReset()
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			<-release
			if _, err := w.Write(body); err != nil {
				return
			}
		}))
		defer srv.Close()
		defer close(release)

		// A context deadline shorter than certURLTimeout keeps the test fast. The
		// deadline exercises the same cancellation path the 5s bound uses.
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := fetchHashAndURL(ctx, curlPayload(body, srv.URL), curlLoopbackAllow())
		if err == nil {
			t.Fatal("a server that never responds did not produce an error")
		}
		if elapsed := time.Since(start); elapsed > certURLTimeout {
			t.Errorf("the fetch ran %v, past the %v bound", elapsed, certURLTimeout)
		}
	})

	t.Run("redirect", func(t *testing.T) {
		curlReset()
		var secondHits atomic.Int64
		second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			secondHits.Add(1)
			if _, err := w.Write(body); err != nil {
				return
			}
		}))
		defer second.Close()
		first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, second.URL, http.StatusFound)
		}))
		defer first.Close()

		_, err := fetchHashAndURL(context.Background(),
			curlPayload(body, first.URL), curlLoopbackAllow())
		if err == nil {
			t.Fatal("a redirect was followed")
		}
		if !strings.Contains(err.Error(), "redirect") {
			t.Errorf("the refusal does not name the redirect: %v", err)
		}
		if n := secondHits.Load(); n != 0 {
			t.Errorf("the redirect target received %d requests; it must receive none", n)
		}
	})

	t.Run("destination metadata address", func(t *testing.T) {
		curlReset()
		_, err := fetchHashAndURL(context.Background(),
			curlPayload(body, "http://169.254.169.254/latest/meta-data/"), nil)
		if err == nil {
			t.Fatal("the cloud metadata address was fetched")
		}
		if !errors.Is(err, errCertURLBlocked) {
			t.Errorf("the metadata address was not refused by the destination check: %v", err)
		}
	})

	t.Run("destination private range", func(t *testing.T) {
		curlReset()
		for _, target := range []string{
			"http://10.0.0.1/cert", "http://192.168.1.1/cert", "http://172.16.0.1/cert",
			"http://127.0.0.1/cert",
		} {
			_, err := fetchHashAndURL(context.Background(), curlPayload(body, target), nil)
			if !errors.Is(err, errCertURLBlocked) {
				t.Errorf("%s was not refused by the destination check: %v", target, err)
			}
		}
	})

	// THE ROW THAT MATTERS MOST. The server returns well-formed bytes that are not the
	// object the peer named. A fetcher that parsed before it compared the hash would
	// have handed these to x509.ParseCertificate already.
	t.Run("hash mismatch", func(t *testing.T) {
		curlReset()
		other := []byte("a-completely-different-but-valid-looking-DER")
		srv := httptest.NewServer(curlServe(other))
		defer srv.Close()

		// The payload names body's hash, and the server serves other.
		sum := sha1.Sum(body)
		got, err := fetchHashAndURL(context.Background(),
			curlPayloadWithHash(sum[:], srv.URL), curlLoopbackAllow())
		if err == nil {
			t.Fatal("bytes that do not match the peer's hash were accepted")
		}
		if !errors.Is(err, errCertURLHash) {
			t.Fatalf("the refusal did not come from the hash comparison: %v", err)
		}
		if got != nil {
			t.Error("bytes were returned to the caller despite the hash mismatch; " +
				"nothing may reach a parser before the hash is verified")
		}
		// Nothing was cached, so a retry cannot pick up the rejected bytes.
		if _, cached := certURLCache.Load(string(sum[:])); cached {
			t.Error("bytes that failed the hash check were cached")
		}
	})

	t.Run("payload shorter than the hash", func(t *testing.T) {
		curlReset()
		_, err := fetchHashAndURL(context.Background(), make([]byte, 10), curlLoopbackAllow())
		if !errors.Is(err, errCertURLShortData) {
			t.Fatalf("a truncated payload was not refused: %v", err)
		}
		// Exactly the hash length and no URL is also refused.
		_, err = fetchHashAndURL(context.Background(), make([]byte, 20), curlLoopbackAllow())
		if !errors.Is(err, errCertURLShortData) {
			t.Fatalf("a payload with a hash and no URL was not refused: %v", err)
		}
	})
}

// VALIDATES: certURLDenied refuses every address class the fetcher must never reach, and
// admits a public one, so the deny list is a filter rather than a blanket refusal.
// PREVENTS: a narrowing of one clause silently opening a class. The metadata address is
// asserted separately from link-local for exactly that reason.
// rfc-test-change-approved: 2026-08-01 owner standing approval for
// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, strengthening only. Rows are ADDED to the deny
// set and to the reachable set. Nothing is removed or relaxed.
func TestCertURLDeniedCoversEveryPrivateClass(t *testing.T) {
	for _, s := range []string{
		"127.0.0.1", "::1", "10.1.2.3", "192.168.0.5", "172.20.0.1",
		"169.254.169.254", "169.254.1.1", "0.0.0.0", "224.0.0.1",
		"fc00::1", "fd12:3456::1", "fe80::1", "ff02::1",
		// RFC 6598 shared address space. netip.Addr.IsPrivate does not name it. On a BNG
		// it is the subscriber range, and the highest-value target of this deny list on
		// ze's own target platform.
		"100.64.0.1", "100.64.0.0", "100.127.255.255",
		// RFC 6890 IETF protocol assignments, and RFC 2544 benchmarking.
		"192.0.0.1", "192.0.0.170", "198.18.0.1", "198.19.255.255",
		// RFC 1112 reserved: IsMulticast stops at 239.255.255.255.
		"240.0.0.1", "255.255.255.254",
		// The IPv4-mapped form of RFC 4291. Unmap folds it, so the IPv4 rules apply.
		"::ffff:10.0.0.1", "::ffff:100.64.0.1", "::ffff:169.254.169.254",
		// The IPv4-COMPATIBLE form of RFC 4291, which Unmap does NOT fold. Without the
		// embedded-address reading these walk past every IPv4 rule as plain IPv6.
		"::10.0.0.1", "::100.64.0.1", "::169.254.169.254", "::192.168.0.5",
	} {
		if !certURLDenied(netip.MustParseAddr(s)) {
			t.Errorf("certURLDenied(%s) = false; it must be refused", s)
		}
	}
	for _, s := range []string{
		"93.184.216.34", "8.8.8.8", "2606:2800:220:1::1",
		// The boundaries just outside each added class, so the prefixes cannot be
		// widened without this test noticing.
		"100.63.255.255", "100.128.0.0", "192.0.1.1", "198.17.255.255", "198.20.0.0",
		// The mapped form of a PUBLIC address still reaches the network.
		"::ffff:93.184.216.34",
	} {
		if certURLDenied(netip.MustParseAddr(s)) {
			t.Errorf("certURLDenied(%s) = true; a public address must be reachable", s)
		}
	}
}
