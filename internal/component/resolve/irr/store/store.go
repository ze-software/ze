// Design: docs/architecture/resolve.md -- shared IRR prefix resolution + persistence
//
// Package store provides PrefixStore, a shared cache of IRR-resolved prefix
// lists keyed by name (an ASN like "AS13335" or an AS-SET like "AS-CLOUDFLARE").
// It resolves prefixes via the IRR whois client, discovers AS-SETs for bare
// ASNs via PeeringDB, and persists each entry to zefs under meta/irr/{name}.
//
// It lives in a subpackage of resolve/irr (not package irr) so it can import
// resolve/peeringdb directly: peeringdb imports resolve/irr, and irr never
// imports this store, so store -> peeringdb -> irr is acyclic.
//
// Consumers (BGP filter_irr, the upcoming firewall-irr) are process-isolated
// plugins; they do NOT share a PrefixStore instance. Each builds its own and
// they share cached data through the zefs file on disk. In-process writers are
// serialized by fileMu: each persist opens the file, flushes it atomically
// (zefs pwrites in place for small updates, full-rewrites with atomic rename on
// growth), and closes it, so concurrent in-process refreshes cannot lose
// each other's keys. NOTE: zefs's Lock is an in-process mutex, not a file lock,
// so two PROCESSES writing the same store file would clobber each other on
// flush -- a single writer process per store file is required until zefs gains
// a cross-process lock (tracked for the firewall-irr consumer).
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

var logger = slogutil.LazyLogger("resolve.irr.store")

var (
	errEmptyName   = errors.New("irr/store: empty entry name")
	errBadPathName = errors.New("irr/store: name is not a valid zefs path segment")
)

// ErrNoPrefixes reports a refresh that learned nothing: the IRR answered, and
// the answer carried no prefixes for either family. The previously cached
// prefixes are kept and stay enforced, and the entry returned alongside this
// error carries them.
//
// An empty answer is never data. A consumer that replaced its prefix list with
// one has no filter left: an interface binding drops every packet arriving on
// the port, and a BGP filter rejects every UPDATE from the peer. Purge is the
// deliberate way to remove prefixes.
var ErrNoPrefixes = errors.New("irr/store: IRR returned no prefixes")

// CachedEntry is one resolved prefix set, keyed by Name.
// Prefixes serialize to JSON as strings via netip.Prefix's TextMarshaler.
type CachedEntry struct {
	Name  string         `json:"name"`
	ASSet string         `json:"as-set"`
	IPv4  []netip.Prefix `json:"ipv4"`
	IPv6  []netip.Prefix `json:"ipv6"`
	// RefreshedAt dates the OLDEST prefixes the entry carries. The two families
	// are learned by two queries and one of them can answer nothing (Refresh),
	// so the entry keeps the older date: it is the age an operator needs, and
	// the newer one would understate how long enforcement has run on
	// unconfirmed data.
	RefreshedAt time.Time `json:"refreshed-at"`
	// StaleSince is when the first refresh since RefreshedAt learned nothing.
	// Zero means the prefixes above are what the IRR last answered. Non-zero
	// means they are last-known-good data still being enforced, and the gap
	// between the two timestamps is how long that has been true.
	StaleSince time.Time `json:"stale-since,omitzero"`
}

// Stale reports whether the entry's prefixes are last-known-good data kept
// after a refresh learned nothing, rather than what the IRR last answered.
func (e *CachedEntry) Stale() bool {
	return !e.StaleSince.IsZero()
}

// PrefixList returns the entry's prefixes as an irr.PrefixList.
func (e *CachedEntry) PrefixList() irr.PrefixList {
	return irr.PrefixList{IPv4: e.IPv4, IPv6: e.IPv6}
}

// PrefixStore resolves and caches IRR prefix lists, persisting each entry to a
// zefs file. The IRR client is required; the PeeringDB client may be nil
// (AS-SET discovery is then skipped, falling back to the literal "AS<asn>"
// name). An empty path disables persistence (in-memory only).
type PrefixStore struct {
	path string

	fileMu sync.Mutex // serializes in-process open->write->flush on the shared zefs file

	// mu guards the cache and the two clients. The clients change when a config
	// reload moves the IRR server or the PeeringDB URL (UseClients), and the
	// cache outlives that move.
	mu        sync.RWMutex
	irrClient *irr.IRR
	pdb       *peeringdb.PeeringDB
	entries   map[string]*CachedEntry
}

