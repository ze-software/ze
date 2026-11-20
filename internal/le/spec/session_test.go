package spec

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	"github.com/ze-software/ze/internal/le/spec/design/designtest"
)

const readySpecFixture = `# Spec

| Field | Value |
|-------|-------|
| Status | ready |
| Updated | 2026-08-01 |

## Goal
`

func TestConcurrentClaimsCannotExceedTheWIPCap(t *testing.T) {
	// VALIDATES: the WIP count, ready transition, and marker publication share
	// one repository lock.
	// PREVENTS: two sessions both observing one free slot and opening two specs.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"spec-a.md", "spec-b.md"} {
		if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", name), []byte(readySpecFixture), 0o600); err != nil {
			t.Fatal(err)
		}
		// Both designs carry Thomas's approval, so the claim gate admits both
		// and the cap alone decides which claim is refused.
		seedReadyApproval(t, root, "plan/post-release/engineering/"+name)
	}
	owners := []specOwner{
		{Root: root, SessionID: "session-a", WIPCap: 1, Now: func() time.Time { return time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC) }},
		{Root: root, SessionID: "session-b", WIPCap: 1, Now: func() time.Time { return time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC) }},
	}
	results := make([]ClaimReport, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for index := range owners {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errs[index] = owners[index].Claim([]string{"spec-a.md", "spec-b.md"}[index])
		}(index)
	}
	wait.Wait()
	refused := 0
	transitioned := 0
	for index := range results {
		if errs[index] != nil {
			t.Fatalf("claim %d: %v", index, errs[index])
		}
		if results[index].Refused {
			refused++
		}
		if results[index].Transitioned {
			transitioned++
		}
	}
	if refused != 1 || transitioned != 1 {
		t.Fatalf("results = %#v, want one transition and one refusal", results)
	}
	wip, err := wip(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(wip.Specs) != 1 {
		t.Fatalf("wip = %#v, want exactly one spec", wip.Specs)
	}
}

func TestConcurrentSessionsKeepIndependentMarkers(t *testing.T) {
	// VALIDATES: two sessions may own the same in-progress spec without
	// overwriting each other's claim.
	// PREVENTS: a shared marker turning the second claim into ownership theft.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte("| Field | Value |\n|-------|-------|\n| Status | in-progress |\n| Updated | 2026-08-27 |\n")
	if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", "spec-a.md"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	owners := []specOwner{{Root: root, SessionID: "one"}, {Root: root, SessionID: "two"}}
	var wait sync.WaitGroup
	for index := range owners {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			if _, err := owners[index].Claim("spec-a.md"); err != nil {
				t.Errorf("claim: %v", err)
			}
		}(index)
	}
	wait.Wait()
	for _, owner := range owners {
		got, err := owner.currentSpec()
		if err != nil || got != "spec-a.md" {
			t.Errorf("currentSpec(%s) = %q, %v", owner.SessionID, got, err)
		}
	}
}

func TestMalformedClaimMarkerFailsClosed(t *testing.T) {
	// VALIDATES: a marker containing a path rather than a spec basename is
	// reported as malformed.
	// PREVENTS: state-path construction escaping the per-session state directory.
	root := t.TempDir()
	marker := filepath.Join(root, "tmp", "session", ".session-bad")
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("../../spec-other.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (specOwner{Root: root, SessionID: "bad"}).currentSpec(); err == nil {
		t.Fatal("currentSpec accepted a path-bearing marker")
	}
}

