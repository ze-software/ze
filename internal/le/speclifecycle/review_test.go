package speclifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReviewRoundSixNeedsAProductDefectAndOwnerAuthorisation(t *testing.T) {
	// VALIDATES: round 6 requires both bounds to be lifted independently.
	// PREVENTS: a session authorising its own unbounded review loop or using the
	// owner's words without naming what the extra product pass found.
	root := reviewFixture(t)
	base := reviewRecord{Spec: "demo", Verdict: "clean", Rounds: 6, Files: []string{"pkg/a.go"}, Model: "claude-opus-5", SessionID: "session"}
	_, err := recordReview(root, base)
	if err == nil {
		t.Fatal("missing rounds reason was accepted")
	}
	if !strings.Contains(err.Error(), "rounds reason") {
		t.Fatalf("missing reason error = %v", err)
	}
	base.RoundsReason = "round 5 found a guard that fails open"
	_, err = recordReview(root, base)
	if err == nil {
		t.Fatal("missing owner authorisation was accepted")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "authorisation") {
		t.Fatalf("missing authorisation error = %v", err)
	}
	base.OwnerAuthorised = "Thomas asked for one more pass"
	artifact, err := recordReview(root, base)
	if err != nil {
		t.Fatalf("recordReview: %v", err)
	}
	if artifact.Rounds != 6 {
		t.Fatalf("artifact rounds = %d", artifact.Rounds)
	}
	if artifact.OwnerAuthorised == "" {
		t.Fatalf("artifact = %#v", artifact)
	}
}

func TestReviewRoundBoundaryAcceptsFiveAndRejectsZero(t *testing.T) {
	// VALIDATES: five rounds is the last unqualified review count and zero
	// cannot claim that a review ran.
	// PREVENTS: moving either side of the review-round boundary by one.
	root := reviewFixture(t)
	request := reviewRecord{
		Spec: "demo", Verdict: "clean", Files: []string{"pkg/a.go"},
		Model: "claude-opus-5", SessionID: "session",
	}
	if _, err := recordReview(root, request); err == nil {
		t.Fatal("zero review rounds were accepted")
	}
	request.Rounds = 5
	if _, err := recordReview(root, request); err != nil {
		t.Fatalf("five review rounds: %v", err)
	}
}

