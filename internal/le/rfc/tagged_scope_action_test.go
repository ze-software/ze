package rfc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/env"
)

const taggedScopeOld = `package x

// ` + rfcTagMarker + ` RFC9999-2-1 positive
func TestOne() {
	got := 1
	_ = got
}
`

const taggedScopeChanged = `package x

// ` + rfcTagMarker + ` RFC9999-2-1 positive
func TestOne() {
	got := 2
	_ = got
}
`

func TestTaggedScopeActionPublishesClosedKeywordGrammar(t *testing.T) {
	found := false
	for _, action := range Actions().Actions {
		if action.Verb == "tagged-scope" {
			found = true
			if action.Writes {
				t.Error("tagged-scope is published as a writer")
			}
		}
	}
	if !found {
		t.Fatal("RFC action catalogue does not publish tagged-scope")
	}
	if _, code := Answer([]string{"tagged-scope", "some/path"}); code != 2 {
		t.Errorf("a path before its keyword answered %d, want grammar refusal 2", code)
	}
	if answer, code := Answer([]string{"tagged-scope"}); code != 2 || answer == nil {
		t.Errorf("a missing path answered (%T, %d), want structured refusal and 2", answer, code)
	}
}

func taggedScopeRequest(operation, content string, hunks ...EditHunk) taggedScopeActionRequest {
	return taggedScopeActionRequest{Operation: operation, Content: &content, Hunks: hunks}
}

func runTaggedScopeAction(t *testing.T, tree, path string, request any) (taggedScopeActionReport, int) {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal stdin request: %v", err)
	}
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	restore := cliio.SwapStreams(bytes.NewReader(body), io.Discard)
	t.Cleanup(restore)
	answer, code := Answer([]string{"tagged-scope", "path", path})
	report, ok := answer.(taggedScopeActionReport)
	if !ok {
		t.Fatalf("tagged-scope answered %T, want TaggedScopeReport", answer)
	}
	return report, code
}

func TestTaggedScopeStdinHandlesWriteAndEditProposals(t *testing.T) {
	t.Run("new Write", func(t *testing.T) {
		tree := fixtureTree(t, map[string]string{"go.mod": "module x\n", "feature-gates.txt": ""})
		report, code := runTaggedScopeAction(t, tree, "internal/x/new_test.go",
			taggedScopeRequest("write", taggedScopeOld))
		if code != 0 || !report.Allowed || !report.Carrier || report.Scope != nil || report.Reason != "no-tags" {
			t.Errorf("new Write answered code %d, report %+v", code, report)
		}
	})

	t.Run("overwrite Write", func(t *testing.T) {
		tree := fixtureTree(t, map[string]string{
			"go.mod": "module x\n", "feature-gates.txt": "",
			"internal/x/x_test.go": taggedScopeOld,
		})
		report, code := runTaggedScopeAction(t, tree, "internal/x/x_test.go",
			taggedScopeRequest("write", taggedScopeChanged))
		if code != 1 || report.Allowed || report.Decision != "block" || len(report.Changes) != 1 {
			t.Fatalf("overwrite Write answered code %d, report %+v", code, report)
		}
		if report.Changes[0].Name != "TestOne" ||
			!strings.Contains(report.Message, "test/rfc-changed.md") {
			t.Errorf("overwrite Write is not actionable: %+v", report)
		}
	})

	t.Run("Edit", func(t *testing.T) {
		tree := fixtureTree(t, map[string]string{
			"go.mod": "module x\n", "feature-gates.txt": "",
			"internal/x/x_test.go": taggedScopeOld,
		})
		report, code := runTaggedScopeAction(t, tree, "internal/x/x_test.go",
			taggedScopeRequest("edit", taggedScopeChanged, EditHunk{Old: "got := 1"}))
		if code != 1 || report.Scope == nil || report.Resolution != ScopeFunc {
			t.Fatalf("Edit answered code %d, report %+v", code, report)
		}
		want := FunctionUnits(taggedScopeOld)[0].Text
		if *report.Scope != want {
			t.Errorf("widened scope differs:\nwant:\n%s\ngot:\n%s", want, *report.Scope)
		}
	})
}

