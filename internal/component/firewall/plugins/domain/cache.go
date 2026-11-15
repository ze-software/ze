// Design: docs/architecture/firewall/firewall-domain-group.md -- persisted last-good address cache
//
// The cache is what makes AC-7 work: a box that reboots with its upstream DNS
// down programs its firewall sets from here, so filtering resumes without
// waiting for a name to resolve. It is also what verify reads, so a commit
// naming a group Ze has never resolved is refused at the terminal rather than
// discovered later as a table held back.
//
// The daemon's owned store holds this bounded per-group state. The append-only
// change log has a separate file; changelog.go describes its lifetime.

package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// resolvedName is the last good answer for one DNS name, one family. Addresses
// are stored as strings because that is what reaches nftables and what the
// change log records; they are parsed on the way IN, so nothing invalid is
// ever written (addressesFromRecords).
type resolvedName struct {
	Addresses  []string  `json:"addresses"`
	ResolvedAt time.Time `json:"resolved-at"`
	// Status is what the last recorded answer said about the name, in the
	// resolver's spelling.
	Status string `json:"status,omitzero"`
	// FailingSince is when this name and family started failing to resolve,
	// and is zero while it is answering.
	//
	// It exists because the addresses alone cannot say it: a name whose server
	// has been down for a week holds exactly the addresses it held before, and
	// the firewall is enforcing them. It changes only when the failing state
	// FLIPS, once at the start of an outage and once at its end, so an outage
	// costs one write however long it lasts.
	FailingSince time.Time `json:"failing-since,omitzero"`
	// ExpiresAt is when this answer stops being valid: the answer's arrival
	// plus max(TTL, ttl-floor), capped at one day (refreshInterval). It is the
	// time the schedule asks again, and a start arms the unit at it (reset), so
	// a reboot does not ask upstream for a name whose answer is still valid.
	//
	// Every authoritative answer rewrites it, the ones that confirm the same
	// addresses included (Thomas, spec-fetched-data-survives-restart). It is
	// written in UTC whole seconds, so a confirming answer writes a value the
	// same size as the one it replaces. Zero is an answer stored before the
	// field existed, and reads as expired.
	ExpiresAt time.Time `json:"expires-at,omitzero"`
}

// failing reports whether this name and family is currently unable to resolve.
// A caller MUST use this rather than testing FailingSince against the zero
// time, so the meaning of the field stays in one place.
func (r resolvedName) failing() bool { return !r.FailingSince.IsZero() }

// cacheKeys is fulfilled by owned storage and by the plugin state RPC adapter.
type cacheKeys interface {
	ReadKey(string) ([]byte, error)
	WriteKey(string, []byte) error
	RemoveKey(string) error
}

// store holds last-good answers. It borrows persistence; the caller MUST keep
// that owner alive until the cache's refresh worker stops. Safe for concurrent use.
type store struct {
	mu      sync.RWMutex
	entries map[nameKey]resolvedName
	// unreadable holds every unit whose stored answer exists and could not be
	// read or decoded at the last open. Each reads as unresolved, and the
	// doctor check names it (codeDomainGroupUnreadableData).
	unreadable []nameKey
	// unsaved holds every unit whose last answer could not be written (put).
	// The answer is not in memory either, because put keeps memory and the
	// store equal, so the unit enforces what it held before. A later write of
	// the unit clears it. It lives in this process only, so the plugin's
	// runtime doctor check names it (codeDomainGroupUnsavedAnswer).
	unsaved map[nameKey]unsavedAnswer
	keys    cacheKeys
}

// unsavedAnswer is one answer put could not write: why, and when.
type unsavedAnswer struct {
	err error
	at  time.Time
}

// storedAnswerKey is the store key of one unit's answer.
func storedAnswerKey(key nameKey) string {
	return zefs.KeyFirewallDomainGroup.Key(key.group, key.name, key.family.label)
}

func newStore(keys cacheKeys) *store {
	return &store{
		keys:    keys,
		entries: make(map[nameKey]resolvedName),
		unsaved: make(map[nameKey]unsavedAnswer),
	}
}