// New creates a PrefixStore. irrClient must be non-nil.
func New(irrClient *irr.IRR, pdb *peeringdb.PeeringDB, path string) *PrefixStore {
	return &PrefixStore{
		irrClient: irrClient,
		pdb:       pdb,
		path:      path,
		entries:   make(map[string]*CachedEntry),
	}
}

// UseClients points the store at new resolvers and keeps every cached entry.
//
// A config reload can move the IRR server or the PeeringDB URL. The prefixes
// already resolved outlive that move, because they are what the consumer
// enforces until a refresh replaces them: a caller that built a second store
// for the new address would answer every Get with nil, and the rules naming
// those prefixes would filter nothing until the next fetch.
//
// irrClient MUST be non-nil. pdb MAY be nil, and AS-SET discovery is then
// skipped. Safe for concurrent use.
func (s *PrefixStore) UseClients(irrClient *irr.IRR, pdb *peeringdb.PeeringDB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.irrClient = irrClient
	s.pdb = pdb
}

// clients returns the resolvers to query. A lookup reads them once here and
// then runs without the lock: a reload MUST NOT wait behind a whois query, and
// one lookup MUST NOT change server halfway through.
func (s *PrefixStore) clients() (irrClient *irr.IRR, pdb *peeringdb.PeeringDB) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.irrClient, s.pdb
}

// Get returns the cached entry for name, or nil if absent. No network access.
func (s *PrefixStore) Get(name string) *CachedEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries[name]
}

// Put seeds a cache entry without network access or persistence.
// Intended for tests and programmatic pre-population.
func (s *PrefixStore) Put(name string, ipv4, ipv6 []netip.Prefix) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[name] = &CachedEntry{Name: name, IPv4: ipv4, IPv6: ipv6}
}

// Refresh resolves prefixes for name and persists the result.
//
// name is the identity and the zefs key (e.g. "AS13335" or "AS-CLOUDFLARE").
// asSet, when non-empty, is the AS-SET to query. When asSet is empty, the store
// queries name directly if it is an AS-SET, or discovers the AS-SET via
// PeeringDB if name is a bare ASN (falling back to the literal "AS<asn>" name).
//
// A refresh that does not learn prefixes never replaces what is cached. The
// previously resolved prefixes stay in memory and on disk and stay enforced,
// and the returned entry carries them with StaleSince set. Two cases reach it:
// a lookup error, which is returned unchanged, and a lookup that succeeded and
// carried no prefixes for either family, which returns ErrNoPrefixes.
//
// The decision is made per family, because each family is queried separately
// and each one is enforced separately (commit). An answer carrying one family
// and nothing for the other keeps what is cached for the family that answered
// nothing, marks the entry stale, and reports no error: it did learn prefixes.
//
// On success the in-memory cache and the zefs file are updated, and StaleSince
// is cleared when both families answered.
func (s *PrefixStore) Refresh(ctx context.Context, name, asSet string) (*CachedEntry, error) {
	entry, err := s.resolve(ctx, name, asSet)
	if err != nil {
		return s.markStale(name, entry), err
	}
	if entry.PrefixList().Empty() {
		return s.markStale(name, entry), fmt.Errorf("%w for %s (as-set %s)", ErrNoPrefixes, name, entry.ASSet)
	}
	return s.commit(name, entry), nil
}

