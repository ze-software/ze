// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
package lepath

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// runAs points the ambient harness identity at one session and, when agent is
// not empty, at one of its subagents, the way the pretool hook's prefix does.
func runAs(t *testing.T, session, agent string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", session)
	t.Setenv("ZE_AGENT_ID", agent)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

func commitSessionAs(t *testing.T, root, session, agent string) string {
	t.Helper()
	runAs(t, session, agent)
	namespace, err := CommitSession(root, "")
	if err != nil {
		t.Fatalf("CommitSession as %s/%s: %v", session, agent, err)
	}
	return namespace
}

// VALIDATES: two subagents of one parent each get a commit namespace of their
// own, distinct from the parent's and from each other's, and the same agent
// keeps its namespace across calls.
// PREVENTS: siblings sharing one weakened-test shard and one debt file, which
// let one sibling's commit prune or be refused by another's rows
// (plan/journal/concurrent-session-corruption.md, 2026-10-10).
func TestSiblingAgentsGetDistinctCommitSessions(t *testing.T) {
	root := t.TempDir()
	parent := commitSessionAs(t, root, "parent-sid", "")
	first := commitSessionAs(t, root, "parent-sid", "agent-one")
	second := commitSessionAs(t, root, "parent-sid", "agent-two")
	if first == second || first == parent || second == parent {
		t.Fatalf("namespaces parent=%s first=%s second=%s, want three distinct", parent, first, second)
	}
	if again := commitSessionAs(t, root, "parent-sid", "agent-one"); again != first {
		t.Fatalf("agent-one's namespace moved from %s to %s", first, again)
	}
	named, err := CommitSessionFor(root, "parent-sid", "agent-two")
	if err != nil || named != second {
		t.Fatalf("CommitSessionFor(agent-two) = %q, %v; want %s", named, err, second)
	}
}

// VALIDATES: a main thread answers exactly the namespaces of its own
// subagents, an unrelated session's agents are not among them, and a subagent
// answers none, so it can carry no sibling's shard and no parent's.
// PREVENTS: the parent being refused its own subagent's shard, and that
// allowance opening to an unrelated session.
func TestChildCommitSessionsAreThisMainThreadsAgentsOnly(t *testing.T) {
	root := t.TempDir()
	first := commitSessionAs(t, root, "parent-sid", "agent-one")
	second := commitSessionAs(t, root, "parent-sid", "agent-two")
	foreign := commitSessionAs(t, root, "other-sid", "agent-one")
	commitSessionAs(t, root, "parent-sid", "")

	children, err := ChildCommitSessions(root)
	if err != nil {
		t.Fatalf("ChildCommitSessions: %v", err)
	}
	want := []string{first, second}
	slices.Sort(want)
	if !slices.Equal(children, want) {
		t.Fatalf("children = %v, want %v", children, want)
	}
	if slices.Contains(children, foreign) {
		t.Fatalf("children %v carry the unrelated session's agent %s", children, foreign)
	}

	runAs(t, "parent-sid", "agent-one")
	children, err = ChildCommitSessions(root)
	if err != nil || len(children) != 0 {
		t.Fatalf("ChildCommitSessions under an agent = %v, %v; want none", children, err)
	}
}

// VALIDATES: an unsafe agent id and a harness id carrying the agent separator
// are refused rather than mapped to a namespace.
// PREVENTS: "../escape" naming a file outside tmp/, and "parent+x" posing as
// parent's subagent x.
func TestCommitSessionRefusesUnsafeAgentIdentity(t *testing.T) {
	root := t.TempDir()
	runAs(t, "parent-sid", "../escape")
	if _, err := CommitSession(root, ""); err == nil {
		t.Fatal("CommitSession accepted agent id ../escape")
	}
	if _, err := CommitSessionFor(root, "parent+x", ""); err == nil {
		t.Fatal("CommitSessionFor accepted a session id carrying the agent separator")
	}
	if _, err := CommitSessionFor(root, "parent", "x+y"); err == nil {
		t.Fatal("CommitSessionFor accepted an agent id carrying the agent separator")
	}
}

// VALIDATES: the (session, agent) encoding is injective. Session P-agent with
// agent x and session P with agent agent-x get distinct namespaces and distinct
// spec claim markers, and P's main thread does not count session P-agent's
// agents, or P-agent's own namespace, among its children.
// PREVENTS: the round-1 defect of the "-agent-" separator, where both pairs
// encoded to P-agent-agent-x and P's child glob matched P-agent's files.
func TestAgentIdentityEncodingIsInjective(t *testing.T) {
	root := t.TempDir()
	lookalike := commitSessionAs(t, root, "P-agent", "x")
	lookalikeMain := commitSessionAs(t, root, "P-agent", "")
	own := commitSessionAs(t, root, "P", "agent-x")
	if lookalike == own {
		t.Fatalf("P-agent/x and P/agent-x share namespace %s", own)
	}
	first, err := SpecClaimMarker(root, "P-agent", "x")
	if err != nil {
		t.Fatal(err)
	}
	second, err := SpecClaimMarker(root, "P", "agent-x")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("P-agent/x and P/agent-x share spec claim marker %s", first)
	}
	commitSessionAs(t, root, "P", "")
	children, err := ChildCommitSessions(root)
	if err != nil {
		t.Fatalf("ChildCommitSessions: %v", err)
	}
	if !slices.Equal(children, []string{own}) {
		t.Fatalf("P's children = %v, want only %s (not P-agent's %s or %s)", children, own, lookalike, lookalikeMain)
	}
}