func TestTaggedScopeAllowsUnchangedAndNonRFCCases(t *testing.T) {
	cases := []struct {
		name, path, oldText, proposed, reason string
		carrier                               bool
	}{
		{name: "unchanged", path: "internal/x/x_test.go", oldText: taggedScopeOld,
			proposed: taggedScopeOld, reason: "unchanged", carrier: true},
		{name: "not carrier", path: "internal/x/x.go", oldText: taggedScopeOld,
			proposed: taggedScopeChanged, reason: "not-carrier"},
		{name: "no tags", path: "internal/x/x_test.go",
			oldText:  "package x\nfunc TestOne() {}\n",
			proposed: "package x\nfunc TestOne() { panic(1) }\n", reason: "no-tags", carrier: true},
		{name: "comment only", path: "internal/x/x_test.go", oldText: taggedScopeOld,
			proposed: strings.Replace(taggedScopeOld, "got := 1", "got := 1 // same assertion", 1),
			reason:   "unchanged-behaviour", carrier: true},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tree := fixtureTree(t, map[string]string{
				"go.mod": "module x\n", "feature-gates.txt": "", one.path: one.oldText,
			})
			report, code := runTaggedScopeAction(t, tree, one.path,
				taggedScopeRequest("write", one.proposed))
			if code != 0 || !report.Allowed || report.Reason != one.reason || report.Carrier != one.carrier {
				t.Errorf("answer code %d, report %+v", code, report)
			}
		})
	}
}

func TestTaggedScopeStdinAndReadFailuresFailClosed(t *testing.T) {
	valid, err := json.Marshal(taggedScopeRequest("write", "x"))
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, err := readTaggedScopeRequest(bytes.NewReader(valid), int64(len(valid))); err != nil {
		t.Errorf("request at the boundary was refused: %v", err)
	}
	if _, err := readTaggedScopeRequest(bytes.NewReader(append(valid, ' ')), int64(len(valid))); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized request was not refused: %v", err)
	}
	if _, err := readTaggedScopeRequest(errorReader{}, 100); err == nil ||
		!strings.Contains(err.Error(), "read proposed content") {
		t.Errorf("stdin read error was not refused: %v", err)
	}
	for _, malformed := range []string{
		``,
		`{"operation":"edit","content":"x"}`,
		`{"operation":"write"}`,
		`{"operation":"edit","content":"x","hunks":[{"old":""}]}`,
		`{"operation":"invented","content":"x"}`,
		`{"operation":"write","content":"x","unknown":true}`,
		`{"operation":"write","content":"\u0000"}`,
	} {
		if _, err := readTaggedScopeRequest(strings.NewReader(malformed), 1024); err == nil {
			t.Errorf("malformed stdin was accepted: %q", malformed)
		}
	}
	tree := fixtureTree(t, map[string]string{
		"go.mod": "module x\n", "feature-gates.txt": "",
		"internal/x/bad_test.go/held": "child",
	})
	request := taggedScopeRequest("write", "x")
	if report, err := evaluateTaggedScope(tree, "internal/x/bad_test.go", request); err == nil ||
		report.Decision != "error" {
		t.Errorf("existing-file read error did not fail closed: %+v, %v", report, err)
	}
	outside := filepath.Join(filepath.Dir(tree), "outside_test.go")
	if report, err := evaluateTaggedScope(tree, outside, request); err == nil || report.Decision != "error" {
		t.Errorf("outside path did not fail closed: %+v, %v", report, err)
	}
	missingEdit := taggedScopeRequest("edit", "x", EditHunk{Old: "held"})
	if report, err := evaluateTaggedScope(tree, "internal/x/missing_test.go", missingEdit); err == nil ||
		report.Decision != "error" {
		t.Errorf("missing Edit target did not fail closed: %+v, %v", report, err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestTaggedScopeMalformedActionInputExitsCannotRun(t *testing.T) {
	tree := fixtureTree(t, map[string]string{"go.mod": "module x\n", "feature-gates.txt": ""})
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	restore := cliio.SwapStreams(strings.NewReader("{"), io.Discard)
	t.Cleanup(restore)
	answer, code := Answer([]string{"tagged-scope", "path", "internal/x/x_test.go"})
	report, ok := answer.(taggedScopeActionReport)
	if code != 2 || !ok || report.Decision != "error" || report.Allowed {
		t.Errorf("malformed action input answered code %d, %#v", code, answer)
	}
}

func TestTaggedScopeDoesNotWriteTheProposedContent(t *testing.T) {
	tree := fixtureTree(t, map[string]string{
		"go.mod": "module x\n", "feature-gates.txt": "", "internal/x/x_test.go": taggedScopeOld,
	})
	if _, err := evaluateTaggedScope(tree, "internal/x/x_test.go",
		taggedScopeRequest("write", taggedScopeChanged)); err != nil {
		t.Fatalf("EvaluateTaggedScope: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(tree, "internal", "x", "x_test.go"))
	if err != nil {
		t.Fatalf("read old file: %v", err)
	}
	if string(body) != taggedScopeOld {
		t.Error("the read-only scope action wrote proposed content")
	}
}
