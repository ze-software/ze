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
	"log/slog"
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

	// certURLMaxHeaderBytes caps the RESPONSE HEADER. It is a separate budget from
	// certURLMaxBytes, which caps only the body, and Go's default for it is 10 MiB.
	// Without this the body cap bounds nothing that matters: an attacker-operated
	// server answers with megabytes of header fields, and certURLMaxInFlight
	// multiplies whatever one response costs. 8 KiB is far above any real answer to a
	// GET for a certificate.
	certURLMaxHeaderBytes = 8 << 10

	// certURLMaxPending bounds the background lookups certURLFetcher can run at one time.
	//
	// An unauthenticated peer names the hashes. Every dropped IKE_AUTH is sent again. An
	// unbounded pending set is therefore a goroutine-growth primitive, and handshakes that
	// never authenticate drive it. certURLMaxInFlight refuses a fetch beyond its own budget
	// and does not block, so a worker past this bound returns at once.
	certURLMaxPending = certURLMaxInFlight

	// certURLCacheMaxEntries and certURLCacheMaxBytes bound the cache below.
	//
	// The key is a hash the PEER chose, so an unauthenticated peer decides how many
	// distinct entries exist. Each IKE_AUTH can name several, and each is up to
	// certURLMaxBytes. An unbounded cache is therefore a heap-growth primitive driven
	// by repeated handshakes that never authenticate.
	//
	// 64 entries at the 64 KiB body cap is exactly the 4 MiB byte bound. The two meet
	// for maximum-size objects. The entry bound binds first for ordinary ones.
	certURLCacheMaxEntries = 64
	certURLCacheMaxBytes   = 4 << 20

	// certURLFailBackoff is how long a failed hash-and-url lookup is remembered before
	// another attempt is made for the same hash.
	//
	// It bounds what a peer's retransmissions can spend. maxRetransmissions is 7 and the
	// retransmit schedule backs off exponentially from retransmitBase, so a whole
	// handshake's worth of repeats falls inside this window and costs ONE outbound GET
	// rather than one per retransmission. It is deliberately shorter than a reconnect
	// cycle, so an operator who fixes the certificate server is not made to wait.
	certURLFailBackoff = 30 * time.Second
)

var (
	errCertURLScheme    = errors.New("ike cert-url: only the http scheme is supported for hash-and-url lookup")
	errCertURLTooLarge  = errors.New("ike cert-url: the response exceeds the size cap")
	errCertURLRedirect  = errors.New("ike cert-url: the server attempted a redirect")
	errCertURLBlocked   = errors.New("ike cert-url: the URL resolves to an address the fetcher refuses")
	errCertURLHash      = errors.New("ike cert-url: the fetched bytes do not match the hash the peer sent")
	errCertURLBusy      = errors.New("ike cert-url: too many certificate lookups are already in flight")
	errCertURLShortData = errors.New("ike cert-url: the payload is shorter than the 20-octet hash it must carry")

	// errCertURLPending reports that the object is not cached. A background lookup for it
	// has started.
	//
	// It is NOT a refusal, and it must never kill the SA. The caller DROPS the message that
	// named the object. RFC 7296 Section 2.1 has the peer send it again, and that message
	// finds the object cached.
	errCertURLPending = errors.New("ike cert-url: the lookup is in flight, so the message is dropped and the peer will retransmit")
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
//
// It is BOUNDED, in entries and in bytes, and it evicts in insertion order. A peer that
// has not authenticated chooses the key, so an unbounded map lets repeated handshakes
// grow the heap without limit. Eviction costs a later peer one refetch, and the fetch is
// bounded in every other dimension already.
//
// A sync.Map cannot carry this: it reports no size, so nothing can decide when to evict.
type certURLStore struct {
	mu    sync.Mutex
	items map[string][]byte
	order []string // insertion order, oldest first
	bytes int
}

var certURLCache = certURLStore{items: make(map[string][]byte)}

// Load returns the cached DER for a hash key.
//
// Load and Store keep the names of the sync.Map methods this type replaced. The call
// sites that read the cache therefore did not change with it.
func (c *certURLStore) Load(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	der, ok := c.items[key]
	return der, ok
}

// Store records der under key and evicts the oldest entries until both bounds hold.
//
// An object larger than the whole byte budget is never stored. Storing it would evict
// every other entry and then sit alone, which is a cache one peer can flush at will.
// certURLMaxBytes already bounds a body well below the budget, so this refuses only a
// value a future edit of that cap would let through (ai/rules/fail-closed-guards.md).
func (c *certURLStore) Store(key string, der []byte) {
	if len(der) > certURLCacheMaxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.items[key]; ok {
		// The key is the SHA-1 of the value, so a repeat carries identical bytes and
		// the entry keeps its original position. Only the accounting is restated.
		c.bytes += len(der) - len(old)
		c.items[key] = der
		return
	}
	c.items[key] = der
	c.order = append(c.order, key)
	c.bytes += len(der)
	for len(c.order) > certURLCacheMaxEntries || c.bytes > certURLCacheMaxBytes {
		oldest := c.order[0]
		c.order = c.order[1:]
		c.bytes -= len(c.items[oldest])
		delete(c.items, oldest)
	}
}

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
	certURLCache.mu.Lock()
	defer certURLCache.mu.Unlock()
	certURLCache.items = make(map[string][]byte)
	certURLCache.order = nil
	certURLCache.bytes = 0
}