// open reads every cached answer for the named groups into memory. It is
// called on each configure, not only once, so a group added by a commit picks
// up whatever an earlier run of the same box had already resolved.
//
// A group is loaded by NAME rather than by listing the store, because only a
// configured group can be programmed: an orphan key from a group an operator
// deleted must not put addresses back into the kernel.
//
// A nil persistence handle selects an in-memory startup cache. OnStarted binds
// the daemon state RPCs and reloads persisted answers before refresh begins.
func (s *store) open(groups []group) error {
	if s.keys == nil {
		return nil
	}

	entries := make(map[nameKey]resolvedName)
	configured := make(map[nameKey]bool)
	var unreadable []nameKey
	for _, g := range groups {
		for _, name := range g.Names {
			for _, fam := range families {
				unit := nameKey{group: g.Name, name: name, family: fam}
				configured[unit] = true
				data, readErr := s.keys.ReadKey(storedAnswerKey(unit))
				if readErr != nil {
					// A key this box never wrote is the normal state of a name
					// that has not resolved yet, and it says nothing. Any other
					// read failure is an entry that EXISTS and cannot be
					// recovered, and it is reported: skipping it silently would
					// leave the group short of addresses the box did learn,
					// which is the outcome this function is written to avoid.
					if !errors.Is(readErr, fs.ErrNotExist) {
						unreadable = append(unreadable, unit)
						logger().Warn("firewall-domain: cached addresses could not be read, the name reads as unresolved and is asked for at once",
							"group", g.Name, "name", name, "family", fam.label, "error", readErr,
							"code", codeDomainGroupUnreadableData, "fix", updateFix(g.Name))
					}
					continue
				}
				if len(data) == 0 {
					continue
				}
				var value resolvedName
				if decodeErr := json.Unmarshal(data, &value); decodeErr != nil {
					unreadable = append(unreadable, unit)
					logger().Warn("firewall-domain: cached addresses could not be decoded, the name reads as unresolved and is asked for at once",
						"group", g.Name, "name", name, "family", fam.label, "error", decodeErr,
						"code", codeDomainGroupUnreadableData, "fix", updateFix(g.Name))
					continue
				}
				entries[unit] = value
			}
		}
	}

	s.mu.Lock()
	s.entries = entries
	s.unreadable = unreadable
	// A failed write stays reported across a reopen: the store still holds the
	// older value, so the answer is still lost. A unit no longer configured
	// enforces nothing, so its record goes.
	for unit := range s.unsaved {
		if !configured[unit] {
			delete(s.unsaved, unit)
		}
	}
	s.mu.Unlock()
	return nil
}

// unsavedUnit is one answer unsavedFor reports: the unit and its failure.
type unsavedUnit struct {
	key     nameKey
	failure unsavedAnswer
}

// unsavedFor returns every unit whose last answer could not be written, in
// group, name and family order, for the runtime doctor check to name.
func (s *store) unsavedFor() []unsavedUnit {
	s.mu.RLock()
	out := make([]unsavedUnit, 0, len(s.unsaved))
	for key, failure := range s.unsaved {
		out = append(out, unsavedUnit{key: key, failure: failure})
	}
	s.mu.RUnlock()
	slices.SortFunc(out, func(a, b unsavedUnit) int {
		if c := strings.Compare(a.key.group, b.key.group); c != 0 {
			return c
		}
		if c := strings.Compare(a.key.name, b.key.name); c != 0 {
			return c
		}
		return strings.Compare(a.key.family.label, b.key.family.label)
	})
	return out
}

// expiry returns when the stored answer for one unit stops being valid. The
// bool is false when no answer is stored, and when the stored answer predates
// the expiry field: neither says the unit may wait, so a caller asks at once.
func (s *store) expiry(key nameKey) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[key]
	if !ok {
		return time.Time{}, false
	}
	if entry.ExpiresAt.IsZero() {
		return time.Time{}, false
	}
	return entry.ExpiresAt, true
}

// unreadableFor returns the units of one group whose stored answer could not
// be read or decoded at the last open, for the doctor check to name.
func (s *store) unreadableFor(groupName string) []nameKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []nameKey
	for _, key := range s.unreadable {
		if key.group == groupName {
			out = append(out, key)
		}
	}
	return out
}

// get returns the last good answer for one name and family.
func (s *store) get(key nameKey) (resolvedName, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.entries[key]
	return v, ok
}

// put records a new last-good answer, in memory and on disk. The disk write is
// what a restart reads, so a failure to write is returned rather than logged:
// a caller that recorded a change in memory alone would report success for
// state the next boot does not have. A failure leaves memory unchanged and is
// recorded for the doctor (unsavedFor) until a later put of the unit succeeds.
func (s *store) put(key nameKey, value resolvedName) error {
	err := s.write(key, value)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.unsaved[key] = unsavedAnswer{err: err, at: time.Now()}
		return err
	}
	s.entries[key] = value
	delete(s.unsaved, key)
	return nil
}

