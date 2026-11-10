package le

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/helpfmt"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leretired "github.com/ze-software/ze/internal/le/le/retired"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// VALIDATES: every renamed action a caller still types reaches a registered
// action, and no retired spelling is still served. For each row of the rename
// map (leretired.Renames, the one declaration of old and new spellings) that
// names a new action, that action is a verb its command's own table declares;
// for every row, the old words name no live command and no verb of one.
// PREVENTS: the 2026-10-27 defects, where le phase 4 renamed `le doc index
// write` to `update`, `le verify status write` to `record` and the `le test
// qemu` names dropped `-test`, while callers kept the old spellings: a row
// whose successor is itself gone answers a caller with a command that does not
// exist, and an old spelling still registered makes its row dead, so a caller
// on the old spelling never learns of the rename.
//
// Method: membership through the live registry, the same lookups the
// dispatcher reads (leroot.LookupCommand, leroot.ActionsOf, List.AtVerb). No
// row count is asserted, so a new rename leaves the test green while it
// resolves. TestEveryNewNameResolvesToItsArea (rename_test.go) judges the
// command half of each row; this test judges the action half and the old side.
//
// MUTATION: (le/retired/retired.go, `{Retired: "verify status write", Command: "verify status", Action: "record"},`, `{Retired: "verify status write", Command: "verify status", Action: "write"},`): the row sends a caller to a verb no table declares, and this test goes red.
// MUTATION: (le/retired/retired.go, `{Retired: "doc index write", Command: "doc index", Action: "update"},`, `{Retired: "doc index check", Command: "doc index", Action: "update"},`): the row claims a spelling the doc index table still serves, and this test goes red.
func TestRenamedCallersReachRegisteredActions(t *testing.T) {
	judged := 0
	for _, row := range leretired.Renames() {
		old := strings.Join(row.Old(), " ")
		if live := liveSpelling(row.Old()); live != "" {
			t.Errorf("retired %q is still served as %q, so its rename to %q is never reached", old, live, strings.Join(row.New(), " "))
		}
		if row.Action == "" {
			continue
		}
		judged++
		if reason := unresolvedAction(row.New()); reason != "" {
			t.Errorf("retired %q is renamed to %q, which %s", old, strings.Join(row.New(), " "), reason)
		}
	}
	if judged == 0 {
		t.Fatal("the rename map holds no action rename, so the action half judged nothing")
	}
}

// longestCommand answers the longest prefix of words that names a registered
// command, and the words after it. It answers an empty name when none does.
func longestCommand(words []string) (string, []string) {
	for length := len(words); length > 0; length-- {
		name := strings.Join(words[:length], " ")
		if leroot.LookupCommand(name) != nil {
			return name, words[length:]
		}
	}
	return "", words
}

// declaresVerb reports whether the table declares verb.
func declaresVerb(list leaction.List, verb string) bool {
	for _, row := range list.Actions {
		if row.Verb == verb {
			return true
		}
	}
	return false
}

// unresolvedAction answers why words do not reach a declared action, or an
// empty string when they do. A row's new words are a command followed by the
// verb a caller's own words follow.
func unresolvedAction(words []string) string {
	command, rest := longestCommand(words)
	if command == "" {
		return "names no registered command"
	}
	list, declared := leroot.ActionsOf(command)
	// An untabled command parses its own words, so registration is all this
	// registry can judge, as workflowcheck.actionExists judges a bare command.
	if !declared {
		return ""
	}
	if list.PassesWords {
		return ""
	}
	line := list.AtVerb(rest)
	if len(line) == 0 {
		return "le " + command + " is reached with no action"
	}
	if !declaresVerb(list, line[0]) {
		return "le " + command + " declares no action " + line[0]
	}
	return ""
}

// liveSpelling answers the live command line the retired words are still
// spelt as, or an empty string. It judges literal spelling only: a default
// action a table supplies (List.AtVerb) does not make a retired word live,
// because the caller did not type it.
func liveSpelling(words []string) string {
	command, rest := longestCommand(words)
	if command == "" {
		return ""
	}
	if len(rest) == 0 {
		return command
	}
	list, declared := leroot.ActionsOf(command)
	if !declared {
		return ""
	}
	if list.PassesWords {
		return ""
	}
	if declaresVerb(list, rest[0]) {
		return command + " " + rest[0]
	}
	return ""
}