// VALIDATES: a marker name belongs to its session and to that session's
// agents, and to no session whose id merely ends the same way.
// PREVENTS: the reaper leaving a dead session's agent markers behind, or
// removing a live session's.
func TestMarkerOwnedByMatchesSessionAndItsAgentsOnly(t *testing.T) {
	cases := []struct {
		name, session string
		want          bool
	}{
		{".session-dead", "dead", true},
		{".session-dead+agent-one", "dead", true},
		{".lsp-loaded-dead+a1", "dead", true},
		{".session-undead", "dead", false},
		{".session-dead+agent-one", "agent-one", false},
		{".session-P-agent+x", "P", false},
		{".session-dead+../x", "dead", false},
	}
	for _, c := range cases {
		if got := MarkerOwnedBy(c.name, c.session); got != c.want {
			t.Errorf("MarkerOwnedBy(%q, %q) = %v, want %v", c.name, c.session, got, c.want)
		}
	}
}

// VALIDATES: an agent's spec claim marker is its own file beside its parent's.
// PREVENTS: one agent's claim overwriting a sibling's or the parent's.
func TestSpecClaimMarkerIsPerAgent(t *testing.T) {
	root := t.TempDir()
	marker := func(agent string) string {
		t.Helper()
		path, err := SpecClaimMarker(root, "parent-sid", agent)
		if err != nil {
			t.Fatalf("SpecClaimMarker(%q): %v", agent, err)
		}
		return path
	}
	parent := marker("")
	if parent != filepath.Join(root, "tmp", "session", ".session-parent-sid") {
		t.Fatalf("parent marker = %s", parent)
	}
	first := marker("agent-one")
	second := marker("agent-two")
	if _, err := SpecClaimMarker(root, "parent-sid", "../x"); err == nil {
		t.Fatal("SpecClaimMarker accepted agent id ../x")
	}
	if first == parent || first == second {
		t.Fatalf("markers parent=%s first=%s second=%s, want three distinct", parent, first, second)
	}
}

// VALIDATES: SessionCommitSession answers the main thread's namespace from a
// subagent as from the main thread, so an owner's RFC approval recorded by
// either is found by the other.
// PREVENTS: per-agent namespaces hiding an approval from the commit or the
// edit it authorizes.
func TestSessionCommitSessionIsTheMainThreadsFromAnyAgent(t *testing.T) {
	root := t.TempDir()
	parent := commitSessionAs(t, root, "parent-sid", "")
	runAs(t, "parent-sid", "agent-one")
	got, err := SessionCommitSession(root)
	if err != nil || got != parent {
		t.Fatalf("SessionCommitSession under an agent = %q, %v; want %s", got, err, parent)
	}
}
