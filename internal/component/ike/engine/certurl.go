// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// RFC: rfc/short/rfc7296.md -- Hash and URL certificate encodings (Section 3.6)
// Related: auth.go -- the CERT payload assembly and remote certificate store this feeds

package engine

import (
	"context"
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.6 mandates SHA-1 as the hash-and-url object identifier
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// The bound on a hash-and-URL lookup.
//
// RFC 7296 Section 3.6 requires an implementation be capable of being configured to
// accept the two Hash and URL formats. Section 3.6 further requires support for the
// "http:" scheme for hash-and-URL lookup. The accept half means Ze fetches a URL chosen
// by a peer that is NOT YET AUTHENTICATED. Ze must retrieve the certificate before Ze can
// authenticate that certificate. The fetch is therefore a pre-authentication,
// attacker-controlled outbound request made with the daemon's network position. That
// request is a server-side request forgery primitive unless every control below holds.
//
// The feature is off unless an operator turns it on (AuthConfig.HashAndURL). With it off
// Ze never advertises HTTP_CERT_LOOKUP_SUPPORTED, so a conforming peer never sends a
// hash-and-URL payload. The collection loops drop a non-conforming one, and no code path
// below is reachable.
const (
	// certURLMaxBytes caps the response body. A certificate is a few KiB and a bundle is
	// a SEQUENCE OF, so 64 KiB is generous. Reading without a cap from an
	// attacker-operated server is a memory exhaustion primitive.
	certURLMaxBytes = 64 << 10

	// certURLTimeout bounds connect, headers and body together. The fetch blocks
	// IKE_AUTH, so an unbounded one holds a goroutine and its buffers per half-open SA.
	certURLTimeout = 5 * time.Second

	// certURLMaxInFlight caps concurrent fetches process-wide. Without it, N half-open
	// SAs are N outbound connections, which is a reflection amplifier.
	certURLMaxInFlight = 8
)

var (
	errCertURLScheme    = errors.New("ike cert-url: only the http scheme is supported for hash-and-url lookup")
	errCertURLTooLarge  = errors.New("ike cert-url: the response exceeds the size cap")
	errCertURLRedirect  = errors.New("ike cert-url: the server attempted a redirect")
	errCertURLBlocked   = errors.New("ike cert-url: the URL resolves to an address the fetcher refuses")
	errCertURLHash      = errors.New("ike cert-url: the fetched bytes do not match the hash the peer sent")
	errCertURLBusy      = errors.New("ike cert-url: too many certificate lookups are already in flight")
	errCertURLShortData = errors.New("ike cert-url: the payload is shorter than the 20-octet hash it must carry")
)

// certURLInFlight is the global concurrency bound.
var certURLInFlight = make(chan struct{}, certURLMaxInFlight)

// certURLCache is keyed by the SHA-1 hash the peer sent, never by the URL. RFC 7296
// Section 3.6 names caching as the point of the feature. The hash key makes the cache
// content-addressed. A URL that serves different bytes on a later request cannot poison
// an entry. And two peers naming the same object share one fetch.
//
// An entry is stored only AFTER the hash comparison below passed, so every entry's bytes
// hash to its own key. A cache HIT therefore returns bytes that satisfy the peer's hash by
// construction. That is why the hit CAN return before any of the network controls run.
var certURLCache sync.Map // string(hash) -> []byte DER

// resetCertURLCache drops every cached object.
//
// The cache is process-global and outlives any one SA. That makes the cache useful, and it
// also makes the cache invisible. A lookup that never touches the network looks identical
// to one that did.
//
// A test must therefore state which of the two it exercises. Without a reset, an entry an
// earlier test stored can silently answer a test that asserts the hash comparison. That
// test then passes and proves nothing.
func resetCertURLCache() {
	certURLCache.Range(func(key, _ any) bool {
		certURLCache.Delete(key)
		return true
	})
}

// splitHashAndURL splits a Hash and URL payload into the 20-octet SHA-1 and the URL text
// that follows it (RFC 7296 Section 3.6).
func splitHashAndURL(data []byte) (hash []byte, rawURL string, err error) {
	if len(data) <= wire.CertHashURLHashLen {
		return nil, "", errCertURLShortData
	}
	return data[:wire.CertHashURLHashLen], string(data[wire.CertHashURLHashLen:]), nil
}

// certURLDenied reports whether an address is one the fetcher must never connect to.
//
// The daemon runs on a router and holds routes an internet host does not. Without this
// check the fetch is an internal port scanner driven by an unauthenticated peer. The cloud
// metadata address is called out because it is the highest-value target of the class.
//
// The check runs at DIAL time, against the address Ze connects to. It does not run against
// a pre-resolved answer. A name that resolves differently on a second lookup therefore
// cannot walk past it (DNS rebinding).
func certURLDenied(addr netip.Addr) bool {
	addr = addr.Unmap()
	switch {
	case !addr.IsValid(),
		addr.IsLoopback(),
		addr.IsPrivate(),
		addr.IsLinkLocalUnicast(),
		addr.IsLinkLocalMulticast(),
		addr.IsMulticast(),
		addr.IsUnspecified(),
		addr.IsInterfaceLocalMulticast():
		return true
	}
	// 169.254.169.254 is link-local and already denied above. State it so a future
	// narrowing of the link-local rule cannot silently expose it.
	if addr.Is4() && addr == netip.AddrFrom4([4]byte{169, 254, 169, 254}) {
		return true
	}
	// IPv4-compatible and IPv6 unique-local space.
	if addr.Is6() && addr.As16()[0]&0xfe == 0xfc {
		return true
	}
	return false
}

// certURLClient builds the bounded HTTP client. Every control is set here so a reader can
// audit them in one place, and so no caller can get a client missing one.
//
// allow names extra destinations an operator has permitted. It widens the deny list above,
// which is otherwise absolute.
func certURLClient(allow []netip.Prefix) *http.Client {
	dialer := &net.Dialer{
		Timeout: certURLTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return errCertURLBlocked
			}
			addr, err := netip.ParseAddr(host)
			if err != nil {
				return errCertURLBlocked
			}
			for _, p := range allow {
				if p.Contains(addr.Unmap()) {
					return nil
				}
			}
			if certURLDenied(addr) {
				return fmt.Errorf("%w: %s", errCertURLBlocked, addr)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: certURLTimeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			DisableKeepAlives:     true,
			MaxIdleConns:          1,
			ResponseHeaderTimeout: certURLTimeout,
		},
		// Zero redirects. A redirect is the standard scheme-and-host laundering step.
		// The hash pins the CONTENT rather than the location, so a redirect gains
		// nothing. And a redirect re-opens the destination check against a new host.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errCertURLRedirect
		},
	}
}