func TestSpecSessionCommandMapsEveryOwnershipAction(t *testing.T) {
	// VALIDATES: every member of the spec namespace (current, claim, wip,
	// state, model, review hash, release) answers through its own handler.
	// PREVENTS: final hook routing reaching an API that has no command grammar.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", "spec-a.md"), []byte(readySpecFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	// The design carries Thomas's approval, so the claim below reaches the
	// ready transition this test maps.
	seedReadyApproval(t, root, "plan/post-release/engineering/spec-a.md")
	setAnswerRoot(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "command-session")

	payload, code := Current(nil)
	if current, ok := payload.(currentReport); code != 0 || !ok || current.Spec != "" {
		t.Fatalf("current before claim = (%#v, %d)", payload, code)
	}
	payload, code = Claim([]string{"plan/post-release/engineering/spec-a.md"})
	if claim, ok := payload.(ClaimReport); code != 0 || !ok || !claim.Transitioned {
		t.Fatalf("claim = (%#v, %d)", payload, code)
	}
	payload, code = Current(nil)
	if current, ok := payload.(currentReport); code != 0 || !ok || current.Spec != "spec-a.md" {
		t.Fatalf("current = (%#v, %d)", payload, code)
	}
	if payload, code = WIP(nil); code != 0 {
		t.Fatalf("wip = (%#v, %d)", payload, code)
	}
	if payload, code = State([]string{"current"}); code != 0 {
		t.Fatalf("state current = (%#v, %d)", payload, code)
	}
	state, ok := payload.(statePathReport)
	if !ok || state.Path == "" {
		t.Fatalf("state current payload = %#v", payload)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(state.Path)), []byte("state\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if payload, code = State([]string{"latest", "spec", "a"}); code != 0 {
		t.Fatalf("state latest = (%#v, %d)", payload, code)
	}
	latest, ok := payload.(statePathReport)
	if !ok || latest.Path != state.Path {
		t.Fatalf("state latest payload = %#v, want %s", payload, state.Path)
	}
	if payload, code = Model([]string{"current"}); code != 1 {
		t.Fatalf("unreadable model = (%#v, %d), want advisory code 1", payload, code)
	}
	if payload, code = Review([]string{"hash", "file", "plan/post-release/engineering/spec-a.md"}); code != 0 {
		t.Fatalf("review hash = (%#v, %d)", payload, code)
	}
	if payload, code = Release(nil); code != 0 {
		t.Fatalf("release = (%#v, %d)", payload, code)
	}
	// `spec` before the file is the retired verb (D-P4-areaword): the
	// verbless table reads it as the spec and refuses the second value.
	if _, code = Claim([]string{"spec", "spec-a.md"}); code != 2 {
		t.Fatalf("claim accepted the retired spec selector before a value, code %d", code)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "../unreadable")
	if payload, code = Model([]string{"current"}); code != 1 {
		t.Fatalf("model fail-speak = (%#v, %d)", payload, code)
	}
	if payload, code = WIP(nil); code != 0 {
		t.Fatalf("wip depended on session ownership = (%#v, %d)", payload, code)
	}
	if spec, err := (specOwner{Root: root, SessionID: "command-session"}).currentSpec(); err != nil || spec != "" {
		t.Fatalf("released currentSpec = %q, %v", spec, err)
	}
}

// TestClaimRefusesReadySpecWithoutApproval is the wiring row "`./le spec
// claim` on a ready spec" (plan/release/validation/spec-design-needs-owner-approval.md,
// AC-22). Method: claim a ready spec that no approval record or trailer
// covers, with Thomas not away, through the registered command; it exits 2,
// Status stays ready, and the session claims nothing.
func TestClaimRefusesReadySpecWithoutApproval(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "plan", "post-release", "engineering", "spec-a.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(readySpecFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	// An empty history: the lookup reads no trailer rather than failing, so
	// the exit 2 below is the refusal and not a git error (AC-26).
	initFixtureRepository(t, root)
	setAnswerRoot(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "unapproved-claim")

	payload, code := Claim([]string{"plan/post-release/engineering/spec-a.md"})
	if code != 2 {
		t.Fatalf("a claim of a ready spec with no owner approval answered (%#v, %d), want exit 2", payload, code)
	}
	report, ok := payload.(ClaimReport)
	if !ok {
		t.Fatalf("the refused claim answered %#v, want the claim report carrying the refusal", payload)
	}
	requireClaimText(t, report.Text(), "refusing to start spec-a.md: its design has no owner approval, and only Thomas records one",
		"why: a design reaches implementation only once Thomas approved it",
		"./le spec design check spec plan/post-release/engineering/spec-a.md",
		`./le spec design approve spec plan/post-release/engineering/spec-a.md words "<what he said>"`)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "| Status | ready |") {
		t.Fatalf("the refused claim changed Status:\n%s", body)
	}
	if spec, err := (specOwner{Root: root, SessionID: "unapproved-claim"}).currentSpec(); err != nil || spec != "" {
		t.Fatalf("the refused claim left the session claiming %q (%v)", spec, err)
	}
}

