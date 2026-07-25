// Design: docs/architecture/config/environment.md -- shared env catalog
// Overview: catalog_test.go -- unit tests
// Related: ../env/registry.go -- authoritative env entry source
// Related: ../slogutil/slogutil.go -- concrete log subsystem source

package envcatalog

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type CatalogEntry struct {
	Key         string
	Type        string
	Description string
}

// VisibleEntries returns all public env keys sorted by key, including
// concrete ze.log.<subsystem> rows expanded from slogutil.Subsystems().
// Private entries and angle-bracket template keys are excluded.
func VisibleEntries() []CatalogEntry {
	raw := env.Entries()
	subs := slogutil.Subsystems()
	entries := make([]CatalogEntry, 0, len(raw)+len(subs))

	var tb textbuf.Buffer

	for _, e := range raw {
		if strings.ContainsRune(e.Key, '<') {
			continue
		}
		entries = append(entries, CatalogEntry{
			Key:         e.Key,
			Type:        e.Type,
			Description: e.Description,
		})
	}

	for _, sub := range subs {
		key := tb.Reset().Str("ze.log.").Str(sub.Name).String()
		desc := sub.Description
		if desc == "" {
			desc = tb.Reset().Str("Log level for ").Str(sub.Name).String()
		}
		entries = append(entries, CatalogEntry{
			Key:         key,
			Type:        "string",
			Description: desc,
		})
	}

	slices.SortFunc(entries, func(a, b CatalogEntry) int {
		return strings.Compare(a.Key, b.Key)
	})
	return entries
}

// LookupLogSubsystem resolves a concrete ze.log.<subsystem> key to
// its SubsystemInfo. Returns false if the key does not start with
// "ze.log." or does not match a registered subsystem.
func LookupLogSubsystem(key string) (slogutil.SubsystemInfo, bool) {
	name, ok := strings.CutPrefix(key, "ze.log.")
	if !ok || name == "" {
		return slogutil.SubsystemInfo{}, false
	}
	for _, sub := range slogutil.Subsystems() {
		if sub.Name == name {
			return sub, true
		}
	}
	return slogutil.SubsystemInfo{}, false
}
