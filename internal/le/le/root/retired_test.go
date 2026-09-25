// VALIDATES: every row of the rename map, against the registry le really
// composes. Each retired name answers `unknown command` and exit 1 through
// Dispatch (AC-17), and no retired command stays in the manifest (AC-2).
// PREVENTS: an old name that still runs, whether through a surviving alias or
// through a command registered under the retired spelling.
//
// This file is an external test package so it can link internal/le, the
// composition root, which imports leroot and every area. The package's own
// tests register probes and see no area.
package leroot_test

import (
	"os"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/le"
	"github.com/ze-software/ze/internal/le/doc/check"
	"github.com/ze-software/ze/internal/le/le/root"
)

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

// TestEveryRetiredNameAnswersUnknownCommand drives every row of the rename map
// through Dispatch, as a caller typing the old words would, and proves AC-17:
// the old name is refused with no rename line and runs nothing. The refusal
// depends on what the old words now meet.
//
//   - A word le does not register answers `unknown command` and exit 1.
//   - A first word that is now a namespace (`spec session`, `test harness`)
//     is refused as a word that names no member of it, exit 1.
//   - A retired ACTION of a command that is still registered (`verify lock`,
//     `repo tracked-build`) reaches that command's closed action table, which
//     refuses the verb with `no such action` and exit 2 before any work runs
//     (leaction.Area.refuseVerb).
func TestEveryRetiredNameAnswersUnknownCommand(t *testing.T) {
	for _, row := range doccheck.Renames() {
		old := row.Old()
		t.Run(strings.Join(old, " "), func(t *testing.T) {
			code := 0
			_, stderr := streams(t, func() { code = leroot.Dispatch("le", old) })
			if strings.Contains(stderr, "is renamed") {
				t.Errorf("wrote a rename line, so an alias survives:\n%s", stderr)
			}
			if leroot.LookupCommand(strings.Join(old, " ")) != nil {
				t.Fatalf("the retired command `le %s` is still registered", strings.Join(old, " "))
			}
			if len(old) > 1 && leroot.LookupCommand(old[0]) != nil {
				refusal := "error: no such action in " + old[0] + ": " + old[1]
				if code != 2 || !strings.Contains(stderr, refusal) {
					t.Errorf("answered %d with\n%s\nwant 2 and %q", code, stderr, refusal)
				}
				return
			}
			if code != 1 {
				t.Errorf("answered %d, want 1", code)
			}
			refused := strings.Contains(stderr, "unknown command: "+old[0]) ||
				strings.Contains(stderr, "error: "+old[0]+" is a namespace; it needs one of:")
			if !refused {
				t.Errorf("the refusal names neither an unknown command nor a namespace member:\n%s", stderr)
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
	for _, row := range doccheck.Renames() {
		if reported[row.Retired] {
			continue
		}
		reported[row.Retired] = true
		if listed[row.Retired] {
			t.Errorf("`le %s` is retired and still in the manifest; its replacement is `le %s`", row.Retired, row.Command)
		}
	}
}

// TestSpecIsANamespace pins the one family whose retired command became a
// namespace rather than a command. `spec session` flattened into `spec`, so
// the bare `le spec` asks what the namespace holds (AC-31), and a word that
// names no member is refused. The retired `spec session` rows are refused by
// the row test above.
func TestSpecIsANamespace(t *testing.T) {
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
}
