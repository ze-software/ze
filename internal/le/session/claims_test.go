// VALIDATES: the claim enumerator gives every spec claim marker its holder and
// one of three verdicts from the reaper's judgement, and the reaper removes the
// markers of a dead session that owns no directory while keeping every marker
// whose holder is live or cannot be judged.
// PREVENTS: claim markers leaking for every session whose directory is gone,
// and the opposite failure, a live or unjudged session's claim destroyed.
package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/procscan"
)

// claimWorld is a checkout with tmp/session, a Claude config directory with
// one project, and a Claude CLI that started at started.
type claimWorld struct {
	root     string
	sessions string
	config   string
	projects string
	started  time.Time
	cli      procscan.Process
}

func newClaimWorld(t *testing.T) claimWorld {
	t.Helper()
	root := t.TempDir()
	world := claimWorld{
		root:     root,
		sessions: filepath.Join(root, "tmp", "session"),
		config:   filepath.Join(root, "config"),
		started:  time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC),
	}
	world.projects = filepath.Join(world.config, "projects", "fixture")
	for _, dir := range []string{world.sessions, world.projects} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	world.cli = procscan.Process{PID: 45, Start: "456", StartedAt: world.started, Argv: []string{"/usr/bin/claude"}, CLI: true}
	return world
}

// marker writes one flat file under tmp/session holding body, last written
// twice IdleAge ago, so only the proofs of life keep its session alive.
func (w claimWorld) marker(t *testing.T, name, body string) string {
	t.Helper()
	path := w.freshMarker(t, name, body)
	ageTree(t, path, 2*IdleAge)
	return path
}

// freshMarker writes one flat file under tmp/session holding body, now.
func (w claimWorld) freshMarker(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(w.sessions, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertPresent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was removed: %v", filepath.Base(path), err)
		}
	}
}

func assertRemoved(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s remains: %v", filepath.Base(path), err)
		}
	}
}

// claimsBySession indexes a judgement's claims by session and agent.
func claimsBySession(claims []Claim) map[string]Claim {
	index := make(map[string]Claim, len(claims))
	for _, claim := range claims {
		index[claim.Session+"+"+claim.Agent] = claim
	}
	return index
}

// VALIDATES: a dead session that owns no dated directory loses its claim
// marker, its agents' claim markers and its other flat markers.
// Method: three markers of "gone", no directory for it, a running Claude CLI
// and a transcript of "gone" older than the CLI; reap removes all three.
func TestAMarkerWithNoSessionDirectoryIsReaped(t *testing.T) {
	world := newClaimWorld(t)
	claim := world.marker(t, ".session-gone", "spec-a.md\n")
	agentClaim := world.marker(t, ".session-gone+agent1", "spec-b.md\n")
	other := world.marker(t, ".lsp-loaded-gone", "x")
	writeTranscriptFixture(t, world.projects, "gone", world.started.Add(-time.Minute))

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps([]procscan.Process{world.cli}))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 0 || report.RemovedMarkers != 3 || report.UndecidedClaims != 0 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertRemoved(t, claim, agentClaim, other)
}

// VALIDATES: a claim marker whose session owns no directory is kept when any
// proof of life names its session: this process's own id, a live pid pin, a
// running process's arguments, a transcript written since the oldest CLI
// started.
// Method: one directory-less marker per proof beside one dead one; reap
// removes the dead one alone, which proves the reaper judged them all.
func TestALiveSessionsMarkerIsKept(t *testing.T) {
	world := newClaimWorld(t)
	own := world.marker(t, ".session-own", "spec-own.md\n")
	pinned := world.marker(t, ".session-pinned", "spec-pinned.md\n")
	argv := world.marker(t, ".session-argv-live", "spec-argv.md\n")
	transcript := world.marker(t, ".session-transcript-live", "spec-transcript.md\n")
	gone := world.marker(t, ".session-gone", "spec-gone.md\n")
	world.marker(t, ".sid-by-pid-44-123", "pinned\n")
	writeTranscriptFixture(t, world.projects, "transcript-live", world.started.Add(time.Minute))
	processes := []procscan.Process{
		world.cli,
		{PID: 44, Start: "123", Argv: []string{"worker"}},
		{PID: 46, Start: "789", Argv: []string{"tool", "--session-id", "argv-live"}},
	}

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedMarkers != 1 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertPresent(t, own, pinned, argv, transcript)
	assertRemoved(t, gone)
}