// claimFixtureDay is the day every approval fixture claims on.
var claimFixtureDay = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

// seedReadyApproval records Thomas's approval of the spec at relative as it
// stands on disk under root, dated 2026-11-19.
func seedReadyApproval(t *testing.T, root, relative string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	if err := designtest.SeedApproval(root, relative, string(body), "2026-11-19", "go ahead"); err != nil {
		t.Fatal(err)
	}
}

// initFixtureRepository makes root a git repository with no commit.
func initFixtureRepository(t *testing.T, root string) {
	t.Helper()
	if err := designtest.InitRepository(root); err != nil {
		t.Fatal(err)
	}
}

// runFixtureGit runs one git command in root as a fixture author.
func runFixtureGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if err := designtest.Git(root, args...); err != nil {
		t.Fatal(err)
	}
}

// writeClaimSpec writes text at relative under root.
func writeClaimSpec(t *testing.T, root, relative, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// requireClaimText fails for each want text does not hold.
func requireClaimText(t *testing.T, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("the claim report does not say %q:\n%s", want, text)
		}
	}
}

// requireStillReady fails unless the spec at relative is still ready and the
// owner claims nothing: what every refused claim leaves.
func requireStillReady(t *testing.T, owner specOwner, relative string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(owner.Root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "| Status | ready |") {
		t.Errorf("the refused claim changed Status:\n%s", body)
	}
	if spec, err := owner.currentSpec(); err != nil || spec != "" {
		t.Errorf("the refused claim left the session claiming %q (%v)", spec, err)
	}
}

// approvedSpecText is a ready spec with the four sections the design digest
// covers.
const approvedSpecText = "# Spec: a\n\n| Field | Value |\n|-------|-------|\n| Status | ready |\n| Updated | 2026-11-19 |\n\n" +
	"## Task\n\nRender the widget.\n\n## Acceptance Criteria\n\n| AC ID | Input | Expected |\n|---|---|---|\n| AC-1 | x | y |\n"

// TestClaimAdmitsMatchingApproval is AC-23. Method: record Thomas's approval
// of the spec's text, claim it; it moves to in-progress and the report names
// his approval.
func TestClaimAdmitsMatchingApproval(t *testing.T) {
	root := t.TempDir()
	relative := "plan/post-release/engineering/spec-a.md"
	writeClaimSpec(t, root, relative, approvedSpecText)
	seedReadyApproval(t, root, relative)
	owner := specOwner{Root: root, SessionID: "approved", WIPCap: 12, Now: func() time.Time { return claimFixtureDay }}
	report, err := owner.Claim("spec-a.md")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Transitioned {
		t.Fatalf("the approved spec did not start: %#v", report)
	}
	requireClaimText(t, report.Text(), "ready -> in-progress", `approved: 2026-11-19 "go ahead"`)
}

// TestClaimRefusesStaleApproval is AC-24. Method: approve one text, change
// the Task, and claim: refused naming the approval's date, Status ready, no
// marker. Then approve again and change only the Updated row, outside the
// four sections: the claim proceeds.
func TestClaimRefusesStaleApproval(t *testing.T) {
	root := t.TempDir()
	initFixtureRepository(t, root)
	relative := "plan/post-release/engineering/spec-a.md"
	writeClaimSpec(t, root, relative, approvedSpecText)
	seedReadyApproval(t, root, relative)
	writeClaimSpec(t, root, relative, strings.Replace(approvedSpecText, "Render the widget.", "Render the widget twice.", 1))
	owner := specOwner{Root: root, SessionID: "stale", WIPCap: 12, Now: func() time.Time { return claimFixtureDay }}
	report, err := owner.Claim("spec-a.md")
	if err != nil {
		t.Fatal(err)
	}
	if report.Transitioned {
		t.Fatal("a spec whose Task changed after the approval started")
	}
	requireClaimText(t, report.Text(), "refusing to start spec-a.md: its design changed after Thomas approved it on 2026-11-19",
		"./le spec design check spec "+relative, "./le spec design approve spec "+relative)
	requireStillReady(t, owner, relative)

	writeClaimSpec(t, root, relative, approvedSpecText)
	seedReadyApproval(t, root, relative)
	writeClaimSpec(t, root, relative, strings.Replace(approvedSpecText, "| Updated | 2026-11-19 |", "| Updated | 2026-11-20 |", 1))
	if report, err := owner.Claim("spec-a.md"); err != nil || !report.Transitioned {
		t.Fatalf("a change outside the four sections staled the approval: %#v, %v", report, err)
	}
}