// VALIDATES: P-38 of spec-tooling-one-off-defects, the successful-dispatch half
// that TestRenamedCallersReachRegisteredActions leaves open. For every row of
// the rename map (leretired.Renames) that names a new action, the current line
// goes through leroot.Dispatch, the dispatcher the le binary runs, and answers
// exit 0 with exactly that action's own page: once spelt as a caller types it
// with a trailing `--help` (the dispatchLine route: retired-spelling refusal,
// resolve, asksForUsage, helpTrailing), and once as `le help <line>` (the
// helpAsked route). The QEMU names the retired TestQEMUActionsIncludeTheHostRun
// snapshot asserted (`ipsec-mobike` among them) are rows of that map, so each
// is judged here by its current name with no hand list.
// PREVENTS: a renamed action that resolves in the registry but whose current
// line dispatches somewhere else. helpTrailing answers the AREA page with exit
// 0 for a verb its listing does not page, so a resolver or listing defect
// would leave every caller on the area page with a green exit code.
//
// Method: the expected page is the action's own listing rendered by
// leaction.List.ActionPage through the same kind of stdout file the dispatch
// writes to, so the comparison is byte-exact. The help word is the
// dash-leading `--help`, which the grammar never reads as a positional value
// (trailingAsksForHelp), and the test asks List.TrailingWordAsksForHelp before
// it dispatches, so no row can start an action's work. A forwarding row hands
// a trailing help word to its program, so it is judged on the `le help` route
// only. Running an action's handler is not side-effect-free for these rows (a
// QEMU guest boot, a regeneration, a publication), so dispatch is proved up to
// the action's own declared row, the last step before its handler.
//
// MUTATION: (le/root/dispatch.go helpTrailing, `if _, held := list.ActionPage(program, own[0]); !held {`, `if _, held := list.ActionPage(program, own[0]); held {`): every trailing-help line answers its area page with exit 0, and this test goes red on every non-forwarding row while TestRenamedCallersReachRegisteredActions stays green.
// MUTATION: (le/root/dispatch.go helpAsked, `return writeActionPage(program, name, list, own[0])`, `return helpPage(program, name)`): every `le help <command> <verb>` line answers its area page with exit 0, and this test goes red on every row.
func TestRenamedActionsDispatchToTheirOwnPage(t *testing.T) {
	judged := 0
	seen := make(map[string]bool)
	for _, row := range leretired.Renames() {
		if row.Action == "" {
			continue
		}
		command, rest := longestCommand(row.New())
		// TestRenamedCallersReachRegisteredActions refuses a row that names no
		// command or no declared verb, so it is not judged twice here.
		if command == "" {
			continue
		}
		list, declared := leroot.ActionsOf(command)
		// An untabled or pass-through command declares no action to page.
		if !declared {
			continue
		}
		if list.PassesWords {
			continue
		}
		line := list.AtVerb(rest)
		if len(line) == 0 {
			continue
		}
		if !declaresVerb(list, line[0]) {
			continue
		}
		verb := line[0]
		if verb == "" {
			t.Errorf("rename %q names an action of the verbless area %q, which has no action word to dispatch", strings.Join(row.Old(), " "), command)
			continue
		}
		words := append(strings.Fields(command), verb)
		key := strings.Join(words, " ")
		if seen[key] {
			continue
		}
		seen[key] = true
		judged++

		page, held := list.ActionPage("le", verb)
		if !held {
			t.Errorf("le %s declares %q but pages no action for it", command, verb)
			continue
		}
		want := pageToStdout(t, &page)

		got, code := dispatchToStdout(t, append([]string{"help"}, words...)...)
		if code != 0 || got != want {
			t.Errorf("le help %s = exit %d, page:\n%s\nwant exit 0 and the action's own page:\n%s", key, code, got, want)
		}
		if actionForwards(list, verb) {
			continue
		}
		if !list.TrailingWordAsksForHelp([]string{verb, "--help"}) {
			t.Errorf("le %s --help is not read as a help question, so dispatching it would run the action", key)
			continue
		}
		got, code = dispatchToStdout(t, append(slices.Clone(words), "--help")...)
		if code != 0 || got != want {
			t.Errorf("le %s --help = exit %d, page:\n%s\nwant exit 0 and the action's own page:\n%s", key, code, got, want)
		}
	}
	if judged == 0 {
		t.Fatal("the rename map holds no action rename that reaches a paged action, so nothing was dispatched")
	}
}

// actionForwards reports whether verb's row hands the words after it to
// another program, which then owns a trailing help word.
func actionForwards(list leaction.List, verb string) bool {
	for _, row := range list.Actions {
		if row.Verb == verb {
			return row.Forwards
		}
	}
	return false
}

// pageToStdout renders page the way the dispatcher does (helpfmt.Page.WriteOut
// at leaction.PageWidth of stdout), with stdout on a file as dispatchToStdout
// sets it, and answers the bytes.
func pageToStdout(t *testing.T, page *helpfmt.Page) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "page")
	if err != nil {
		t.Fatalf("create the page file: %v", err)
	}
	defer file.Close() //nolint:errcheck // a test's temporary file

	saved := os.Stdout
	os.Stdout = file
	page.Width = leaction.PageWidth(os.Stdout)
	page.WriteOut()
	os.Stdout = saved

	out, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read the page file: %v", err)
	}
	return string(out)
}
