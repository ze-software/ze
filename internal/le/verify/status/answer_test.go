// VALIDATES: verify status dispatches through its declared action table, and
// the table publishes `path` as the repeatable keyword the check verb reads.
// PREVENTS: the four verbs drifting from the codes they answer today, and the
// repeat machinery keeping a test as its only user, which is dead code the
// manifest still advertises (ai/rules/completion.md).
package verifystatus

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/verify"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestTheCheckVerbDeclaresARepeatablePathKeyword holds the published grammar to
// what the body reads. check accumulates every `path` the operator names, so
// the keyword is declared Repeat and read with Values.
func TestTheCheckVerbDeclaresARepeatablePathKeyword(t *testing.T) {
	list := Actions()
	if list.Area != name {
		t.Fatalf("the table names the area %q, want %q", list.Area, name)
	}

	verbs := make(map[string]leaction.Row, len(list.Actions))
	for _, row := range list.Actions {
		verbs[row.Verb] = row
	}
	for _, verb := range []string{"write", "check", "show", "tree-hash"} {
		if _, held := verbs[verb]; !held {
			t.Errorf("the table holds no %q, so the verb is unreachable", verb)
		}
	}

	check := verbs["check"].Parameters
	if len(check) != 1 {
		t.Fatalf("check declares %d parameters, want one", len(check))
	}
	if check[0].Keyword != "path" || !check[0].Repeat {
		t.Errorf("check declares %+v, want a repeatable path keyword", check[0])
	}
	if check[0].Requirement != leaction.Optional {
		t.Errorf("check declares path %v, want Optional: a bare check reads the whole tree", check[0].Requirement)
	}
}

// TestCheckAcceptsThePathKeywordTwice proves the repeat declaration is live in
// the parser. Two path pairs answer the freshness verdict, rather than the
// refusal a keyword given twice earns.
func TestCheckAcceptsThePathKeywordTwice(t *testing.T) {
	answer, code := Answer([]string{"check", "path", "internal", "path", "docs"})
	if code == 2 {
		t.Fatalf("two path keywords were refused by the parser, code %d", code)
	}
	if _, verdict := answer.(verifyengine.Freshness); !verdict {
		t.Fatalf("check answered %T, want a freshness verdict", answer)
	}
}

// TestEveryRefusalKeepsTheCodeItAnsweredBeforeTheTable pins the exit-code
// discipline through the real entry point. Exit codes are their own spec, so
// the migration onto the action table MUST NOT move one.
func TestEveryRefusalKeepsTheCodeItAnsweredBeforeTheTable(t *testing.T) {
	for _, args := range [][]string{
		{"write"},
		{"write", "exit-code"},
		{"write", "exit-code", "not-a-number"},
		{"write", "mode", "full"},
		{"write", "exit-code", "0", "mode"},
		{"check", "path"},
		{"check", "scope", "internal"},
		{"show", "extra"},
		{"tree-hash", "extra"},
		{"no-such-verb"},
	} {
		answer, code := Answer(args)
		if code != 2 {
			t.Errorf("%v answered %d, want 2", args, code)
		}
		if answer != nil {
			t.Errorf("%v answered a payload: %v", args, answer)
		}
	}
}

// TestTheBareAreaListsItsActions keeps `le verify status` answering what it
// holds, with no verb named and nothing run.
func TestTheBareAreaListsItsActions(t *testing.T) {
	answer, code := Answer(nil)
	list, published := answer.(leaction.List)
	if code != 0 || !published {
		t.Fatalf("the bare area answered %T, %d; want the listing and 0", answer, code)
	}
	if len(list.Actions) != 4 {
		t.Errorf("the listing carries %d actions, want four", len(list.Actions))
	}
}

// TestStatusWriteNeverWritesGreenRecord: `le verify status write exit-code 0
// mode full` writes the certificate as it always has, and the green record is
// neither written nor changed. The record is the verify lifecycle's alone: a
// caller-declared exit code proves no run, so a push cannot be bought with one.
// The method records a green record first and compares its bytes after.
//
// MUTATION: write the green record from WriteCertificate and this fails.
func TestStatusWriteNeverWritesGreenRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("tmp/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := env.Get(lepath.RootKey)
	if err := env.Set(lepath.RootKey, root); err != nil {
		t.Fatalf("point the checkout root at the fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set(lepath.RootKey, previous); err != nil {
			t.Fatalf("restore the checkout root: %v", err)
		}
	})

	if _, code := Answer([]string{"write", "exit-code", "0", "mode", "full"}); code != 0 {
		t.Fatalf("status write answered %d", code)
	}
	if _, err := verifyengine.ReadCertificate(root); err != nil {
		t.Fatalf("status write wrote no certificate: %v", err)
	}
	if _, err := verify.ReadGreenRecord(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("status write made a green record (%v)", err)
	}

	held, err := verify.NewGreenRecord(strings.Repeat("a", 40), strings.Repeat("b", 40), 1,
		time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := verify.WriteGreenRecord(root, held); err != nil {
		t.Fatal(err)
	}
	if _, code := Answer([]string{"write", "exit-code", "0", "mode", "full"}); code != 0 {
		t.Fatalf("status write answered %d", code)
	}
	after, err := verify.ReadGreenRecord(root)
	if err != nil || after != held {
		t.Errorf("status write changed the green record to (%+v, %v), want %+v", after, err, held)
	}
}