// certURLFetcher runs hash-and-URL lookups OFF the shared IKE dispatch goroutine.
//
// THIS TYPE EXISTS BECAUSE THE FETCH USED TO BLOCK EVERY IKE SESSION. A half-open
// handshake is driven INLINE on the one dispatch goroutine that serves every peer
// (routeInbound, register.go: the owner-loop hand-off needs ps.ownedSA, and runEstablished
// is the only writer of it, so nothing owns an SA until it is established). Resolution ran
// on that goroutine, before verifyRemoteAuth. One unauthenticated peer therefore chose both
// the number of lookups, up to its certificate-count, and the server that answers each of
// them, at certURLTimeout apiece. That is every other peer's IKE stopped for as long as the
// attacker likes.
//
// Deferring the fetch until after authentication is not available: verifyRemoteAuth needs
// the certificate to verify the signature, so the object must be in hand first. The fetch
// moves instead.
//
// A cache HIT costs no I/O and the handshake continues inline. A MISS starts a worker and
// the message is dropped. The peer retransmits it (RFC 7296 Section 2.1) and the
// retransmission finds the object cached. That is the same drop-and-retransmit hand-off
// routeInbound already uses when an owner queue is full, and it is survivable for the same
// reason. Nothing here advances a Message ID or caches a response, so the retransmission is
// processed as the fresh request it is.
//
// Every bound stays where it was. This type performs NO network work of its own. It calls
// fetchHashAndURL, and that function still applies every control:
//
//   - the size cap and the timeout;
//   - the redirect refusal and the in-flight cap;
//   - the destination deny list;
//   - the hash comparison that comes before any parser.
//
// Only the goroutine the call runs on has changed.
type certURLFetcher struct {
	mu      sync.Mutex
	pending map[string]struct{}
	// failed records when a lookup last failed, so a repeat is not attempted again until
	// certURLFailBackoff has passed.
	//
	// WITHOUT THIS THE RETRANSMIT IS THE ATTACK. A miss drops the IKE_AUTH, the peer
	// retransmits it (RFC 7296 Section 2.1), and the retransmission re-enters start.
	// On success the cache answers the repeat. On FAILURE nothing was cached and the
	// worker had already removed its own pending key, so every retransmission launched a
	// fresh outbound GET to a host the unauthenticated peer chose. The peer controls both
	// the destination and the repeat rate, and the retransmit schedule multiplies it.
	//
	// Before the fetch moved off the dispatch goroutine the SA simply died after one
	// attempt, so the repeat did not exist. Bounding it here restores that ceiling
	// without putting the stall back.
	//
	// THE KEY CARRIES THE PEER, and the object cache above deliberately does not. The two
	// records answer different questions. A cached object is content-addressed and its
	// bytes hash to its own key, so it is equally true for every peer. A FAILURE is a fact
	// about one peer's attempt: the URL and the allow-list that refused it are both that
	// peer's. Keyed on the hash alone, any peer that completes IKE_SA_INIT could name a
	// victim's certificate hash beside a URL its own allow-list rejects, record the
	// failure, and suppress the victim's legitimate lookup for certURLFailBackoff,
	// renewably. The backoff exists to bound ONE peer's retransmissions, so it is scoped to
	// the peer whose retransmissions it bounds.
	failed map[certURLFailure]time.Time
}