// commit installs a resolved entry, keeping the cached prefixes of every family
// the answer carried nothing for, and returns what is now enforced.
//
// The last-known-good decision is per family because the risk is per family. An
// IRR answers each family with its own query, and a family the server does not
// hold reads exactly like a family with no route objects: both are "D", and
// lookupFamilyPrefixes returns no prefixes and no error for either
// (internal/component/resolve/irr/client.go). A wholesale replace therefore
// drops a family on the strength of an answer that cannot tell an outage from
// an AS-SET with no IPv6, and the consumer that enforces it emits one accept
// term per family that has prefixes and closes the interface with a drop term
// naming no family: every packet of the dropped family is then dropped
// (internal/component/firewall/plugins/irr/sets.go, buildIfaceTables).
//
// Removing prefixes stays an operator action, Purge, for the same reason it is
// one when both families answer nothing.
func (s *PrefixStore) commit(name string, fresh *CachedEntry) *CachedEntry {
	keptV4, keptV6 := false, false

	s.mu.Lock()
	if cached := s.entries[name]; cached != nil {
		fresh.IPv4, keptV4 = enforcedFamily(fresh.IPv4, cached.IPv4)
		fresh.IPv6, keptV6 = enforcedFamily(fresh.IPv6, cached.IPv6)
		if keptV4 || keptV6 {
			// The kept prefixes are the oldest data the entry now carries, so
			// they date it. StaleSince keeps the value it had: a family that
			// has been missing for a week must not read as stale since this
			// tick.
			fresh.RefreshedAt = cached.RefreshedAt
			fresh.StaleSince = cached.StaleSince
			if fresh.StaleSince.IsZero() {
				fresh.StaleSince = time.Now()
			}
		}
	}
	s.entries[name] = fresh
	s.mu.Unlock()

	if keptV4 {
		logger().Warn("irr/store: refresh learned no IPv4 prefixes, keeping the cached ones",
			"name", name, "kept-prefixes", len(fresh.IPv4))
	}
	if keptV6 {
		logger().Warn("irr/store: refresh learned no IPv6 prefixes, keeping the cached ones",
			"name", name, "kept-prefixes", len(fresh.IPv6))
	}

	s.persist([]*CachedEntry{fresh})
	return fresh
}

// enforcedFamily answers with the prefixes one family must enforce after a
// refresh, and whether they are the cached ones rather than the answered ones.
// An answer that carried nothing for the family keeps what was cached, because
// nothing distinguishes an AS-SET with no route objects in that family from a
// server that has stopped answering for it (see commit).
func enforcedFamily(answered, cached []netip.Prefix) (prefixes []netip.Prefix, kept bool) {
	if len(answered) > 0 {
		return answered, false
	}
	if len(cached) == 0 {
		return answered, false
	}
	return cached, true
}

// markStale records a refresh that did not learn prefixes for name. The cached
// prefixes stay in memory and on disk, and StaleSince dates the first refresh
// since they were learned that came back with nothing, so an operator can see
// how long enforcement has run on data nobody has confirmed.
//
// When nothing was cached, nothing is written: an entry that exists and holds
// no prefixes reads to every consumer as an answer, and a zero value must never
// look like a valid one (ai/rules/evidence.md). fresh is returned instead, so a
// caller still sees the AS-SET the name resolved to.
func (s *PrefixStore) markStale(name string, fresh *CachedEntry) *CachedEntry {
	if fresh == nil {
		return nil // the name never reached a lookup (invalid name)
	}

	s.mu.Lock()
	kept := s.entries[name]
	if kept == nil {
		s.mu.Unlock()
		// Nothing was learned, so RefreshedAt must not date prefixes that do
		// not exist. The caller gets the resolved AS-SET and a stale marker.
		fresh.RefreshedAt = time.Time{}
		fresh.StaleSince = time.Now()
		return fresh
	}
	updated := *kept
	changed := false
	if fresh.ASSet != "" && fresh.ASSet != updated.ASSet {
		updated.ASSet = fresh.ASSet
		changed = true
	}
	if updated.StaleSince.IsZero() {
		updated.StaleSince = time.Now()
		changed = true
	}
	if !changed {
		s.mu.Unlock()
		return kept // already recorded; a long outage must not rewrite zefs per tick
	}
	s.entries[name] = &updated
	s.mu.Unlock()

	s.persist([]*CachedEntry{&updated})
	return &updated
}

// Purge removes name from the in-memory cache and from the persisted cache, and
// reports whether an entry was there to remove.
//
// It is the deliberate exit from last-known-good. A refresh that learns nothing
// keeps the previous prefixes, so an AS-SET that was deregistered upstream stops
// being enforced when an operator purges it, never because a server had a bad
// minute.
func (s *PrefixStore) Purge(name string) bool {
	if validateName(name) != nil {
		// The store can hold no such name, and zefs key substitution panics on
		// some of them. Nothing to remove, and nothing to reach the key with.
		return false
	}
	s.mu.Lock()
	_, found := s.entries[name]
	delete(s.entries, name)
	s.mu.Unlock()

	if found {
		s.removePersisted(name)
	}
	return found
}