// TestClaimAdmitsApprovalTrailer is AC-25. Method: no local record; a commit
// carries the spec and a Design-approved: line with its stem and digest, as
// FormatApprovalTrailer spells it; the claim proceeds and names the approval.
func TestClaimAdmitsApprovalTrailer(t *testing.T) {
	root := t.TempDir()
	initFixtureRepository(t, root)
	relative := "plan/post-release/engineering/spec-a.md"
	writeClaimSpec(t, root, relative, approvedSpecText)
	trailer := specdesign.FormatApprovalTrailer(specdesign.Approval{Stem: "spec-a",
		Digest: specdesign.DesignDigest(approvedSpecText), Date: "2026-11-18", Words: "approved on the other machine"})
	runFixtureGit(t, root, "add", "--", relative)
	runFixtureGit(t, root, "commit", "-q", "-m", "the approved spec\n\n"+trailer)
	owner := specOwner{Root: root, SessionID: "trailer", WIPCap: 12, Now: func() time.Time { return claimFixtureDay }}
	report, err := owner.Claim("spec-a.md")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Transitioned {
		t.Fatalf("a spec approved by a reachable trailer did not start: %#v", report)
	}
	requireClaimText(t, report.Text(), `approved: 2026-11-18 "approved on the other machine"`)
}

// TestClaimFailsClosedOnUnreadableRecord is AC-26. Method: an approval record
// with no Digest line, then no record in a tree with no git history; each
// claim answers an error naming the record or git, and starts nothing.
func TestClaimFailsClosedOnUnreadableRecord(t *testing.T) {
	relative := "plan/post-release/engineering/spec-a.md"
	record := t.TempDir()
	writeClaimSpec(t, record, relative, approvedSpecText)
	writeClaimSpec(t, record, "tmp/spec-design/approved/spec-a.md", "Spec: spec-a.md\n")
	owner := specOwner{Root: record, SessionID: "unreadable", WIPCap: 12, Now: func() time.Time { return claimFixtureDay }}
	if _, err := owner.Claim("spec-a.md"); err == nil || !strings.Contains(err.Error(), "tmp/spec-design/approved/spec-a.md") {
		t.Errorf("a malformed approval record answered %v, want an error naming it", err)
	}
	requireStillReady(t, owner, relative)

	history := t.TempDir()
	writeClaimSpec(t, history, relative, approvedSpecText)
	owner = specOwner{Root: history, SessionID: "unreadable", WIPCap: 12, Now: func() time.Time { return claimFixtureDay }}
	if _, err := owner.Claim("spec-a.md"); err == nil || !strings.Contains(err.Error(), "git") {
		t.Errorf("an unreadable history answered %v, want an error naming git", err)
	}
	requireStillReady(t, owner, relative)
}