// certURLFailure names one peer's failed lookup of one object.
//
// A struct key rather than a joined string: the peer name is operator-chosen text and the
// hash is arbitrary octets, so no separator is safe against a peer name that contains it.
type certURLFailure struct {
	peer string
	hash string
}

var certURLFetches = certURLFetcher{
	pending: make(map[string]struct{}),
	failed:  make(map[certURLFailure]time.Time),
}

// start begins a background lookup for one Hash and URL payload, on behalf of one peer.
//
// It refuses a second worker for an object a worker already holds, so a storm of repeats
// cannot multiply goroutines. It refuses any worker at all past certURLMaxPending.
//
// Both refusals are silent, and each costs the peer one more repeat of its message. The
// miss stays explicit at the caller, which returns errCertURLPending in either case
// (ai/rules/fail-closed-guards.md).
//
// The two records it consults are scoped differently on purpose. The pending set is keyed
// on the hash alone, so two peers naming the same object share ONE fetch, which is the
// deduplication the content-addressed cache is built around. The failure record is keyed
// on the peer as well, so one peer's failure can never suppress another's lookup
// (certURLFailure, above).
func (f *certURLFetcher) start(peer string, data []byte, allow []netip.Prefix, log *slog.Logger) {
	hash, _, err := splitHashAndURL(data)
	if err != nil {
		return
	}
	key := string(hash)
	failKey := certURLFailure{peer: peer, hash: key}

	f.mu.Lock()
	if _, running := f.pending[key]; running {
		f.mu.Unlock()
		return
	}
	// A lookup this peer just failed is not retried until the backoff expires. The peer
	// retransmits its IKE_AUTH on its own schedule, and without this every retransmission
	// is another outbound GET to a destination it chose.
	if until, ok := f.failed[failKey]; ok {
		if time.Now().Before(until) {
			f.mu.Unlock()
			log.Debug("ike: hash-and-url lookup recently failed, not retrying yet", "peer", peer)
			return
		}
		delete(f.failed, failKey)
	}
	if len(f.pending) >= certURLMaxPending {
		f.mu.Unlock()
		log.Debug("ike: hash-and-url lookups are at the pending bound, not starting another")
		return
	}
	f.pending[key] = struct{}{}
	f.mu.Unlock()

	// The payload is copied because it points into the transport read buffer, which is
	// reused as soon as this message is dropped. The worker outlives that.
	payload := make([]byte, len(data))
	copy(payload, data)

	go func() {
		// context.Background rather than a per-message context: the message that named
		// this object is already dropped, so no caller waits on the result. The bound
		// that matters is certURLTimeout, which fetchHashAndURL applies itself.
		_, err := fetchHashAndURL(context.Background(), payload, allow)
		f.mu.Lock()
		delete(f.pending, key)
		if err != nil {
			// The failure is recorded BEFORE the pending key is released to the next
			// caller, in one critical section, so a retransmission that arrives between
			// the two cannot slip through and start a second GET.
			f.noteFailureLocked(failKey)
		}
		f.mu.Unlock()
		if err != nil {
			log.Debug("ike: background hash-and-url lookup failed", "error", err)
		}
	}()
}

// noteFailureLocked records a failed lookup for the backoff, and bounds the record set.
//
// Half of the key is a hash the PEER chose, so the failure map is attacker-sized exactly
// like the cache above it. certURLCacheMaxEntries is the same bound the cache uses; past it
// the whole set is dropped rather than evicted one entry at a time. Dropping is safe
// because the map is an optimisation: losing it costs one more GET per hash, which is the
// behavior that existed before this record did. The caller must hold f.mu.
func (f *certURLFetcher) noteFailureLocked(key certURLFailure) {
	if len(f.failed) >= certURLCacheMaxEntries {
		clear(f.failed)
	}
	f.failed[key] = time.Now().Add(certURLFailBackoff)
}

// inFlight reports how many background lookups are running. Only tests read it.
func (f *certURLFetcher) inFlight() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pending)
}