// removePersisted drops name's key from the zefs file. It is a no-op when
// persistence is disabled or the file does not exist.
func (s *PrefixStore) removePersisted(name string) {
	if s.path == "" {
		return
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	bs, err := openExisting(s.path)
	if err != nil {
		return // no file: the in-memory delete is the whole job
	}
	defer func() { _ = bs.Close() }()

	wl, err := bs.Lock()
	if err != nil {
		return
	}
	key := zefs.KeyIRRPrefixCache.Key(name)
	if wl.Has(key) {
		if rmErr := wl.Remove(key); rmErr != nil {
			logger().Warn("irr/store: purge removal failed", "name", name, "error", rmErr)
		}
	}
	if rErr := wl.Release(); rErr != nil {
		logger().Warn("irr/store: purge lock release failed", "name", name, "error", rErr)
	}
}

// resolve performs AS-SET resolution and the IRR lookup without mutating state.
//
// The lookup goes through RefreshPrefixes, which always queries the server. A
// refresh answered from the client's 1h cache would stamp a new RefreshedAt on
// data nobody re-read, and "update firewall irr as-set X" exists to reach the
// server. This store is the durable cache, in memory and in zefs.
func (s *PrefixStore) resolve(ctx context.Context, name, asSet string) (*CachedEntry, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	irrClient, pdb := s.clients()

	effective := asSet
	if effective == "" {
		if asn, ok := parseBareASN(name); ok {
			effective = discoverASSet(ctx, pdb, asn)
		} else {
			effective = name
		}
	}

	pl, err := irrClient.RefreshPrefixes(ctx, effective)
	if err != nil {
		return &CachedEntry{Name: name, ASSet: effective}, err
	}
	return &CachedEntry{
		Name:        name,
		ASSet:       effective,
		IPv4:        pl.IPv4,
		IPv6:        pl.IPv6,
		RefreshedAt: time.Now(),
	}, nil
}

// discoverASSet returns the AS-SET for a bare ASN via PeeringDB, falling back to
// the literal "AS<asn>" name when PeeringDB is unavailable or has no AS-SET.
// pdb is passed in rather than read from the store, so the whole lookup runs on
// the client resolve took, and no state is read outside the lock.
func discoverASSet(ctx context.Context, pdb *peeringdb.PeeringDB, asn uint32) string {
	if pdb != nil {
		if sets, err := pdb.LookupASSet(ctx, asn); err == nil && len(sets) > 0 {
			return sets[0]
		}
	}
	return asnName(asn)
}

// Open loads persisted entries into memory, first migrating any legacy
// single-blob cache (meta/bgp/irr-cache) into per-entry keys. It is safe to
// call when the zefs file does not exist yet (no-op). An empty path disables
// persistence entirely.
//
// The common path is read-only (no write lock): reads coordinate through the
// store's own RWMutex. A write lock is taken only when a legacy blob is
// actually present and needs migrating, so reconfigures do not contend for the
// shared store's write lock.
func (s *PrefixStore) Open() error {
	if s.path == "" {
		return nil
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	bs, err := openExisting(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			// File exists but could not be opened (corrupt/locked): surface it
			// rather than silently loading an empty cache.
			logger().Warn("irr/store: cache not loaded", "path", s.path, "error", err)
		}
		return nil
	}
	defer func() { _ = bs.Close() }()

	if _, rerr := bs.ReadFile(zefs.KeyIRRCache.Pattern); rerr == nil {
		s.migrate(bs)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	dir := zefs.KeyIRRPrefixCache.Dir()
	for _, key := range bs.List(dir) {
		data, readErr := bs.ReadFile(key)
		if readErr != nil {
			continue
		}
		var e CachedEntry
		if json.Unmarshal(data, &e) != nil || e.Name == "" {
			continue
		}
		// Trust the on-disk key segment as the identity, not the blob's
		// self-reported Name: a corrupt or tampered entry must not land in
		// another name's slot (the store file is shared across consumers).
		if len(key) <= len(dir)+1 || key[len(dir)+1:] != e.Name {
			logger().Warn("irr/store: entry name does not match its key; skipping", "key", key, "name", e.Name)
			continue
		}
		entry := e
		s.entries[e.Name] = &entry
	}
	return nil
}

// legacyEntry is one element of the old single-blob cache (keyed by ASN).
type legacyEntry struct {
	ASN   uint32   `json:"asn"`
	ASSet string   `json:"as-set"`
	IPv4  []string `json:"ipv4"`
	IPv6  []string `json:"ipv6"`
}

// migrate converts the legacy single-blob cache into per-entry keys, then
// removes the legacy key. Each entry is keyed by "AS<asn>" (the legacy
// identity). Existing per-entry keys are never clobbered (newer data wins).
// All writes and the removal happen under a single write lock, so the flush on
// Release is atomic: the legacy key disappears only once every new key is
// written.
func (s *PrefixStore) migrate(bs *zefs.BlobStore) {
	wl, err := bs.Lock()
	if err != nil {
		return
	}
	defer func() {
		if rErr := wl.Release(); rErr != nil {
			logger().Warn("irr/store: migrate release failed", "error", rErr)
		}
	}()

	data, err := wl.ReadFile(zefs.KeyIRRCache.Pattern)
	if err != nil {
		return // raced away
	}
	var old []legacyEntry
	if json.Unmarshal(data, &old) != nil {
		if rmErr := wl.Remove(zefs.KeyIRRCache.Pattern); rmErr != nil {
			logger().Warn("irr/store: drop corrupt legacy cache failed", "error", rmErr)
		}
		return
	}
	for _, c := range old {
		name := asnName(c.ASN)
		key := zefs.KeyIRRPrefixCache.Key(name)
		if wl.Has(key) {
			continue // newer per-entry data already present
		}
		blob, marshalErr := json.Marshal(&CachedEntry{
			Name:  name,
			ASSet: c.ASSet,
			IPv4:  parsePrefixes(c.IPv4),
			IPv6:  parsePrefixes(c.IPv6),
		})
		if marshalErr != nil {
			continue
		}
		if wErr := wl.WriteFile(key, blob, 0); wErr != nil {
			logger().Warn("irr/store: migrate write failed", "key", key, "error", wErr)
		}
	}
	if rmErr := wl.Remove(zefs.KeyIRRCache.Pattern); rmErr != nil {
		logger().Warn("irr/store: legacy cache removal failed", "error", rmErr)
	}
}

// persist writes the given entries to zefs under a single write lock. It is a
// no-op when persistence is disabled or the zefs file does not exist.
func (s *PrefixStore) persist(entries []*CachedEntry) {
	if s.path == "" || len(entries) == 0 {
		return
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	bs, err := openExisting(s.path)
	if err != nil {
		return // no file: in-memory cache is still valid
	}
	defer func() { _ = bs.Close() }()

	wl, err := bs.Lock()
	if err != nil {
		return
	}
	for _, e := range entries {
		blob, marshalErr := json.Marshal(e)
		if marshalErr != nil {
			continue
		}
		if wErr := wl.WriteFile(zefs.KeyIRRPrefixCache.Key(e.Name), blob, 0); wErr != nil {
			logger().Warn("irr/store: persist write failed", "name", e.Name, "error", wErr)
		}
	}
	if rErr := wl.Release(); rErr != nil {
		logger().Warn("irr/store: write lock release failed", "error", rErr)
	}
}

// openExisting opens the zefs store only if the file exists.
func openExisting(path string) (*zefs.BlobStore, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return zefs.Open(path)
}

func parsePrefixes(ss []string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(ss))
	for _, s := range ss {
		if p, err := netip.ParsePrefix(s); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// asnName renders an ASN as its canonical "AS<n>" name.
func asnName(asn uint32) string {
	var tb textbuf.Buffer
	return tb.Str("AS").Uint32(asn).String()
}

// parseBareASN reports whether name is a bare ASN ("13335" or "AS13335"),
// returning the ASN. AS-SET names ("AS-CLOUDFLARE", "RIPE::AS-FOO") are not
// bare ASNs.
func parseBareASN(name string) (uint32, bool) {
	num := name
	if len(name) >= 2 && (name[0] == 'A' || name[0] == 'a') && (name[1] == 'S' || name[1] == 's') {
		num = name[2:]
	}
	if num == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(num, 10, 32)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint32(n), true
}

// validateName rejects names that are empty, contain characters invalid for an
// AS-SET/ASN reference, or contain ".." (which would panic zefs key
// substitution).
func validateName(name string) error {
	if name == "" {
		return errEmptyName
	}
	if err := irr.ValidateASSetName(name); err != nil {
		return err
	}
	// "." and ".." pass ValidateASSetName (it allows '.') but are invalid zefs
	// path segments: keys "meta/irr/." / "meta/irr/.." fail fs.ValidPath at
	// decode and would make the whole (shared) store file unreadable, and ".."
	// also panics KeyEntry.Key(). Reject them before they reach the key.
	if name == "." || strings.Contains(name, "..") {
		return errBadPathName
	}
	return nil
}
