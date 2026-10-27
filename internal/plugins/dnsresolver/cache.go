// Design: docs/architecture/dns/lan-resolver.md -- the answer and negative caches, ranking and LRU
// Related: state.go -- the resolver that owns both caches
// Related: resolve.go -- what a reply puts into them

package dnsresolver

import (
	"container/list"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// typeNameError keys a name-error (NXDOMAIN) entry in the negative cache.
// RFC 6895 Section 3.1 reserves type 0, so no query type collides with it.
const typeNameError uint16 = 0

// ttlSignBit is the most significant bit of a received TTL.
const ttlSignBit = 1 << 31

// cacheKey names one RRset, or one negative answer, by owner, type and
// class. The owner is lower-cased once at construction.
type cacheKey struct {
	name   string
	qtype  uint16
	qclass uint16
}

// keyOf builds the cache key of name, qtype and qclass.
//
// RFC 1035 Section 2.3.3: "For all parts of the DNS that are part of the
// official protocol, all comparisons between character strings (e.g.,
// labels, domain names, etc.) are done in a case-insensitive manner."
//
// The key folds case so every spelling reaches one entry; the records keep
// the case they arrived with, and the reply's question echoes the query's.
func keyOf(name string, qtype, qclass uint16) cacheKey {
	return cacheKey{name: strings.ToLower(dns.Fqdn(name)), qtype: qtype, qclass: qclass}
}

// rank is how trustworthy cached data is. Zero is unspecified, so an entry
// built without a rank never outranks real data.
type rank uint8

const (
	rankUnspecified rank = iota
	// rankAnswer is the answer section of a non-authoritative reply, which
	// is what a forwarding upstream sends, and every RRset in the answer
	// section of an authoritative reply that the name sought does not own.
	rankAnswer
	// rankAnswerAuthoritative is the RRset owned by the name sought in the
	// answer section of a reply with AA set.
	rankAnswerAuthoritative
)

// rankOf is the rank of the answer-section RRset owned by owner in reply, a
// reply to a query for sought.
//
// RFC 2181 Section 5.4.1: "An authoritative answer from a reply should
// replace cached data that had been obtained from additional information in
// an earlier reply."
//
// RFC 2181 Section 5.4.1: "However when the name sought is an alias (see
// section 10.1.1) only the record describing that alias is necessarily
// authoritative.  Clients should assume that other records may have come
// from the server's cache."
//
// RFC 2181 Section 5.4.1 ranks "Data from the answer section of a
// non-authoritative answer, and non-authoritative data from the answer
// section of authoritative answers," together, so an RRset on a CNAME chain
// past the name sought ranks as a non-authoritative answer even when AA is
// set: only the RRset sought owns, the alias record or the answer itself,
// takes the authoritative rank.
//
// Only answer-section data is ranked: the resolver caches nothing from the
// authority or additional section but the SOA of a negative answer, so
// additional data can never be returned as an answer.
func rankOf(reply *dns.Msg, owner, sought string) rank {
	if !reply.Authoritative {
		return rankAnswer
	}
	if !strings.EqualFold(dns.Fqdn(owner), dns.Fqdn(sought)) {
		return rankAnswer
	}
	return rankAnswerAuthoritative
}

// cacheEntry is one cached RRset, or one negative answer whose rrs hold the
// SOA. It is immutable once stored: a hit copies the records it returns.
type cacheEntry struct {
	key    cacheKey
	rrs    []dns.RR
	rcode  int
	rank   rank
	stored time.Time
	ttl    uint32
	elem   *list.Element
}

// remaining is the entry's TTL at now in whole seconds, and false once it
// has expired.
//
// RFC 2308 Section 5: "This TTL decrements in a similar manner to a normal
// cached answer and upon reaching zero (0) indicates the cached negative
// answer MUST NOT be used again."
func (e *cacheEntry) remaining(now time.Time) (uint32, bool) {
	elapsed := max(now.Sub(e.stored)/time.Second, 0)
	if uint64(elapsed) >= uint64(e.ttl) {
		return 0, false
	}
	return e.ttl - uint32(elapsed), true
}

// lru is one bounded cache with least-recently-used eviction. A limit of
// zero stores nothing. Safe for concurrent use.
type lru struct {
	mu      sync.Mutex
	limit   int
	entries map[cacheKey]*cacheEntry
	order   *list.List
}

func newLRU(limit int) *lru {
	return &lru{limit: limit, entries: make(map[cacheKey]*cacheEntry), order: list.New()}
}

// get returns the live entry for key and marks it used. An expired entry is
// removed and reported as a miss.
func (c *lru) get(key cacheKey, now time.Time) (*cacheEntry, uint32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, 0, false
	}
	ttl, live := e.remaining(now)
	if !live {
		c.removeLocked(e)
		return nil, 0, false
	}
	c.order.MoveToFront(e.elem)
	return e, ttl, true
}

