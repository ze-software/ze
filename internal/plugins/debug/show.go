// Design: plan/learned/891-granular-debug.md -- structured debug state display
// Related: debug.go -- CLI handler, profile.go -- Profile data model

package debug

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ShowEntry represents one row in the show debug output.
type ShowEntry struct {
	Module string
	Level  string
	Flags  string
	Scopes string
}

// showEntries builds structured display entries from a profile,
// optionally filtered to a module subtree prefix.
func showEntries(p *Profile, subtree string) []ShowEntry {
	names := p.ModuleNames()
	entries := make([]ShowEntry, 0, len(names))

	var tb textbuf.Buffer
	subtreePrefix := ""
	if subtree != "" {
		subtreePrefix = tb.Str(subtree).Byte('.').String()
	}

	for _, name := range names {
		if subtree != "" && name != subtree && !strings.HasPrefix(name, subtreePrefix) {
			continue
		}

		entry := p.Module(name)
		if entry == nil {
			continue
		}

		entries = append(entries, ShowEntry{
			Module: name,
			Level:  entry.Level,
			Flags:  formatFlags(entry.Flags),
			Scopes: formatScopes(entry.Scopes),
		})
	}

	return entries
}

func formatFlags(flags []FlagEntry) string {
	if len(flags) == 0 {
		return ""
	}

	var tb textbuf.Buffer
	for i, f := range flags {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(f.Name)
	}
	return tb.String()
}

func formatScopes(scopes []ScopeEntry) string {
	if len(scopes) == 0 {
		return ""
	}

	var tb textbuf.Buffer
	for i, s := range scopes {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(s.Kind).Byte('=').Str(s.Value)
	}
	return tb.String()
}
