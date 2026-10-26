// VALIDATES: every row of the rename map, against the registry le really
// composes. Each retired name is refused through Dispatch naming its
// successor, exit 2 (D-retired of spec-le-cli-usability), and no retired
// command stays in the manifest (AC-2 of spec-le-subject-first-command-tree).
// PREVENTS: an old name that still runs, whether through a surviving alias or
// through a command registered under the retired spelling, and an old name
// refused with a bare `unknown command` that leaves the caller guessing.
//
// This file is an external test package so it can link internal/le, the
// composition root, which imports leroot and every area. The package's own
// tests register probes and see no area.
package leroot_test

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/le"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leretired "github.com/ze-software/ze/internal/le/le/retired"
	leroot "github.com/ze-software/ze/internal/le/le/root"
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

// TestRetiredSpellingNamesItsSuccessor drives every row of the rename map
// through Dispatch, as a caller typing the old words would, with one trailing
// word, and proves D-retired: the old spelling is refused before anything
// parses it, exit 2, with one line naming the spelling that does the work now,
// the caller's trailing word carried over. The successor is a line le really
// holds, and the old words are not a line le still holds, so the refusal can
// never shadow a live command.
func TestRetiredSpellingNamesItsSuccessor(t *testing.T) {
	for _, row := range leretired.Renames() {
		old := row.Old()
		t.Run(strings.Join(old, " "), func(t *testing.T) {
			if leroot.LookupCommand(strings.Join(old, " ")) != nil {
				t.Fatalf("the retired command `le %s` is still registered", strings.Join(old, " "))
			}
			for cut := 1; cut < len(old); cut++ {
				list, tabled := leroot.ActionsOf(strings.Join(old[:cut], " "))
				for _, action := range list.Actions {
					if tabled && action.Verb == old[cut] {
						t.Fatalf("the retired line `le %s` is the live action %q of `le %s`",
							strings.Join(old, " "), action.Verb, strings.Join(old[:cut], " "))
					}
				}
			}
			if leroot.LookupCommand(row.Command) == nil && !namespaceHolds(row.Command) {
				t.Fatalf("the successor `le %s` is not registered", row.Command)
			}
			// A forwarding area's words belong to the program it runs (`test
			// wire isis`), so only a table's own verb can be checked here.
			if list, tabled := leroot.ActionsOf(row.Command); row.Action != "" && tabled && list.PassesTo == "" {
				// A successor that takes a positional value spells it after the
				// verb (`check generated`), so the verb is the first word.
				verb := strings.Fields(row.Action)[0]
				if !slices.ContainsFunc(list.Actions, func(action leaction.Row) bool { return action.Verb == verb }) {
					t.Errorf("the successor `le %s %s` names an action `le %s` does not hold", row.Command, row.Action, row.Command)
				}
			}

			line := append(slices.Clone(old), "trailing")
			code := 0
			_, stderr := streams(t, func() { code = leroot.Dispatch("le", line) })
			if code != 2 {
				t.Errorf("answered %d, want 2: a retired spelling is a usage error", code)
			}
			want := "error: le " + strings.Join(old, " ") + " was renamed: run le " +
				strings.Join(append(row.New(), "trailing"), " ") + "\n"
			if stderr != want {
				t.Errorf("stderr is\n%q\nwant\n%q", stderr, want)
			}
		})
	}
}

// VALIDATES: AC-21 and AC-22 -- `le help <retired spelling>` and completion
// of a retired spelling answer the refusal dispatch gives for the same words:
// exit 2, the old words, and the line that does the work now, with the help
// word carried on both sides of a help question.
// PREVENTS: a help question or a Tab answering "no such action" for a
// spelling dispatch would have named the successor of, so the reader who asks
// before running learns less than the one who runs.
//
// Method: the dispatch refusal is captured first and the other two are
// compared to it, so the three are proven one mechanism, not three copies.
func TestRetiredSpellingIsRefusedByHelpAndCompletion(t *testing.T) {
	for _, row := range leretired.Renames() {
		old := row.Old()
		t.Run(strings.Join(old, " "), func(t *testing.T) {
			_, refusal := streams(t, func() { leroot.Dispatch("le", slices.Clone(old)) })
			prefix := "error: le " + strings.Join(old, " ") + " was renamed: run le "
			successor, named := strings.CutPrefix(strings.TrimSuffix(refusal, "\n"), prefix)
			if !named {
				t.Fatalf("dispatch refused `le %s` with\n%q\nwhich does not open with %q", strings.Join(old, " "), refusal, prefix)
			}

			// `ai` after the help word is reserved for the agent reference, so
			// `le help ai <words>` asks for its sections and never names the ai
			// area; a retired ai spelling is asked about as `le ai <old> help`,
			// which dispatch refuses above.
			if old[0] != "ai" {
				code := 0
				_, stderr := streams(t, func() { code = leroot.Dispatch("le", append([]string{"help"}, old...)) })
				if code != 2 {
					t.Errorf("`le help %s` answered %d, want 2: a retired spelling is a usage error", strings.Join(old, " "), code)
				}
				want := "error: le help " + strings.Join(old, " ") + " was renamed: run le help " + successor + "\n"
				if stderr != want {
					t.Errorf("`le help %s` wrote\n%q\nwant\n%q", strings.Join(old, " "), stderr, want)
				}
			}

			candidates, err := leroot.Complete(append(slices.Clone(old), ""))
			if _, retired := errors.AsType[*leroot.RetiredError](err); !retired {
				t.Fatalf("completing after `le %s` answered %v and %d candidates, want the retired refusal",
					strings.Join(old, " "), err, len(candidates))
			}
			wantErr := "le " + strings.Join(old, " ") + " was renamed: run le " + successor
			if err.Error() != wantErr {
				t.Errorf("completing after `le %s` refused with\n%q\nwant\n%q", strings.Join(old, " "), err.Error(), wantErr)
			}
		})
	}
}

