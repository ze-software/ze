// Design: docs/contributing/committing.md -- authorization retries preserve the actual operation.
package commit

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestAttestationRefusalPreservesRetryOperation decodes the actual refusal and
// replays the registered Create writer under a PTY, never its prepared script.
// The original target, owner words, message options and namespace must survive
// shell-sensitive text and a different operator checkout/session/agent.
func TestAttestationRefusalPreservesRetryOperation(t *testing.T) {
	humantest.Streams(t, "piped-both")
	root := newOwedRepository(t)
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "attest-retry-origin")
	t.Setenv("ZE_AGENT_ID", "origin-agent")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	owed := commitOwed(t, root, "the change", map[string]string{"a.txt": "a\n"}, nil, "Review-owed: demo: no reviewer")
	options := Options{Subject: "owner's \"$widget\"", Body: []string{"a `printf marker` body", "", "Thomas's verdict"},
		Attest: owed, AttestWords: "Thomas's \"$widget\" and `printf marker`", Session: "abcd1234", Tag: "owner",
		Replace: true}
	t.Run("prepared-script", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		prepared, err := Create(root, &Options{Subject: options.Subject, Body: options.Body, Attest: owed,
			AttestWords: "the prior fixture instruction", Session: options.Session, Tag: "prior"})
		if err != nil {
			t.Fatal(err)
		}
		options.Script = prepared.Script
	})
	_, err := Create(root, &options)
	t.Setenv("ZE_REPO_ROOT", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "operator-fixture")
	t.Setenv("ZE_AGENT_ID", "operator-agent")
	env.ResetCache()
	arguments := humantest.RetryArguments(t, err)
	if !slices.Equal(arguments[:3], []string{"./le", "commit", "create"}) {
		t.Fatalf("attest retry route = %q", arguments)
	}
	retry, problem := parseCreate(arguments[3:])
	if problem != nil {
		t.Fatal(problem)
	}
	if !sameAttestationRetryOptions(&retry, &options) {
		t.Errorf("retry options = %+v, want %+v", retry, options)
	}
	t.Run("retry", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		payload, code := Answer(arguments[2:])
		if code != 0 {
			t.Fatalf("printed attest retry exit = %d, want 0", code)
		}
		prepared, ok := payload.(*Prepared)
		if !ok {
			t.Fatalf("create answer = %T", payload)
		}
		message, err := os.ReadFile(filepath.Join(root, prepared.Message))
		if err != nil {
			t.Fatal(err)
		}
		want := trailerOwnerAttested + stablePatchID(t, root, owed) + ": " + options.AttestWords
		if !strings.Contains(string(message), want) || prepared.Session != options.Session || prepared.Script != options.Script {
			t.Fatalf("replayed attestation = %+v, message %q; want %q", prepared, message, want)
		}
	})
}

// TestPushRefusalPreservesRetryOperation replays only preparation through the
// registered push entry under a PTY. It reads the resulting script and target;
// no script or actual push is executed.
func TestPushRefusalPreservesRetryOperation(t *testing.T) {
	humantest.Streams(t, "piped-both")
	root := newOwedRepository(t)
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "push-retry-origin")
	t.Setenv("ZE_AGENT_ID", "origin-agent")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	head := commitOwed(t, root, "the change", map[string]string{"a.txt": "a\n"}, nil)
	session, err := lepath.CommitSession(root, "")
	if err != nil {
		t.Fatal(err)
	}
	words := "Thomas's \"$widget\" and `printf marker` may be pushed"
	_, _, err = preparePush(root, words)
	t.Setenv("ZE_REPO_ROOT", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "operator-fixture")
	t.Setenv("ZE_AGENT_ID", "operator-agent")
	env.ResetCache()
	arguments := humantest.RetryArguments(t, err)
	want := []string{"./le", "commit", "push", words}
	if !slices.Equal(arguments, want) {
		t.Errorf("push retry argv = %q, want %q", arguments, want)
	}
	t.Run("retry", func(t *testing.T) {
		humantest.Streams(t, "terminal")
		payload, code := Answer(arguments[2:])
		if code != 0 {
			t.Fatalf("printed push retry exit = %d, want 0", code)
		}
		prepared, ok := payload.(*pushPrepared)
		if !ok {
			t.Fatalf("push answer = %T", payload)
		}
		if prepared.Target != head || !strings.Contains(prepared.Script, "commit-"+session+"-") {
			t.Errorf("retry target or namespace changed: %+v", prepared)
		}
		if script := readScript(t, root, prepared.Script); !strings.Contains(script, markerLine(pushMarker, words)) {
			t.Fatalf("replayed push script lost the owner's instruction: %s", script)
		}
	})
}

// sameAttestationRetryOptions compares every operand while treating nil and empty
// slices as the same absence. Remaining fields retain full structural comparison.
func sameAttestationRetryOptions(firstOptions, secondOptions *Options) bool {
	first, second := *firstOptions, *secondOptions
	if !slices.Equal(first.Body, second.Body) {
		return false
	}
	if !slices.Equal(first.Files, second.Files) {
		return false
	}
	if !slices.Equal(first.Hunks, second.Hunks) {
		return false
	}
	if !slices.Equal(first.Remove, second.Remove) {
		return false
	}
	if !slices.Equal(first.LedgerCarried, second.LedgerCarried) {
		return false
	}
	first.Body, second.Body = nil, nil
	first.Files, second.Files = nil, nil
	first.Hunks, second.Hunks = nil, nil
	first.Remove, second.Remove = nil, nil
	first.LedgerCarried, second.LedgerCarried = nil, nil
	return reflect.DeepEqual(first, second)
}
