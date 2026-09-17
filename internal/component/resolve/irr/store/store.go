// Design: docs/architecture/resolve.md -- shared IRR prefix resolution + persistence
//
// Package store provides PrefixStore, a shared cache of IRR-resolved prefix
// lists keyed by name (an ASN like "AS13335" or an AS-SET like "AS-CLOUDFLARE").
// It resolves prefixes via the IRR whois client, discovers AS-SETs for bare
// ASNs via PeeringDB, and persists each entry under meta/irr/{name}.
//
// It lives in a subpackage of resolve/irr (not package irr) so it can import
// resolve/peeringdb directly: peeringdb imports resolve/irr, and irr never
// imports this store, so store -> peeringdb -> irr is acyclic.
//
// The caller supplies the daemon's owned store or its plugin state RPC client.
// PrefixStore never opens a second writer and never closes the supplied store.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
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

// KeyStore is the raw-key contract implemented by managed storage and plugin
// state RPCs. The caller owns its lifetime.
type KeyStore interface {
	ReadKey(string) ([]byte, error)
	WriteKey(string, []byte) error
	RemoveKey(string) error
	ListKeys(string) ([]string, error)
}

// PrefixStore resolves and caches IRR prefix lists. A nil store explicitly
// selects in-memory operation. The caller MUST keep persistence alive until
// all refreshes finish. Safe for concurrent use.
type PrefixStore struct {
	persistence KeyStore

	fileMu sync.Mutex // Serializes cache persistence and legacy migration.

	// mu guards the cache and the two clients. The clients change when a config
	// reload moves the IRR server or the PeeringDB URL (UseClients), and the
	// cache outlives that move.
	mu        sync.RWMutex
	irrClient *irr.IRR
	pdb       *peeringdb.PeeringDB
	entries   map[string]*CachedEntry
}

// New creates a PrefixStore. irrClient must be non-nil.
func New(irrClient *irr.IRR, pdb *peeringdb.PeeringDB, persistence KeyStore) *PrefixStore {
	return &PrefixStore{
		irrClient:   irrClient,
		pdb:         pdb,
		persistence: persistence,
		entries:     make(map[string]*CachedEntry),
	}
}

// UsePersistence binds a runtime store after the plugin startup handshake.
// The caller MUST call this before starting refresh workers.
func (s *PrefixStore) UsePersistence(persistence KeyStore) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	s.persistence = persistence
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
// name is the identity and the storage key (e.g. "AS13335" or "AS-CLOUDFLARE").
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
// On success the in-memory cache is updated and persistence is attempted.
// StaleSince is cleared when both families answered.
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

// removePersisted drops a key through the caller's owned store.
func (s *PrefixStore) removePersisted(name string) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if s.persistence == nil {
		return
	}
	if err := s.persistence.RemoveKey(zefs.KeyIRRPrefixCache.Key(name)); err != nil {
		logger().Warn("irr/store: purge removal failed", "name", name, "error", err)
	}
}

// resolve performs AS-SET resolution and the IRR lookup without mutating state.
//
// The lookup goes through RefreshPrefixes, which always queries the server. A
// refresh answered from the client's 1h cache would stamp a new RefreshedAt on
// data nobody re-read, and "update firewall irr as-set X" exists to reach the
// server. This store keeps the cache in memory and persists through its owner.
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

// Open loads persisted entries and migrates the legacy cache when writable.
// A read-only diagnostic loads legacy values without changing its source.
func (s *PrefixStore) Open() error {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if s.persistence == nil {
		return nil
	}
	bs := s.persistence
	if err := s.migrate(bs); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	dir := zefs.KeyIRRPrefixCache.Dir()
	keys, err := bs.ListKeys(dir)
	if err != nil {
		return err
	}
	for _, key := range keys {
		data, readErr := bs.ReadKey(key)
		if readErr != nil {
			continue
		}
		var e CachedEntry
		if json.Unmarshal(data, &e) != nil || e.Name == "" {
			continue
		}
		// Trust the key segment as the identity rather than the value's Name:
		// a corrupt entry must not occupy another name's slot.
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

// migrate writes all legacy entries before removing their source. A failure
// keeps the source so a later attempt can resume without losing prefixes.
func (s *PrefixStore) migrate(bs KeyStore) error {
	data, err := bs.ReadKey(zefs.KeyIRRCache.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var old []legacyEntry
	if err := json.Unmarshal(data, &old); err != nil {
		return fmt.Errorf("irr/store: decode legacy cache: %w", err)
	}
	for _, c := range old {
		name := asnName(c.ASN)
		key := zefs.KeyIRRPrefixCache.Key(name)
		if _, err := bs.ReadKey(key); err == nil {
			continue // Newer per-entry data already exists.
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		blob, marshalErr := json.Marshal(&CachedEntry{
			Name:  name,
			ASSet: c.ASSet,
			IPv4:  parsePrefixes(c.IPv4),
			IPv6:  parsePrefixes(c.IPv6),
		})
		if marshalErr != nil {
			return marshalErr
		}
		if err := bs.WriteKey(key, blob); err != nil {
			if errors.Is(err, storage.ErrReadOnly) {
				var entry CachedEntry
				if err := json.Unmarshal(blob, &entry); err != nil {
					return err
				}
				s.mu.Lock()
				s.entries[name] = &entry
				s.mu.Unlock()
				continue
			}
			return err
		}
	}
	if err := bs.RemoveKey(zefs.KeyIRRCache.Pattern); err != nil {
		if !errors.Is(err, storage.ErrReadOnly) {
			return err
		}
	}
	return nil
}

// persist writes through the existing owner, never a second store handle.
func (s *PrefixStore) persist(entries []*CachedEntry) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if s.persistence == nil {
		return
	}
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			logger().Warn("irr/store: encode cache failed", "name", e.Name, "error", err)
			continue
		}
		if err := s.persistence.WriteKey(zefs.KeyIRRPrefixCache.Key(e.Name), data); err != nil {
			logger().Warn("irr/store: persist write failed", "name", e.Name, "error", err)
		}
	}
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
