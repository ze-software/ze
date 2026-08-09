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

// CachedEntry is one resolved prefix set, keyed by Name.
// Prefixes serialize to JSON as strings via netip.Prefix's TextMarshaler.
type CachedEntry struct {
	Name        string         `json:"name"`
	ASSet       string         `json:"as-set"`
	IPv4        []netip.Prefix `json:"ipv4"`
	IPv6        []netip.Prefix `json:"ipv6"`
	RefreshedAt time.Time      `json:"refreshed-at"`
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
	irrClient *irr.IRR
	pdb       *peeringdb.PeeringDB
	path      string

	fileMu  sync.Mutex // serializes in-process open->write->flush on the shared zefs file
	mu      sync.RWMutex
	entries map[string]*CachedEntry
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
// On a lookup error the returned entry is non-nil and carries the resolved
// AS-SET (with no prefixes) plus the error, and the cached data for name is
// left untouched. On success the in-memory cache and the zefs file are updated.
func (s *PrefixStore) Refresh(ctx context.Context, name, asSet string) (*CachedEntry, error) {
	entry, err := s.resolve(ctx, name, asSet)
	if err != nil {
		return entry, err
	}
	s.mu.Lock()
	s.entries[name] = entry
	s.mu.Unlock()
	s.persist([]*CachedEntry{entry})
	return entry, nil
}

// resolve performs AS-SET resolution and the IRR lookup without mutating state.
func (s *PrefixStore) resolve(ctx context.Context, name, asSet string) (*CachedEntry, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	effective := asSet
	if effective == "" {
		if asn, ok := parseBareASN(name); ok {
			effective = s.discoverASSet(ctx, asn)
		} else {
			effective = name
		}
	}

	pl, err := s.irrClient.LookupPrefixes(ctx, effective)
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
func (s *PrefixStore) discoverASSet(ctx context.Context, asn uint32) string {
	if s.pdb != nil {
		if sets, err := s.pdb.LookupASSet(ctx, asn); err == nil && len(sets) > 0 {
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
