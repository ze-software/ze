package le

import (
	"strings"
	"testing"

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