// TestClaimAwayAdmitsJudgedFixSpecOnly is AC-28 and AC-29. Method: Thomas
// away from 2026-11-19 to 2026-11-25. The judged clean fix spec starts and its
// report names the away admission. A spec outside the fixes category, one
// with a departs row, and one no judge approved are each refused with the
// away rule and their problem. On 2026-11-26 the absence has lapsed and the
// clean spec is refused as an unapproved one.
func TestClaimAwayAdmitsJudgedFixSpecOnly(t *testing.T) {
	awayRoot := func(t *testing.T, relative, text string, judged bool) string {
		t.Helper()
		root := t.TempDir()
		initFixtureRepository(t, root)
		if err := designtest.WriteCleanTree(root, relative, text); err != nil {
			t.Fatal(err)
		}
		if err := designtest.SeedAway(root, "2026-11-19", "2026-11-25"); err != nil {
			t.Fatal(err)
		}
		if judged {
			if err := designtest.SeedJudgment(root, relative, text, "approve"); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	claim := func(t *testing.T, root, relative string, day time.Time) (specOwner, ClaimReport) {
		t.Helper()
		owner := specOwner{Root: root, SessionID: "away", WIPCap: 12, Now: func() time.Time { return day }}
		report, err := owner.Claim(filepath.Base(relative))
		if err != nil {
			t.Fatal(err)
		}
		return owner, report
	}
	awayRule := "while Thomas is away, only a judged fix spec with no departure starts without his approval"

	root := awayRoot(t, designtest.CleanSpec, designtest.CleanSpecText, true)
	_, report := claim(t, root, designtest.CleanSpec, claimFixtureDay)
	if !report.Transitioned {
		t.Fatalf("the judged clean fix spec did not start while Thomas is away:\n%s", report.Text())
	}
	requireClaimText(t, report.Text(), "admitted while Thomas is away (2026-11-19 to 2026-11-25): no design approval; he reviews it on return")

	departing := strings.Replace(designtest.CleanSpecText, "| agrees |", "| departs |", 1)
	engineering := "plan/post-release/engineering/spec-widget.md"
	for _, refused := range []struct {
		name, relative, text, problem string
		judged                        bool
	}{
		{"outside the fixes category", engineering, designtest.CleanSpecText, "is not a fix spec", true},
		{"a departs row", designtest.CleanSpec, departing, "departs from docs/architecture/widget.md", true},
		{"no judge approval", designtest.CleanSpec, designtest.CleanSpecText, "no judge has judged this design", false},
	} {
		t.Run(refused.name, func(t *testing.T) {
			root := awayRoot(t, refused.relative, refused.text, refused.judged)
			owner, report := claim(t, root, refused.relative, claimFixtureDay)
			requireClaimText(t, report.Text(), "refusing to start spec-widget.md", awayRule, refused.problem)
			requireStillReady(t, owner, refused.relative)
		})
	}

	t.Run("lapsed", func(t *testing.T) {
		root := awayRoot(t, designtest.CleanSpec, designtest.CleanSpecText, true)
		owner, report := claim(t, root, designtest.CleanSpec, time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC))
		requireClaimText(t, report.Text(), "refusing to start spec-widget.md: its design has no owner approval")
		if strings.Contains(report.Text(), awayRule) {
			t.Errorf("a lapsed absence still applied the away rule:\n%s", report.Text())
		}
		requireStillReady(t, owner, designtest.CleanSpec)
	})
}

func TestStatePathRejectsAnUnsafeOwner(t *testing.T) {
	// VALIDATES: public lifecycle APIs reject a session ID that could escape the
	// marker or state directory.
	// PREVENTS: a malformed state owner writing outside tmp/session.
	root := t.TempDir()
	owner := specOwner{Root: root, SessionID: "../other"}
	if _, err := owner.currentSpec(); err == nil {
		t.Fatal("currentSpec accepted an unsafe session ID")
	}
	if _, err := owner.StateFile(); err == nil {
		t.Fatal("StateFile accepted an unsafe session ID")
	}
}

func setAnswerRoot(t *testing.T, root string) {
	t.Helper()
	old, present := os.LookupEnv("ZE_REPO_ROOT")
	if err := os.Setenv("ZE_REPO_ROOT", root); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	t.Cleanup(func() {
		var err error
		if present {
			err = os.Setenv("ZE_REPO_ROOT", old)
		} else {
			err = os.Unsetenv("ZE_REPO_ROOT")
		}
		if err != nil {
			t.Errorf("restore ZE_REPO_ROOT: %v", err)
		}
		env.ResetCache()
	})
}

// VALIDATES: each agent of one session owns its claim, an agent with no claim
// of its own answers its parent's, and an agent's release removes only its own
// claim.
// PREVENTS: the shared marker, where the last sibling to claim won and one
// sibling's release cleared every other one's claim
// (concurrent-session-corruption journal, 2026-08-14 and 2026-09-05).
func TestAgentClaimsAreTheirOwnAndFallBackToTheParent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte("| Field | Value |\n|-------|-------|\n| Status | in-progress |\n| Updated | 2026-08-27 |\n")
	for _, name := range []string{"spec-parent.md", "spec-one.md", "spec-two.md"} {
		if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	parent := specOwner{Root: root, SessionID: "sid"}
	one := specOwner{Root: root, SessionID: "sid", AgentID: "one"}
	two := specOwner{Root: root, SessionID: "sid", AgentID: "two"}
	for owner, spec := range map[*specOwner]string{&parent: "spec-parent.md", &one: "spec-one.md"} {
		if _, err := owner.Claim(spec); err != nil {
			t.Fatalf("claim %s: %v", spec, err)
		}
	}
	current := func(owner specOwner, want string) {
		t.Helper()
		got, err := owner.currentSpec()
		if err != nil || got != want {
			t.Errorf("currentSpec(agent %q) = %q, %v; want %q", owner.AgentID, got, err, want)
		}
	}
	current(parent, "spec-parent.md")
	current(one, "spec-one.md")
	current(two, "spec-parent.md")
	if _, err := two.Claim("spec-two.md"); err != nil {
		t.Fatal(err)
	}
	current(one, "spec-one.md")
	current(two, "spec-two.md")
	current(parent, "spec-parent.md")
	if err := one.Release(); err != nil {
		t.Fatal(err)
	}
	current(one, "spec-parent.md")
	current(two, "spec-two.md")
	current(parent, "spec-parent.md")
}

// TestClaimedSpecFallsBackToTheParent pins the exported claim reader the
// commit tooling attributes a commit with.
// VALIDATES: ClaimedSpec resolves the ambient harness session and agent the
// way `./le spec claim` keys its marker, answers an agent's own claim, falls
// back to its parent's when it has none, and answers "" with no claim.
// PREVENTS: a second reader of the marker keyed by another identity, which
// finds nothing and attributes no commit.
func TestClaimedSpecFallsBackToTheParent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte("| Field | Value |\n|-------|-------|\n| Status | in-progress |\n| Updated | 2026-10-29 |\n")
	for _, name := range []string{"spec-parent.md", "spec-own.md"} {
		if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	as := func(agent, want string) {
		t.Helper()
		t.Setenv("CLAUDE_CODE_SESSION_ID", "claimed-sid")
		t.Setenv("ZE_AGENT_ID", agent)
		env.ResetCache()
		got, err := ClaimedSpec(root)
		if err != nil || got != want {
			t.Errorf("ClaimedSpec(agent %q) = %q, %v; want %q", agent, got, err, want)
		}
	}
	t.Cleanup(env.ResetCache)
	as("", "")
	if _, err := (specOwner{Root: root, SessionID: "claimed-sid"}).Claim("spec-parent.md"); err != nil {
		t.Fatal(err)
	}
	as("", "spec-parent.md")
	as("agent-a", "spec-parent.md")
	if _, err := (specOwner{Root: root, SessionID: "claimed-sid", AgentID: "agent-a"}).Claim("spec-own.md"); err != nil {
		t.Fatal(err)
	}
	as("agent-a", "spec-own.md")
	as("agent-b", "spec-parent.md")
}

// TestClaimFindsTheMovedIdentity preserves the stable marker when the source
// path disappears and the same spec is carried in a current category.
func TestClaimFindsTheMovedIdentity(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "tmp", "session", ".session-moved")
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("spec-moved.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "plan", "release", "fixes", "spec-moved.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(readySpecFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	// The design carries Thomas's approval, so the claim reaches the
	// transition whose marker this test pins.
	seedReadyApproval(t, root, "plan/release/fixes/spec-moved.md")
	owner := specOwner{Root: root, SessionID: "moved", WIPCap: 12}
	report, err := owner.Claim("spec-moved.md")
	if err != nil || !report.Transitioned {
		t.Fatalf("claim moved identity: %+v, %v", report, err)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "spec-moved.md\n" {
		t.Fatalf("stable marker = %q, %v", contents, err)
	}
}