// put stores e, replacing the whole entry under its key, and reports
// whether it was stored.
//
// RFC 2181 Section 5.4: "Servers must never merge RRs from a response with
// RRs in their cache to form an RRSet.  If a response contains data that
// would form an RRSet with data in a server's cache the server must either
// ignore the RRs in the response, or discard the entire RRSet currently in
// the cache, as appropriate."
//
// RFC 2181 Section 5.4.1: "However additional information from a reply will
// be ignored if the cache contains data from an authoritative answer or a
// zone file."
//
// A live entry of a higher rank keeps its place and e is ignored; otherwise
// the old entry is discarded whole.
func (c *lru) put(e *cacheEntry, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.rank == rankUnspecified {
		panic("BUG: dnsresolver: cache entry without a rank")
	}
	if c.limit <= 0 {
		return false
	}
	if e.ttl == 0 {
		return false
	}
	if old, ok := c.entries[e.key]; ok {
		if _, live := old.remaining(now); live && old.rank > e.rank {
			return false
		}
		c.removeLocked(old)
	}
	e.elem = c.order.PushFront(e)
	c.entries[e.key] = e
	c.evictLocked()
	return true
}

// setLimit changes the bound, evicting least-recently-used entries over it.
func (c *lru) setLimit(limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limit = limit
	c.evictLocked()
}

// clear removes every entry.
func (c *lru) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[cacheKey]*cacheEntry)
	c.order.Init()
}

// size is the number of entries held, live or not yet swept.
func (c *lru) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// sweep removes every expired entry. Bounded by the entry count.
func (c *lru) sweep(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if _, live := e.remaining(now); !live {
			c.removeLocked(e)
		}
	}
}

func (c *lru) evictLocked() {
	for len(c.entries) > c.limit {
		back := c.order.Back()
		if back == nil {
			return
		}
		e, ok := back.Value.(*cacheEntry)
		if !ok {
			panic("BUG: dnsresolver: cache list holds a non-entry")
		}
		c.removeLocked(e)
	}
}

func (c *lru) removeLocked(e *cacheEntry) {
	c.order.Remove(e.elem)
	delete(c.entries, e.key)
}

// receivedTTL is the TTL a received RR is trusted for.
//
// RFC 2181 Section 8: "Implementations should treat TTL values received
// with the most significant bit set as if the entire value received was
// zero."
//
// RFC 2181 Section 8: "Implementations are always free to place an upper
// bound on any TTL received, and treat any larger values as if they were
// that upper bound."
func receivedTTL(ttl, limit uint32) uint32 {
	if ttl&ttlSignBit != 0 {
		return 0
	}
	return min(ttl, limit)
}

// withTTL returns copies of rrs carrying ttl, so a stored or shared RR is
// never written.
func withTTL(rrs []dns.RR, ttl uint32) []dns.RR {
	out := make([]dns.RR, len(rrs))
	for i, rr := range rrs {
		c := dns.Copy(rr)
		c.Header().Ttl = ttl
		out[i] = c
	}
	return out
}

// lookup answers q from the caches alone, following cached CNAMEs within
// the restart bound. It reports false on any miss, so a partly cached chain
// is resolved again from the start.
func (r *resolver) lookup(q dns.Question, now time.Time) (result, bool) {
	res := result{outcome: outcomeAnswered}
	name := q.Name
	for range restartsMax + 1 {
		if e, ttl, ok := r.answers.get(keyOf(name, q.Qtype, q.Qclass), now); ok {
			res.answer = append(res.answer, withTTL(e.rrs, ttl)...)
			res.rcode = dns.RcodeSuccess
			return res, true
		}
		if q.Qtype != dns.TypeCNAME {
			if e, ttl, ok := r.answers.get(keyOf(name, dns.TypeCNAME, q.Qclass), now); ok {
				target := cnameTarget(e.rrs)
				if target == "" {
					return result{}, false
				}
				res.answer = append(res.answer, withTTL(e.rrs, ttl)...)
				name = target
				continue
			}
		}
		if e, ttl, ok := r.negatives.get(keyOf(name, typeNameError, q.Qclass), now); ok {
			res.rcode = dns.RcodeNameError
			res.authority = withTTL(e.rrs, ttl)
			return res, true
		}
		if e, ttl, ok := r.negatives.get(keyOf(name, q.Qtype, q.Qclass), now); ok {
			res.rcode = dns.RcodeSuccess
			res.authority = withTTL(e.rrs, ttl)
			return res, true
		}
		return result{}, false
	}
	return result{}, false
}

// cnameTarget is the target of a CNAME RRset, which holds one record
// (RFC 2181 Section 10.1).
func cnameTarget(rrs []dns.RR) string {
	for _, rr := range rrs {
		if c, ok := rr.(*dns.CNAME); ok {
			return c.Target
		}
	}
	return ""
}