// VALIDATES: a marker whose holder's liveness cannot be told is kept and
// reported, never removed: with a CLI running and no projects directory (the
// reaper's notice), and with no Claude CLI in the process table at all.
func TestAMarkerIsKeptWhenLivenessCannotBeTold(t *testing.T) {
	t.Run("no projects directory", func(t *testing.T) {
		world := newClaimWorld(t)
		gone := world.marker(t, ".session-gone", "spec-gone.md\n")
		processes := []procscan.Process{world.cli}
		missing := filepath.Join(world.root, "missing-config")

		report, err := reap(world.root, missing, "own", false, fixtureReapOps(processes))
		if err != nil {
			t.Fatal(err)
		}
		if report.Notice == "" || report.UndecidedClaims != 1 {
			t.Fatalf("report = %#v", report)
		}
		if !strings.Contains(report.Text(), "kept 1 spec claim marker whose holder cannot be judged") {
			t.Errorf("text %q does not report the kept claim", report.Text())
		}
		assertPresent(t, gone)

		t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
		judgement, err := Judge(world.root, missing, processes)
		if err != nil {
			t.Fatal(err)
		}
		if len(judgement.Claims) != 1 || judgement.Claims[0].Liveness != LivenessCannotTell ||
			judgement.Claims[0].Reason != judgement.Notice {
			t.Fatalf("claims under the notice = %#v", judgement.Claims)
		}
		if len(judgement.Live) != 0 {
			t.Errorf("Live under the notice = %v, want empty", judgement.Live)
		}
	})
	t.Run("no Claude CLI", func(t *testing.T) {
		world := newClaimWorld(t)
		gone := world.marker(t, ".session-gone", "spec-gone.md\n")

		report, err := reap(world.root, world.config, "own", false, fixtureReapOps(nil))
		if err != nil {
			t.Fatal(err)
		}
		if report.Notice != noCLINotice || report.RemovedMarkers != 0 || report.UndecidedClaims != 1 {
			t.Fatalf("report = %#v, text %q", report, report.Text())
		}
		if !strings.Contains(report.Text(), "kept 1 spec claim marker whose holder cannot be judged") {
			t.Errorf("text %q does not report the kept claim", report.Text())
		}
		assertPresent(t, gone)
	})
}