// write encodes one answer and writes it to the store.
func (s *store) write(key nameKey, value resolvedName) error {
	data, err := json.Marshal(&value)
	if err != nil {
		return fmt.Errorf("firewall domain-group: encode cache entry: %w", err)
	}
	if s.keys == nil {
		return errors.New("firewall domain-group: persistence unavailable")
	}
	if err := s.keys.WriteKey(storedAnswerKey(key), data); err != nil {
		return fmt.Errorf("firewall domain-group: write cache entry: %w", err)
	}
	return nil
}

// hasAddresses reports whether the group holds at least one address for at
// least one of its names. It is the question verify asks: a group with nothing
// cached cannot filter, so a commit naming it is refused (AC-6).
func (s *store) hasAddresses(g group) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, name := range g.Names {
		for _, fam := range families {
			if len(s.entries[nameKey{group: g.Name, name: name, family: fam}].Addresses) > 0 {
				return true
			}
		}
	}
	return false
}

// groupAddresses returns every cached address of the group in one family,
// deduplicated and sorted. Two names in a group resolving to the same address
// is ordinary, and a set holding the same element twice is refused by
// nftables, so the deduplication is required rather than tidy.
func (s *store) groupAddresses(groupName string, names []string, fam family) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]bool)
	var out []string
	for _, name := range names {
		for _, addr := range s.entries[nameKey{group: groupName, name: name, family: fam}].Addresses {
			if seen[addr] {
				continue
			}
			seen[addr] = true
			out = append(out, addr)
		}
	}
	slices.Sort(out)
	return out
}

// provenance maps each cached address of a group to the DNS name that supplied
// it. It is what the show enricher answers with (AC-8).
//
// An address two names both answer with reports the first name in sorted
// order, and the map cannot say otherwise: one address is one set element, and
// the element is what the operator is reading. The order is stable rather than
// arbitrary so the same ruleset renders the same way twice.
func (s *store) provenance(groupName string, names []string) map[string]string {
	sorted := make([]string, len(names))
	copy(sorted, names)
	slices.Sort(sorted)

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]string)
	for _, name := range sorted {
		for _, fam := range families {
			for _, addr := range s.entries[nameKey{group: groupName, name: name, family: fam}].Addresses {
				if _, taken := out[addr]; taken {
					continue
				}
				out[addr] = name
			}
		}
	}
	return out
}

// entriesFor returns every cached answer of one group, keyed by name and
// family, for the show command and the doctor check to report.
func (s *store) entriesFor(g group) map[nameKey]resolvedName {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[nameKey]resolvedName)
	for _, name := range g.Names {
		for _, fam := range families {
			key := nameKey{group: g.Name, name: name, family: fam}
			if v, ok := s.entries[key]; ok {
				out[key] = v
			}
		}
	}
	return out
}

// purge removes every cached answer of the group, from memory and from disk.
// It is the deliberate exit from last-known-good: a refresh that fails keeps
// the addresses it has, so an operator who knows a group is finished says so
// with `clear firewall domain-group`.
func (s *store) purge(g group) (removed int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys == nil {
		return 0, errors.New("firewall domain-group: persistence unavailable")
	}
	for _, name := range g.Names {
		for _, fam := range families {
			key := nameKey{group: g.Name, name: name, family: fam}
			// A cleared unit holds no answer to save, so a failed write of it
			// is no longer a loss to report.
			delete(s.unsaved, key)
			if _, ok := s.entries[key]; !ok {
				continue
			}
			if err := s.keys.RemoveKey(storedAnswerKey(key)); err != nil {
				return removed, err
			}
			delete(s.entries, key)
			removed++
		}
	}
	return removed, nil
}

// addressesFromRecords keeps the records that parse as an address of the
// wanted family, and caps how many one name may contribute.
//
// The parse is a boundary check, not tidiness: DNS is attacker-influenced
// input, and an address that reached the set builder unparsed would be handed
// to nftables as a literal. The cap bounds what one hijacked name can insert
// into a group a permit rule trusts; the caller logs when it bites, because a
// silently truncated permit list is a filter that does not do what its config
// says.
func addressesFromRecords(records []string, fam family, limit int) (addresses []string, truncated bool) {
	for _, record := range records {
		if len(addresses) >= limit {
			return addresses, true
		}
		addr, err := netip.ParseAddr(record)
		if err != nil {
			continue
		}
		if addr.Is4() != fam.isV4 {
			continue
		}
		addresses = append(addresses, addr.String())
	}
	slices.Sort(addresses)
	return addresses, false
}

// sameAddresses reports whether two answers hold the same addresses. Both
// sides are sorted where they are built, so this is an element-wise compare
// rather than a set compare, and a re-resolution that returned the same
// addresses in a different order still reads as unchanged (AC-2).
func sameAddresses(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
