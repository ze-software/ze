// VALIDATES: every row of the rename map, against the registry le really
// composes. Each retired name runs as a registered new command (AC-3, AC-4,
// AC-5), no retired command stays in the manifest (AC-2), and the rows cannot
// shadow one another or a new name.
// PREVENTS: a row pointing at a command no family registers, which would
// strand every caller of the old name once its family moves. A retired name
// left listed beside its replacement. A row whose old words capture an
// invocation of a new command.
//
// This file is an external test package so it can link internal/le, the
// composition root, which imports leroot and every area. The package's own
// tests register probes and see no area.
//
// Until Phase 1b of plan/spec-le-subject-first-command-tree.md moves a family,
// its rows fail here BY NAME: the new command is not registered, and the old
// one is still in the manifest.
package leroot_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/le"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/leroot"
)

// commandWordsMax mirrors the dispatcher's bound: a command is at most two
// words after `le` (TestDispatchBoundsTheLookupAtTwoWords pins the original).
const commandWordsMax = 2

// streams runs fn with stdout and stderr redirected, and answers both. Each
// pipe is drained on its own goroutine, so a page longer than the pipe buffer
// cannot block the writer.
func streams(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	drain := func(target **os.File) (func() string, func()) {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatalf("open a pipe: %v", err)
		}
		saved := *target
		*target = write
		done := make(chan string, 1)
		go func() {
			var sb strings.Builder
			buf := make([]byte, 4096)
			for {
				n, readErr := read.Read(buf)
				sb.Write(buf[:n]) //nolint:errcheck // strings.Builder never fails
				if readErr != nil {
					break
				}
			}
			done <- sb.String()
		}()
		restore := func() { *target = saved }
		collect := func() string {
			write.Close() //nolint:errcheck // closing our own pipe end
			text := <-done
			read.Close() //nolint:errcheck // closing our own pipe end
			return text
		}
		return collect, restore
	}
	collectOut, restoreOut := drain(&os.Stdout)
	collectErr, restoreErr := drain(&os.Stderr)
	fn()
	restoreOut()
	restoreErr()
	return collectOut(), collectErr()
}

func TestEveryRetiredNameRunsItsNewCommand(t *testing.T) {
	for _, row := range leroot.Renames() {
		old := strings.Join(row.Old(), " ")
		fresh := strings.Join(row.New(), " ")
		t.Run(old, func(t *testing.T) {
			if leroot.LookupCommand(row.Command) == nil {
				t.Fatalf("row `le %s` -> `le %s`: the new command `le %s` is not registered", old, fresh, row.Command)
			}
			if row.Action != "" {
				list, declared := leroot.ActionsOf(row.Command)
				if declared && !slices.ContainsFunc(list.Actions, func(r leaction.Row) bool { return r.Verb == row.Action }) {
					t.Fatalf("row `le %s` -> `le %s`: `le %s` declares no action %q", old, fresh, row.Command, row.Action)
				}
			}

			// A trailing option asks for usage, which no handler answers, so
			// the probe runs no command's work (leroot.asksForUsage). It still
			// takes the whole dispatch path the old name takes.
			oldArgs := append(row.Old(), "--help")
			newArgs := append(row.New(), "--help")
			oldCode, newCode := 0, 0
			oldOut, oldErr := streams(t, func() { oldCode = leroot.Dispatch("le", oldArgs) })
			newOut, newErr := streams(t, func() { newCode = leroot.Dispatch("le", newArgs) })

			if oldCode != newCode {
				t.Errorf("row `le %s`: answered %d, want the %d of `le %s`", old, oldCode, newCode, fresh)
			}
			if oldOut != newOut {
				t.Errorf("row `le %s`: wrote\n%q\nto stdout, want what `le %s` wrote:\n%q", old, oldOut, fresh, newOut)
			}
			line := "warning: le " + old + " is renamed: run le " + fresh + "\n"
			if oldErr != line+newErr {
				t.Errorf("row `le %s`: wrote\n%q\nto stderr, want %q then what `le %s` wrote:\n%q", old, oldErr, line, fresh, newErr)
			}
		})
	}
}

func TestRetiredNamesAreNotInTheManifest(t *testing.T) {
	listed := make(map[string]bool, 128)
	for _, command := range leroot.Commands() {
		listed[command.Name] = true
	}

	reported := make(map[string]bool, 64)
	for _, row := range leroot.Renames() {
		if reported[row.Retired] {
			continue
		}
		reported[row.Retired] = true
		if listed[row.Retired] {
			t.Errorf("`le %s` is retired and still in the manifest; its replacement is `le %s`", row.Retired, row.Command)
		}
	}
}