// VALIDATES: completion strips what dispatch strips before it looks for a
// retired spelling: a leading help word, and the pipe chain after the
// command. `help <old> ` answers the refusal `le help <old>` gives, and
// `<old> | ` answers the refusal of `<old>` with no pipe word in its
// successor.
// PREVENTS: Tab after `help commit audit` offering nothing where the help
// question names the successor, and Tab after `commit audit |` naming a
// successor that carries the `|`, because completion matched the raw words.
//
// Method: each answer is compared with the refusal Dispatch gives for the
// same line, so completion and dispatch are proven one mechanism.
func TestRetiredSpellingIsRefusedByCompletionAfterHelpAndBeforeAPipe(t *testing.T) {
	for _, row := range leretired.Renames() {
		old := row.Old()
		t.Run(strings.Join(old, " "), func(t *testing.T) {
			_, refusal := streams(t, func() { leroot.Dispatch("le", slices.Clone(old)) })
			want := strings.TrimSuffix(strings.TrimPrefix(refusal, "error: "), "\n")
			piped := append(append(slices.Clone(old), "|"), "")
			_, err := leroot.Complete(piped)
			if _, retired := errors.AsType[*leroot.RetiredError](err); !retired {
				t.Fatalf("completing after `le %s |` answered %v, want the retired refusal", strings.Join(old, " "), err)
			}
			if err.Error() != want {
				t.Errorf("completing after `le %s |` refused with\n%q\nwant\n%q", strings.Join(old, " "), err.Error(), want)
			}

			// `ai` after the help word is the agent reference, which dispatch
			// answers before any rename is matched.
			if old[0] == "ai" {
				return
			}
			_, helpRefusal := streams(t, func() { leroot.Dispatch("le", append([]string{"help"}, old...)) })
			wantHelp := strings.TrimSuffix(strings.TrimPrefix(helpRefusal, "error: "), "\n")
			_, err = leroot.Complete(append(append([]string{"help"}, old...), ""))
			if _, retired := errors.AsType[*leroot.RetiredError](err); !retired {
				t.Fatalf("completing after `le help %s` answered %v, want the retired refusal", strings.Join(old, " "), err)
			}
			if err.Error() != wantHelp {
				t.Errorf("completing after `le help %s` refused with\n%q\nwant\n%q", strings.Join(old, " "), err.Error(), wantHelp)
			}
		})
	}
}

// VALIDATES: the whole line a refusal names is one le's live tables accept,
// not only its first word: the area is registered, the verb is one its table
// holds, and every word after the verb parses under that verb's grammar.
// PREVENTS: a successor whose verb exists but whose next word the verb
// refuses (`check generatd`, a keyword the action never declared), which the
// verb-only check of TestRetiredSpellingNamesItsSuccessor passes.
func TestRetiredSuccessorLineIsAcceptedWhole(t *testing.T) {
	parsed := 0
	for _, row := range leretired.Renames() {
		line := row.Successor(row.Old())
		spelled := strings.Join(line, " ")
		area, rest := liveArea(line)
		if area == "" {
			if !namespaceHolds(spelled) {
				t.Errorf("the successor `le %s` of `le %s` names no registered command or namespace",
					spelled, strings.Join(row.Old(), " "))
			}
			continue
		}
		list, tabled := leroot.ActionsOf(area)
		// A forwarding area's words belong to the program it runs, and a
		// hand-rolled area publishes no grammar to parse against.
		if !tabled || list.PassesWords || list.PassesTo != "" {
			continue
		}
		words := list.AtVerb(rest)
		if len(words) == 0 {
			continue
		}
		if _, err := list.Missing(words[0], words[1:]); err != nil {
			t.Errorf("the successor `le %s` of `le %s` is refused by `le %s`: %v",
				spelled, strings.Join(row.Old(), " "), area, err)
			continue
		}
		parsed++
	}
	if parsed == 0 {
		t.Error("no successor line was parsed against an action table, so this test proves nothing")
	}
}

// liveArea answers the longest registered command that opens line, and the
// words after it, or "" when no prefix of line is registered.
func liveArea(line []string) (string, []string) {
	for cut := len(line); cut > 0; cut-- {
		name := strings.Join(line[:cut], " ")
		if leroot.LookupCommand(name) != nil {
			return name, line[cut:]
		}
	}
	return "", nil
}

// namespaceHolds reports whether some registered command sits under the
// namespace word sequence: `test harness` is answered by the members of `test`.
func namespaceHolds(namespace string) bool {
	for _, command := range leroot.Commands() {
		if strings.HasPrefix(command.Name, namespace+" ") {
			return true
		}
	}
	return false
}

func TestRetiredNamesAreNotInTheManifest(t *testing.T) {
	listed := make(map[string]bool, 128)
	for _, command := range leroot.Commands() {
		listed[command.Name] = true
	}

	reported := make(map[string]bool, 64)
	for _, row := range leretired.Renames() {
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
	// A namespace non-member is a usage error (D-Q8): 2.
	if code != 2 {
		t.Errorf("`le spec nope` answered %d, want 2", code)
	}
	if !strings.Contains(stderr, "is a namespace") {
		t.Errorf("`le spec nope` was not refused as a namespace member:\n%s", stderr)
	}
}