// VALIDATES: a session no proof of life names that wrote within IdleAge
// answers cannot tell, not dead, and keeps its directory and its markers:
// for a directory, by the newest write anywhere in its tree, and for a
// session with no directory, by its own flat markers.
// PREVENTS: a session the proof cannot see (an identity from the environment,
// another CLAUDE_CONFIG_DIR, another machine) losing its claim while it runs.
func TestARecentlyWrittenSessionIsNotDead(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
	world := newClaimWorld(t)
	busy := writeSessionFixture(t, world.sessions, "busy")
	state := filepath.Join(busy, "state", "session-state.md")
	if err := os.MkdirAll(filepath.Dir(state), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	busyClaim := world.marker(t, ".session-busy", "spec-busy.md\n")
	quietClaim := world.freshMarker(t, ".session-quiet", "spec-quiet.md\n")
	processes := []procscan.Process{world.cli}

	judgement, err := Judge(world.root, world.config, processes)
	if err != nil {
		t.Fatal(err)
	}
	if len(judgement.Dead) != 0 {
		t.Errorf("Dead = %v, want none", judgement.Dead)
	}
	claims := claimsBySession(judgement.Claims)
	for _, key := range []string{"busy+", "quiet+"} {
		claim := claims[key]
		if claim.Liveness != LivenessCannotTell || !strings.Contains(claim.Reason, "written within") {
			t.Errorf("claim %s = %#v, want cannot tell for a recent write", key, claim)
		}
	}

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 0 || report.RemovedMarkers != 0 || report.UndecidedClaims != 2 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertPresent(t, busy, state, busyClaim, quietClaim)
}

// VALIDATES: a session that owns a directory idle for twice IdleAge, and wrote
// one of its own flat markers within IdleAge, answers cannot tell, not dead:
// the reap removes neither its directory nor its marker, and counts the
// directory undecided by the recent-marker cause.
// PREVENTS: the directory rule reading less evidence than the directory-less
// one, so a session the proof cannot see, whose only recent write is a flat
// marker, losing its directory and its claim while it runs.
// Method: one aged session directory, its claim marker written now, a Claude
// CLI running; Judge and reap judge it.
func TestADirectoryOwnerWithARecentMarkerIsNotDead(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
	world := newClaimWorld(t)
	dir := writeSessionFixture(t, world.sessions, "quietowner")
	claim := world.freshMarker(t, ".session-quietowner", "spec-quiet.md\n")
	processes := []procscan.Process{world.cli}

	judgement, err := Judge(world.root, world.config, processes)
	if err != nil {
		t.Fatal(err)
	}
	if len(judgement.Dead) != 0 {
		t.Errorf("Dead = %v, want none", judgement.Dead)
	}
	if got := claimsBySession(judgement.Claims)["quietowner+"]; got.Liveness != LivenessCannotTell ||
		!strings.Contains(got.Reason, "written within") {
		t.Errorf("claim = %#v, want cannot tell for a recent marker", got)
	}
	if len(judgement.Undecided) != 1 || judgement.Undecided[0].Path != dir ||
		judgement.Undecided[0].Cause != UndecidedRecentMarker {
		t.Errorf("Undecided = %#v, want %s kept for a recent marker", judgement.Undecided, dir)
	}

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	want := UndecidedCount{Cause: UndecidedRecentMarker, Count: 1}
	if report.RemovedDirs != 0 || report.RemovedMarkers != 0 || len(report.Undecided) != 1 ||
		report.Undecided[0] != want {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertPresent(t, dir, claim)
}

// VALIDATES: with no Claude CLI in the process table the reaper refuses to
// judge: no directory, claim marker or stale pin is removed, and the report
// says why and what to run instead.
// PREVENTS: a QEMU guest or a container, which sees the checkout and none of
// its sessions, calling every session dead.
func TestReapRemovesNothingWithoutAClaudeCLI(t *testing.T) {
	world := newClaimWorld(t)
	dead := writeSessionFixture(t, world.sessions, "dead")
	gone := world.marker(t, ".session-gone", "spec-gone.md\n")
	pin := world.marker(t, ".sid-by-pid-44-old", "dead\n")

	report, err := reap(world.root, world.config, "own", false,
		fixtureReapOps([]procscan.Process{{PID: 44, Start: "new", Argv: []string{"worker"}}}))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 0 || report.RemovedMarkers != 0 || report.Notice != noCLINotice {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	for _, want := range []string{"no Claude CLI", "Removed nothing", "./le session clean"} {
		if !strings.Contains(report.Text(), want) {
			t.Errorf("text %q does not say %q", report.Text(), want)
		}
	}
	assertPresent(t, dead, gone, pin)
}

// VALIDATES: a marker of a session not proven dead (here one whose claim
// marker cannot be read) is kept when a dead session's id is the tail of its
// own ("bar" and "foo-bar").
func TestAMarkerOfASessionNotProvenDeadIsKeptWhenADeadIDIsItsTail(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a file whatever its mode, so the marker cannot be made unreadable")
	}
	world := newClaimWorld(t)
	undecided := world.marker(t, ".session-foo-bar", "spec-undecided.md\n")
	if err := os.Chmod(undecided, 0o000); err != nil {
		t.Fatal(err)
	}
	dead := world.marker(t, ".session-bar", "spec-dead.md\n")

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps([]procscan.Process{world.cli}))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedMarkers != 1 || report.UndecidedClaims != 1 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertPresent(t, undecided)
	assertRemoved(t, dead)
}

// VALIDATES: a claim marker this process cannot read is kept and answers
// cannot tell with the read error: another account on the host most likely
// wrote it, and that account's transcripts were not read.
func TestAnUnreadableClaimMarkerIsKept(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a file whatever its mode, so the marker cannot be made unreadable")
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
	world := newClaimWorld(t)
	foreign := world.marker(t, ".session-other-account", "spec-foreign.md\n")
	if err := os.Chmod(foreign, 0o000); err != nil {
		t.Fatal(err)
	}
	processes := []procscan.Process{world.cli}

	judgement, err := Judge(world.root, world.config, processes)
	if err != nil {
		t.Fatal(err)
	}
	if len(judgement.Claims) != 1 || judgement.Claims[0].Liveness != LivenessCannotTell ||
		judgement.Claims[0].Unread == "" || judgement.Claims[0].Spec != "" {
		t.Fatalf("claims = %#v, want one unread claim that cannot be told", judgement.Claims)
	}
	report, err := reap(world.root, world.config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedMarkers != 0 || report.UndecidedClaims != 1 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertPresent(t, foreign)
}

// VALIDATES: the enumerator answers live, dead and cannot tell, each with its
// holder and a reason, and leaves out a marker that claims nothing.
// Method: this session's claim (live), a dead directory-less session's claim
// (dead), an agent's claim under this live session (cannot tell: the marker
// carries no signal of the agent), and an "unassigned" marker.
func TestTheEnumeratorAnswersEachVerdict(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
	world := newClaimWorld(t)
	world.marker(t, ".session-own", "spec-live.md\n")
	world.marker(t, ".session-gone", "spec-dead.md\n")
	world.marker(t, ".session-own+agent1", "spec-agent.md\n")
	world.marker(t, ".session-idle", "unassigned\n")
	world.marker(t, ".lsp-loaded-own", "x")

	judgement, err := Judge(world.root, world.config, []procscan.Process{world.cli})
	if err != nil {
		t.Fatal(err)
	}
	if len(judgement.Claims) != 3 {
		t.Fatalf("claims = %#v, want three", judgement.Claims)
	}
	claims := claimsBySession(judgement.Claims)
	want := map[string]struct {
		spec     string
		liveness Liveness
	}{
		"own+":       {"spec-live.md", LivenessLive},
		"gone+":      {"spec-dead.md", LivenessDead},
		"own+agent1": {"spec-agent.md", LivenessCannotTell},
	}
	for key, expected := range want {
		claim, found := claims[key]
		if !found {
			t.Errorf("no claim for %s in %#v", key, judgement.Claims)
			continue
		}
		if claim.Spec != expected.spec || claim.Liveness != expected.liveness || claim.Reason == "" {
			t.Errorf("claim %s = %#v, want spec %s and %s with a reason", key, claim, expected.spec, expected.liveness)
		}
	}
	if strings.Join(judgement.Live, ",") != "own" {
		t.Errorf("Live = %v, want [own]", judgement.Live)
	}
}

// VALIDATES: an agent's claim whose parent session is dead answers dead, with
// or without the parent's directory, and the reaper removes both markers.
func TestAnAgentClaimUnderADeadParentIsDead(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
	world := newClaimWorld(t)
	writeSessionFixture(t, world.sessions, "dead")
	withDirectory := world.marker(t, ".session-dead+agent1", "spec-one.md\n")
	withoutDirectory := world.marker(t, ".session-gone+agent2", "spec-two.md\n")
	processes := []procscan.Process{world.cli}

	judgement, err := Judge(world.root, world.config, processes)
	if err != nil {
		t.Fatal(err)
	}
	claims := claimsBySession(judgement.Claims)
	for _, key := range []string{"dead+agent1", "gone+agent2"} {
		if claim := claims[key]; claim.Liveness != LivenessDead {
			t.Errorf("claim %s = %#v, want dead", key, claim)
		}
	}

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 1 || report.RemovedMarkers != 2 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertRemoved(t, withDirectory, withoutDirectory)
}

// VALIDATES: a marker a live session owns is kept when a dead session's id is
// the tail of the live one's ("bar" and "foo-bar"), which MarkerOwnedBy's
// suffix match would otherwise hand to the dead one.
func TestAMarkerALiveSessionOwnsIsKeptWhenADeadIDIsItsTail(t *testing.T) {
	world := newClaimWorld(t)
	live := world.marker(t, ".session-foo-bar", "spec-live.md\n")
	dead := world.marker(t, ".session-bar", "spec-dead.md\n")
	writeTranscriptFixture(t, world.projects, "foo-bar", world.started.Add(time.Minute))

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps([]procscan.Process{world.cli}))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedMarkers != 1 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	assertPresent(t, live)
	assertRemoved(t, dead)
}