func TestRenameMapRowsAreDisjoint(t *testing.T) {
	rows := leroot.Renames()
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		old := strings.Join(row.Old(), " ")
		if row.Retired == "" || row.Command == "" {
			t.Errorf("row %+v names no retired command or no new command", row)
			continue
		}
		if words := len(strings.Fields(row.Command)); words > commandWordsMax {
			t.Errorf("row `le %s`: the new command `le %s` has %d words, the dispatcher reads %d", old, row.Command, words, commandWordsMax)
		}
		if seen[old] {
			t.Errorf("two rows retire `le %s`", old)
		}
		seen[old] = true

		// An old word sequence that starts a new invocation would rewrite a
		// caller who already typed the new name.
		for _, other := range rows {
			fresh := other.New()
			if len(row.Old()) > len(fresh) {
				continue
			}
			if slices.Equal(fresh[:len(row.Old())], row.Old()) {
				t.Errorf("row `le %s` captures the new name `le %s`", old, strings.Join(fresh, " "))
			}
		}
	}

	names := make(map[string]bool, len(leroot.Retirements()))
	for _, retired := range leroot.Retirements() {
		if retired.Kind == leroot.RetiredKindUnspecified {
			t.Errorf("retired name %q declares no kind", retired.Old)
		}
		if retired.Replacement == "" {
			t.Errorf("retired name %q names no replacement", retired.Old)
		}
		if names[retired.Old] {
			t.Errorf("two rows retire %q", retired.Old)
		}
		names[retired.Old] = true
	}
}

// TestSpecIsANamespaceAndSpecSessionRunsItsMembers pins the one family whose
// retired command became a namespace rather than a command. `spec session`
// flattened into `spec`, so the bare `le spec` asks what the namespace holds
// (AC-31) and a bare `spec session`, which answered the claimed spec, runs
// `spec current`. It drives the real registry through Dispatch, because the
// generic row test above probes with `--help` and never runs a handler.
func TestSpecIsANamespaceAndSpecSessionRunsItsMembers(t *testing.T) {
	code := 0
	stdout, stderr := streams(t, func() { code = leroot.Dispatch("le", []string{"spec"}) })
	if code != 0 {
		t.Errorf("bare `le spec` answered %d, want 0: a namespace token is a question", code)
	}
	listing := stdout + stderr
	for _, member := range []string{"claim", "current", "release", "state", "review", "wip", "model", "status", "roadmap", "citation", "journal"} {
		if !strings.Contains(listing, member) {
			t.Errorf("bare `le spec` does not list the member %q:\n%s", member, listing)
		}
	}

	_, stderr = streams(t, func() { code = leroot.Dispatch("le", []string{"spec", "nope"}) })
	if code != 1 {
		t.Errorf("`le spec nope` answered %d, want 1", code)
	}
	if !strings.Contains(stderr, "is a namespace") {
		t.Errorf("`le spec nope` was not refused as a namespace member:\n%s", stderr)
	}

	// warning is the one stderr line: it names the words the rename row
	// matched, not the whole line the caller typed.
	for _, row := range []struct {
		old, fresh []string
		warning    string
	}{
		{[]string{"spec", "session"}, []string{"spec", "current"},
			"warning: le spec session is renamed: run le spec current\n"},
		{[]string{"spec", "session", "current"}, []string{"spec", "current"},
			"warning: le spec session current is renamed: run le spec current\n"},
		{[]string{"spec", "session", "state", "current"}, []string{"spec", "state", "current"},
			"warning: le spec session state is renamed: run le spec state\n"},
	} {
		old := strings.Join(row.old, " ")
		fresh := strings.Join(row.fresh, " ")
		oldCode, newCode := 0, 0
		oldOut, oldErr := streams(t, func() { oldCode = leroot.Dispatch("le", row.old) })
		newOut, newErr := streams(t, func() { newCode = leroot.Dispatch("le", row.fresh) })
		if oldCode != newCode || oldOut != newOut {
			t.Errorf("`le %s` answered (%d, %q), want what `le %s` answered (%d, %q)", old, oldCode, oldOut, fresh, newCode, newOut)
		}
		if oldErr != row.warning+newErr {
			t.Errorf("`le %s` wrote %q to stderr, want %q then %q", old, oldErr, row.warning, newErr)
		}
	}
}
