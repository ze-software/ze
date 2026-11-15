// Design: docs/architecture/api/process-protocol.md -- plugin state ownership.
package statestore

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/pkg/zefs"
)

// State grants come from compiled plugin registration, never from a wire claim.
var grants = struct {
	sync.RWMutex
	owners map[string][]stateGrant
}{owners: make(map[string][]stateGrant)}

// stateGrant admits the keys its entry matches (zefs.KeyEntry.Matches). A
// write grant also admits reads; a read grant admits get and list only.
type stateGrant struct {
	entry  zefs.KeyEntry
	prefix string
	write  bool
}

// RegisterPluginKeys grants owner read and write access to registered runtime
// keys. Call from the owning plugin's registration. Template parameters match
// one path segment. Safe for concurrent use; duplicate grants are harmless.
func RegisterPluginKeys(owner string, keys ...zefs.KeyEntry) {
	for _, key := range keys {
		registerGrant(owner, key, key, true)
	}
}

// RegisterPluginReadKeys grants reader read access to registered runtime keys
// another plugin writes: get and list succeed, put, remove and increment are
// refused. A read grant never names the reader as a writer (PluginKeyOwners).
// Call from the reading plugin's registration. Safe for concurrent use.
func RegisterPluginReadKeys(reader string, keys ...zefs.KeyEntry) {
	for _, key := range keys {
		registerGrant(reader, key, key, false)
	}
}

// RegisterPluginKeyInstance grants owner read and write access to ONE concrete
// key of a registered template, key.Key(params...), and to no other instance
// of it. A plugin whose key names the plugin itself (an IRR consumer's
// meta/irr-interest/{consumer}) takes this grant, so it cannot write the key
// of another plugin that shares the template. Safe for concurrent use.
func RegisterPluginKeyInstance(owner string, key zefs.KeyEntry, params ...string) {
	instance := zefs.KeyEntry{Pattern: key.Key(params...), Description: key.Description, ValueMax: key.ValueMax}
	registerGrant(owner, key, instance, true)
}

// registerGrant records one grant. registered is the zefs entry the grant
// derives from, which must exist; granted is what the grant matches.
func registerGrant(owner string, registered, granted zefs.KeyEntry, write bool) {
	grants.Lock()
	defer grants.Unlock()
	if owner == "" {
		panic("BUG: state key grant has no plugin owner")
	}
	if !strings.HasPrefix(granted.Pattern, "meta/") {
		panic("BUG: state key grant must name a metadata key")
	}
	known := false
	for _, entry := range zefs.AllEntries() {
		if entry.Pattern == registered.Pattern {
			known = true
			break
		}
	}
	if !known {
		panic("BUG: state key grant is unregistered")
	}
	for i, grant := range grants.owners[owner] {
		if grant.entry.Pattern != granted.Pattern {
			continue
		}
		// One grant per pattern: a write grant over a read grant widens it.
		if write {
			grants.owners[owner][i].write = true
		}
		return
	}
	prefix, _, _ := strings.Cut(granted.Pattern, "{")
	grants.owners[owner] = append(grants.owners[owner], stateGrant{entry: granted, prefix: prefix, write: write})
}

// PluginKeyReadable reports whether a grant lets the plugin read this key: a
// read grant or a write grant.
func PluginKeyReadable(owner, key string) bool {
	grants.RLock()
	defer grants.RUnlock()
	return grantAdmits(grants.owners[owner], key, false)
}

// PluginKeyWritable reports whether a write grant lets the plugin put, remove
// or increment this key. A read grant never does.
func PluginKeyWritable(owner, key string) bool {
	grants.RLock()
	defer grants.RUnlock()
	return grantAdmits(grants.owners[owner], key, true)
}

// PluginKeyOwners returns the plugin process names granted WRITE access to this
// key, sorted, in a slice the caller owns. A name is in the answer exactly when
// PluginKeyWritable answers true for it, because both judge a grant with
// grantAdmits; a plugin holding only a read grant is not an owner. The reader
// cannot fail: grants are compiled in by registration and nothing is read from
// the store, so an empty answer is the real answer that no plugin holds a write
// grant for the key, never a stand-in for an error. Safe for concurrent use.
func PluginKeyOwners(key string) []string {
	grants.RLock()
	defer grants.RUnlock()
	var owners []string
	for owner, owned := range grants.owners {
		if grantAdmits(owned, key, true) {
			owners = append(owners, owner)
		}
	}
	slices.Sort(owners)
	return owners
}

// grantAdmits reports whether one of an owner's grants matches key, and, when
// write is true, whether that grant is a write grant. It is the one matching
// rule for every reader of the grants. A key that is not a valid
// slash-separated path matches no grant, so a template parameter never admits
// a "." or ".." element. The caller MUST hold the grants read lock.
func grantAdmits(owned []stateGrant, key string, write bool) bool {
	if !fs.ValidPath(key) {
		return false
	}
	for _, grant := range owned {
		if write && !grant.write {
			continue
		}
		if grant.entry.Matches(key) {
			return true
		}
	}
	return false
}

// PluginList lists only keys the owner may read, even when prefix is a parent
// shared with another plugin. A request with no overlapping grant is refused.
func PluginList(owner, prefix string) ([]string, error) {
	grants.RLock()
	allowed := false
	for _, grant := range grants.owners[owner] {
		overlaps := strings.HasPrefix(grant.prefix, prefix) || strings.HasPrefix(prefix, grant.prefix)
		if overlaps {
			allowed = true
			break
		}
	}
	grants.RUnlock()
	if !allowed {
		return nil, fmt.Errorf("plugin %s does not own state prefix %s", owner, prefix)
	}
	keys, err := List(prefix)
	if err != nil {
		return nil, err
	}
	owned := keys[:0]
	for _, key := range keys {
		if PluginKeyReadable(owner, key) {
			owned = append(owned, key)
		}
	}
	return owned, nil
}