// lookupHashAndURL returns the DER for a Hash and URL payload when the content-addressed
// cache already holds it.
//
// It performs NO I/O and takes no lock other than the cache's own. It is therefore the half
// of the resolution that is safe on the shared dispatch goroutine.
//
// certURLStore records an entry only after its hash comparison passed, so a hit is as
// trustworthy as a fetch.
func lookupHashAndURL(hash []byte) ([]byte, bool) {
	return certURLCache.Load(string(hash))
}

// splitHashAndURL splits a Hash and URL payload into the 20-octet SHA-1 and the URL text
// that follows it (RFC 7296 Section 3.6).
func splitHashAndURL(data []byte) (hash []byte, rawURL string, err error) {
	if len(data) <= wire.CertHashURLHashLen {
		return nil, "", errCertURLShortData
	}
	return data[:wire.CertHashURLHashLen], string(data[wire.CertHashURLHashLen:]), nil
}

// certURLDeniedPrefixes are the address classes netip's own predicates do not name.
//
// netip.Addr.IsPrivate covers RFC 1918 and fc00::/7 and NOTHING else. Every class below
// would otherwise be reachable. Each is space that is live INSIDE a network, and none is
// a legitimate place to fetch a certificate from. That is the shape this file refuses.
var certURLDeniedPrefixes = []netip.Prefix{
	// RFC 6598 shared address space. Ze runs on a BNG, where this is the SUBSCRIBER
	// range. Without this row an unauthenticated peer names a subscriber's address, and
	// drives a request at it from the router's position.
	netip.MustParsePrefix("100.64.0.0/10"),
	// RFC 6890 IETF protocol assignments, which carries the DS-Lite AFTR address
	// (192.0.0.1) and the NAT64 well-known addresses among others.
	netip.MustParsePrefix("192.0.0.0/24"),
	// RFC 2544 benchmarking, routinely wired to live test equipment.
	netip.MustParsePrefix("198.18.0.0/15"),
	// RFC 1112 reserved. IsMulticast stops at 239.255.255.255 and leaves this open.
	netip.MustParsePrefix("240.0.0.0/4"),
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
	// Unmap folds the IPv4-mapped form of RFC 4291 (::ffff:0:0/96) down to its IPv4
	// address, so every IPv4 rule below judges ::ffff:10.0.0.1 exactly as 10.0.0.1.
	// The IPv4-COMPATIBLE form is a different encoding and is handled separately.
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
	// IPv6 unique-local space, fc00::/7. IsPrivate already names it. It is restated for
	// the same reason the metadata address above is.
	if addr.Is6() && addr.As16()[0]&0xfe == 0xfc {
		return true
	}
	// The IPv4-compatible IPv6 form of RFC 4291, ::a.b.c.d. Unmap does NOT fold it,
	// because it is not the mapped form. ::10.0.0.1 therefore reaches here as a plain
	// IPv6 address that matches no rule above.
	//
	// RFC 4291 Section 2.5.5.1 deprecated the encoding. A peer that sends one names an
	// IPv4 destination by the one spelling that walks past an IPv4 deny list. The
	// embedded address is judged instead.
	if compat, ok := ipv4Compatible(addr); ok {
		return certURLDenied(compat)
	}
	for _, p := range certURLDeniedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ipv4Compatible reports the IPv4 address embedded in an IPv4-compatible IPv6 address
// (RFC 4291 Section 2.5.5.1: ::a.b.c.d, the deprecated form).
//
// :: and ::1 are excluded: IsUnspecified and IsLoopback already name them, and reading
// them as 0.0.0.0 and 0.0.0.1 would describe them as something they are not.
func ipv4Compatible(addr netip.Addr) (netip.Addr, bool) {
	if !addr.Is6() || addr.Is4In6() {
		return netip.Addr{}, false
	}
	b := addr.As16()
	for _, octet := range b[:12] {
		if octet != 0 {
			return netip.Addr{}, false
		}
	}
	v4 := netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
	if !v4.IsValid() || v4.IsUnspecified() || v4 == netip.AddrFrom4([4]byte{0, 0, 0, 1}) {
		return netip.Addr{}, false
	}
	return v4, true
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
			// Go's default here is 10 MiB, which is 160 times the body cap and is
			// what actually bounds a hostile response until it is set. Header bytes
			// are read before certURLMaxBytes governs anything.
			MaxResponseHeaderBytes: certURLMaxHeaderBytes,
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

	if der, ok := certURLCache.Load(string(hash)); ok {
		return der, nil
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
