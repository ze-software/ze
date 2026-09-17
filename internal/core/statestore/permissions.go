// Design: docs/architecture/api/process-protocol.md -- plugin state ownership.
package statestore

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"sync"

	"github.com/ze-software/ze/pkg/zefs"
)

// State grants come from compiled plugin registration, never from a wire claim.
var grants = struct {
	sync.RWMutex
	owners map[string][]stateGrant
}{owners: make(map[string][]stateGrant)}

type stateGrant struct {
	pattern string
	prefix  string
	match   *regexp.Regexp
}

// RegisterPluginKeys grants owner access to registered runtime keys. Call from
// the owning plugin's registration. Template parameters match one path segment.
// Safe for concurrent use; duplicate grants are harmless.
func RegisterPluginKeys(owner string, keys ...zefs.KeyEntry) {
	grants.Lock()
	defer grants.Unlock()
	if owner == "" {
		panic("BUG: state key grant has no plugin owner")
	}
	for _, key := range keys {
		if !strings.HasPrefix(key.Pattern, "meta/") {
			panic("BUG: state key grant must name a metadata key")
		}
		registered := false
		for _, entry := range zefs.AllEntries() {
			if entry.Pattern == key.Pattern {
				registered = true
				break
			}
		}
		if !registered {
			panic("BUG: state key grant is unregistered")
		}
		duplicate := false
		for _, grant := range grants.owners[owner] {
			if grant.pattern == key.Pattern {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		prefix, _, _ := strings.Cut(key.Pattern, "{")
		var expression strings.Builder
		expression.WriteByte('^')
		rest := key.Pattern
		for {
			before, after, found := strings.Cut(rest, "{")
			expression.WriteString(regexp.QuoteMeta(before))
			if !found {
				break
			}
			_, suffix, closed := strings.Cut(after, "}")
			if !closed {
				panic("BUG: invalid state key template")
			}
			expression.WriteString("[^/]+")
			rest = suffix
		}
		expression.WriteByte('$')
		grants.owners[owner] = append(grants.owners[owner], stateGrant{
			pattern: key.Pattern, prefix: prefix, match: regexp.MustCompile(expression.String()),
		})
	}
}

// PluginKeyAllowed reports whether the registered owner may reach this key.
func PluginKeyAllowed(owner, key string) bool {
	if !fs.ValidPath(key) {
		return false
	}
	grants.RLock()
	defer grants.RUnlock()
	for _, grant := range grants.owners[owner] {
		if grant.match.MatchString(key) {
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
		if PluginKeyAllowed(owner, key) {
			owned = append(owned, key)
		}
	}
	return owned, nil
}