func TestReviewArtifactIsSessionScopedAndDetectsAStaleFile(t *testing.T) {
	// VALIDATES: recording pins the exact file bytes under one session ID and a
	// later edit blocks that session's check.
	// PREVENTS: same-spec sessions sharing evidence or post-review edits riding a
	// clean verdict.
	root := reviewFixture(t)
	request := reviewRecord{
		Spec: "plan/spec-demo.md", Verdict: "clean", Rounds: 1,
		Files: []string{"pkg/a.go"}, Model: "claude-opus-5", SessionID: "session-a",
		Now: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
	}
	artifact, err := recordReview(root, request)
	if err != nil {
		t.Fatalf("recordReview: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(artifact.Path), "tmp/review/demo-session-a.md") {
		t.Fatalf("artifact path = %s", artifact.Path)
	}
	other, err := CheckReview(root, "demo", "session-b", []string{"pkg/a.go"})
	if err != nil {
		t.Fatalf("other-session check: %v", err)
	}
	if !other.Blocked {
		t.Fatalf("other-session check = %#v, want blocked", other)
	}
	if other.Reason != "missing" {
		t.Fatalf("other-session check = %#v, want missing", other)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package a\nfunc B() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale, err := CheckReview(root, "demo", "session-a", []string{"pkg/a.go"})
	if err != nil {
		t.Fatalf("stale check: %v", err)
	}
	if !stale.Blocked {
		t.Fatalf("stale check = %#v, want blocked", stale)
	}
	if stale.Reason != "stale" {
		t.Fatalf("stale check = %#v, want stale", stale)
	}
}

func TestRunningModelSkipsSidechainMessages(t *testing.T) {
	// VALIDATES: the last main-thread model wins even when a later subagent line
	// names a different model.
	// PREVENTS: a helper model answering for the session that spawned it.
	path := filepath.Join(t.TempDir(), "session.jsonl")
	body := "{\"message\":{\"model\":\"claude-opus-5\"}}\n" +
		"{\"isSidechain\":true,\"message\":{\"model\":\"claude-sonnet-4\"}}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := RunningModel(path); got != "claude-opus-5" {
		t.Fatalf("RunningModel = %q", got)
	}
}

func TestModelCommandUsesPayloadTranscriptInsteadOfANewerNeighbour(t *testing.T) {
	// VALIDATES: a hook payload's explicit transcript is the exact session read,
	// even when ambient discovery would select a newer neighbouring session.
	// PREVENTS: dropping transcript_path and attributing another session's model
	// to the hook invocation.
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "")
	unsetDirectSessionID(t)

	dir := transcriptDir(root)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	same := filepath.Join(dir, "same.jsonl")
	neighbour := filepath.Join(dir, "neighbour.jsonl")
	if err := os.WriteFile(same, []byte("{\"message\":{\"model\":\"claude-opus-5-same\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(neighbour, []byte("{\"message\":{\"model\":\"claude-sonnet-newer\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(same, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(neighbour, now, now); err != nil {
		t.Fatal(err)
	}

	payload, code := answerModel(root, []string{"current", "transcript", same})
	if code != 0 {
		t.Fatalf("explicit transcript = (%#v, %d)", payload, code)
	}
	report, ok := payload.(modelReport)
	if !ok {
		t.Fatalf("explicit transcript payload = %T", payload)
	}
	if report.Transcript != same {
		t.Fatalf("transcript = %q, want exact %q", report.Transcript, same)
	}
	if report.Model != "claude-opus-5-same" {
		t.Fatalf("model = %q, want same-session model", report.Model)
	}

	unknown := filepath.Join(dir, "unknown.jsonl")
	if err := os.WriteFile(unknown, []byte("{\"message\":{}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, code = answerModel(root, []string{"current", "transcript", unknown})
	if code != 1 {
		t.Fatalf("unknown explicit transcript = (%#v, %d)", payload, code)
	}

	for _, args := range [][]string{
		{"current", "transcript"},
		{"current", "transcript", "relative.jsonl"},
		{"current", "transcript", filepath.Join(root, "absent.jsonl")},
		{"current", "transcript", root},
	} {
		if _, code := answerModel(root, args); code != 2 {
			t.Errorf("unsafe transcript args %#v answered %d, want 2", args, code)
		}
	}
}

func unsetDirectSessionID(t *testing.T) {
	t.Helper()
	old, present := os.LookupEnv("CLAUDE_CODE_SESSION_ID")
	if err := os.Unsetenv("CLAUDE_CODE_SESSION_ID"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var err error
		if present {
			err = os.Setenv("CLAUDE_CODE_SESSION_ID", old)
		} else {
			err = os.Unsetenv("CLAUDE_CODE_SESSION_ID")
		}
		if err != nil {
			t.Errorf("restore CLAUDE_CODE_SESSION_ID: %v", err)
		}
	})
}

func TestReviewModelBoundarySpeaksWhenItCannotEnforce(t *testing.T) {
	// VALIDATES: an unreadable model is fail-speak rather than fail-open in
	// silence, while a known implementation-tier model is refused.
	// PREVENTS: review evidence being attributed to the wrong model without a
	// warning or an operator-authored override.
	root := reviewFixture(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "../unreadable")
	request := reviewRecord{
		Spec: "demo", Verdict: "clean", Rounds: 1,
		Files: []string{"pkg/a.go"}, SessionID: "session",
	}
	artifact, err := recordReview(root, request)
	if err != nil {
		t.Fatalf("unreadable model: %v", err)
	}
	if len(artifact.Warnings) != 1 {
		t.Fatalf("unreadable-model warnings = %#v", artifact.Warnings)
	}
	if !strings.Contains(artifact.Warnings[0], "UNCHECKED") {
		t.Fatalf("unreadable-model warnings = %#v", artifact.Warnings)
	}

	request.Model = "claude-sonnet-4"
	_, err = recordReview(root, request)
	if err == nil {
		t.Fatal("implementation-tier review was accepted")
	}
	if !strings.Contains(err.Error(), "BLOCKED") {
		t.Fatalf("implementation-tier refusal = %v", err)
	}
	request.ModelOverride = "operator approved emergency review"
	artifact, err = recordReview(root, request)
	if err != nil {
		t.Fatalf("model override: %v", err)
	}
	if len(artifact.Warnings) != 1 {
		t.Fatalf("override warnings = %#v", artifact.Warnings)
	}
	if !strings.Contains(artifact.Warnings[0], "Operator reason") {
		t.Fatalf("override warnings = %#v", artifact.Warnings)
	}
}

func TestReviewCommandUsesTheOwnerAuthorisedKeyword(t *testing.T) {
	// VALIDATES: the grouped command maps the historical British-spelled
	// owner-authorised interface and rejects the drifted American spelling.
	// PREVENTS: round-six closure callers losing the owner refusal at cutover.
	args := []string{
		"spec", "demo", "verdict", "clean", "rounds", "6",
		"file", "pkg/a.go", "rounds-reason", "product defect",
		"owner-authorised", "Thomas approved one more pass",
	}
	record, err := parseReviewRecord(args)
	if err != nil {
		t.Fatalf("parseReviewRecord: %v", err)
	}
	if record.OwnerAuthorised == "" {
		t.Fatal("owner-authorised value was not mapped")
	}
	args[len(args)-2] = "owner-authorized"
	if _, err := parseReviewRecord(args); err == nil {
		t.Fatal("American-spelled owner keyword was accepted")
	}
	spec, files, err := parseReviewCheck([]string{"spec", "demo", "file", "pkg/a.go"})
	if err != nil {
		t.Fatalf("parseReviewCheck: %v", err)
	}
	if spec != "demo" {
		t.Fatalf("review check spec = %q", spec)
	}
	if len(files) != 1 {
		t.Fatalf("review check files = %#v", files)
	}
	if files[0] != "pkg/a.go" {
		t.Fatalf("review check files = %#v", files)
	}
}

func TestReviewCommandMapsRecordAndCheck(t *testing.T) {
	// VALIDATES: the grouped native command maps review_gate.py record and check
	// to the same session-scoped artifact.
	// PREVENTS: final integration exposing hashing while closure cannot record
	// or consume the resulting review.
	root := reviewFixture(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "../unreadable")
	payload, code := answerReview(root, "session", []string{
		"record", "spec", "demo", "verdict", "clean", "rounds", "1",
		"file", "pkg/a.go",
	})
	if code != 0 {
		t.Fatalf("review record = (%#v, %d)", payload, code)
	}
	payload, code = answerReview(root, "session", []string{
		"check", "spec", "demo", "file", "pkg/a.go",
	})
	check, ok := payload.(ReviewCheck)
	if code != 0 {
		t.Fatalf("review check = (%#v, %d)", payload, code)
	}
	if !ok {
		t.Fatalf("review check payload = %T", payload)
	}
	if check.Blocked {
		t.Fatalf("review check = %#v, want clean", check)
	}
}

func TestReviewRejectsAnUnsafeSessionID(t *testing.T) {
	// VALIDATES: public artifact APIs reject a session component that escapes
	// tmp/review.
	// PREVENTS: a caller-controlled session ID overwriting an unrelated file.
	root := reviewFixture(t)
	request := reviewRecord{
		Spec: "demo", Verdict: "clean", Rounds: 1,
		Files: []string{"pkg/a.go"}, Model: "claude-opus-5", SessionID: "../other",
	}
	if _, err := recordReview(root, request); err == nil {
		t.Fatal("recordReview accepted an unsafe session ID")
	}
	if _, err := CheckReview(root, "demo", "../other", nil); err == nil {
		t.Fatal("CheckReview accepted an unsafe session ID")
	}
}

func reviewFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "pkg", "a.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package a\nfunc A() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}