// fetchHashAndURL resolves a Hash and URL certificate payload to its DER bytes.
//
// THE ORDERING BELOW IS THE SECURITY PROPERTY OF THIS FUNCTION. The SHA-1 of the fetched
// bytes is compared against the hash the peer sent BEFORE the bytes are handed to any
// parser. RFC 7296 Section 3.6 makes the hash the identity of the fetched object. Bytes
// that fail it are not a certificate at all, and they must never reach
// x509.ParseCertificate.
//
// A fetcher that parsed first and compared afterwards would satisfy every other control
// here. That fetcher would still hand attacker-chosen bytes to the X.509 parser, which is
// the actual attack surface. Do not reorder these two steps.
//
// SHA-1 is the RFC's choice and is used here only to match a peer's identification of an
// object it also names by URL. It is not relied on for authentication: the certificate
// that comes back is still chained to ca-certificate and still bound to remote-id by
// getRemoteCert.
func fetchHashAndURL(ctx context.Context, data []byte, allow []netip.Prefix) ([]byte, error) {
	hash, rawURL, err := splitHashAndURL(data)
	if err != nil {
		return nil, err
	}

	if cached, ok := certURLCache.Load(string(hash)); ok {
		if der, isDER := cached.([]byte); isDER {
			return der, nil
		}
	}

	// The scheme is checked before any name resolution or connection, so a file: or
	// ftp: URL costs no I/O at all. RFC 7296 Section 3.6 makes http the required
	// scheme and says other schemes SHOULD NOT be used absent a document specifying
	// them. That is the basis for refusing rather than attempting them.
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ike cert-url: parse url: %w", err)
	}
	if u.Scheme != "http" {
		return nil, fmt.Errorf("%w: %q", errCertURLScheme, u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("%w: no host", errCertURLScheme)
	}

	select {
	case certURLInFlight <- struct{}{}:
		defer func() { <-certURLInFlight }()
	default:
		return nil, errCertURLBusy
	}

	ctx, cancel := context.WithTimeout(ctx, certURLTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("ike cert-url: build request: %w", err)
	}
	resp, err := certURLClient(allow).Do(req)
	if err != nil {
		return nil, fmt.Errorf("ike cert-url: fetch %s: %w", u.Redacted(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ike cert-url: fetch %s: status %d", u.Redacted(), resp.StatusCode)
	}

	// One octet beyond the cap is read so that hitting the cap is distinguishable from
	// a body that happens to be exactly the cap.
	body, err := io.ReadAll(io.LimitReader(resp.Body, certURLMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("ike cert-url: read %s: %w", u.Redacted(), err)
	}
	if len(body) > certURLMaxBytes {
		return nil, fmt.Errorf("%w: %s returned more than %d octets",
			errCertURLTooLarge, u.Redacted(), certURLMaxBytes)
	}

	// ---- The load-bearing step. Nothing below parses, and nothing above did. ----
	sum := sha1.Sum(body) //nolint:gosec // mandated by RFC 7296 Section 3.6; identifies the object, never authenticates it
	if subtle.ConstantTimeCompare(sum[:], hash) != 1 {
		return nil, fmt.Errorf("%w: %s", errCertURLHash, u.Redacted())
	}

	certURLCache.Store(string(hash), body)
	return body, nil
}
