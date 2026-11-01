// Design: docs/architecture/diagnostics/debug-filtering.md -- structured debug state display
// Related: debug.go -- CLI handler, profile.go -- Profile data model
//
// The debug commands answer DATA (the answer types below), so `| json`, a
// program and the daemon route read fields. The text an operator has always
// read is the drawing registered for each envelope key (register.go), so the
// text route prints exactly what the commands printed when they wrote it
// themselves.

package debug

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The envelope keys of the debug answers, each with a registered drawing.
const (
	keyDebugChange   = "debug-change"
	keyDebugModules  = "debug-modules"
	keyDebugProfiles = "debug-profiles"
)

// The subjects a debugChange names. The set is closed: only this package
// builds a debugChange.
const (
	subjectModule  = "module"
	subjectTimeout = "timeout"
	subjectProfile = "profile"
	subjectState   = "state"
)

// debugChange is what one edit of the stored debug state changed.
type debugChange struct {
	Subject string `json:"subject"`
	Name    string `json:"name,omitempty"`
	Result  string `json:"result"`
	Minutes int    `json:"minutes,omitempty"`
}

// debugChangeAnswer is the answer of every command that edits the stored state.
type debugChangeAnswer struct {
	Change debugChange `json:"debug-change"`
}

// debugModulesAnswer is the answer of `show debug profile name <name>`: one row
// per module the profile enables.
type debugModulesAnswer struct {
	Modules []showEntry `json:"debug-modules"`
}

// debugProfilesAnswer is the answer of `show debug profile`: the stored
// profile names.
type debugProfilesAnswer struct {
	Profiles []string `json:"debug-profiles"`
}

var errUnknownSubject = errors.New("unknown debug change subject")

// drawDebugChange draws a debugChange as the one line the command printed.
func drawDebugChange(raw json.RawMessage) (string, error) {
	var change debugChange
	if err := json.Unmarshal(raw, &change); err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	switch change.Subject {
	case subjectModule:
		tb.Str("debug module ").Str(change.Name).Byte(' ').Str(change.Result)
	case subjectTimeout:
		if change.Minutes == 0 {
			tb.Str("debug timeout disabled")
		} else {
			tb.Str("debug timeout set to ").Int(int64(change.Minutes)).Str("m")
		}
	case subjectProfile:
		tb.Str("debug profile ").Quoted(change.Name).Byte(' ').Str(change.Result)
	case subjectState:
		tb.Str("debug state ").Str(change.Result)
	default:
		return "", fmt.Errorf("%w %q", errUnknownSubject, change.Subject)
	}
	return tb.Byte('\n').String(), nil
}

// drawDebugProfiles draws the profile names one per line, or says there are
// none.
func drawDebugProfiles(raw json.RawMessage) (string, error) {
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "no saved profiles\n", nil
	}
	var tb textbuf.Buffer
	for _, name := range names {
		tb.Str(name).Byte('\n')
	}
	return tb.String(), nil
}

// drawDebugModules draws the module rows as the aligned table the command
// printed, header included when there is no row.
func drawDebugModules(raw json.RawMessage) (string, error) {
	var entries []showEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return "", err
	}
	var out bytes.Buffer
	w := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	writeTableRow(w, "MODULE", "LEVEL", "FLAGS", "SCOPES")
	writeTableRow(w, "------", "-----", "-----", "------")
	for _, e := range entries {
		writeTableRow(w, e.Module, e.Level, e.Flags, e.Scopes)
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func writeTableRow(w *tabwriter.Writer, cols ...string) {
	var tb textbuf.Buffer
	w.Write(tb.Join(cols, "\t").Byte('\n').Bytes()) //nolint:errcheck // a bytes.Buffer write; Flush reports
}

// showEntry represents one row in the show debug output.
type showEntry struct {
	Module string `json:"module"`
	Level  string `json:"level"`
	Flags  string `json:"flags"`
	Scopes string `json:"scopes"`
}

// showEntries builds structured display entries from a profile,
// optionally filtered to a module subtree prefix.
func showEntries(p *Profile, subtree string) []showEntry {
	names := p.ModuleNames()
	entries := make([]showEntry, 0, len(names))

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

		entries = append(entries, showEntry{
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
