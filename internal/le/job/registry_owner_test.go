// Related: registry.go -- the entry writer and reader these tests drive
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-13. A registry entry
// records the session and the agent that asked for the job, and an entry that
// carries neither is still a valid entry whose owner is not known.
// PREVENTS: a report that shows a held slot and cannot say whose it is, and a
// reader that refuses the entries an older le or the QEMU guest wrote.

package job

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// identify names this test process's caller the way an agent's identity line
// does, for the length of one case. An empty agent removes the variable, which
// is a main thread.
func identify(t *testing.T, session, agent string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", session)
	if agent == "" {
		unset(t, lepath.AgentKey)
	} else {
		t.Setenv("ZE_AGENT_ID", agent)
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// TestEntryRecordsSessionAndAgent admits a job through Admit, the entry point
// every caller uses, while the environment names a session and an agent, and
// reads the entry back through readEntry.
//
// The raw bytes are checked too: the two fields come after the nine an entry
// always held, which is what keeps an older reader reading it.
func TestEntryRecordsSessionAndAgent(t *testing.T) {
	detach(t)
	identify(t, "owner-session-1", "owner-agent-1")
	root := fixtureRepo(t)
	adm := admission(t, root)

	ticket, err := adm.Admit("owned", []string{"true"})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	defer ticket.Release(0)
	if ticket.Kind != KindClaimed {
		t.Fatalf("the job was admitted as %v, want a claimed slot", ticket.Kind)
	}

	held, ok := adm.readEntry(adm.abs(ticket.Entry))
	if !ok {
		t.Fatal("the entry admission wrote cannot be read")
	}
	if held.who.session != "owner-session-1" {
		t.Errorf("the entry records session %q, want owner-session-1", held.who.session)
	}
	if held.who.agent != "owner-agent-1" {
		t.Errorf("the entry records agent %q, want owner-agent-1", held.who.agent)
	}
	if held.command != "true" {
		t.Errorf("the entry records command %q, want true", held.command)
	}

	body := read(t, adm.abs(ticket.Entry))
	command := strings.Index(body, "CMD=")
	session := strings.Index(body, "SESSION=owner-session-1\n")
	agent := strings.Index(body, "AGENT=owner-agent-1\n")
	if command < 0 || session < command || agent < session {
		t.Errorf("the entry is\n%s\nwant SESSION then AGENT after the existing fields", body)
	}

	// The reader the cache cleaner uses still answers the fields it reads.
	holders, err := Holders(root)
	if err != nil {
		t.Fatalf("Holders: %v", err)
	}
	if len(holders) != 1 || holders[0].PID != os.Getpid() || holders[0].Label != "owned" {
		t.Errorf("Holders answered %+v, want the one entry with this pid and its label", holders)
	}
}

// TestEntryOfAMainThreadRecordsNoAgent: a caller with no agent identity is a
// main thread, and its entry carries the session and an empty agent field.
func TestEntryOfAMainThreadRecordsNoAgent(t *testing.T) {
	detach(t)
	identify(t, "owner-session-2", "")
	adm := admission(t, fixtureRepo(t))

	ticket, err := adm.Admit("main", []string{"true"})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	defer ticket.Release(0)

	held, ok := adm.readEntry(adm.abs(ticket.Entry))
	if !ok {
		t.Fatal("the entry admission wrote cannot be read")
	}
	if held.who.session != "owner-session-2" || held.who.agent != "" {
		t.Errorf("the entry records %+v, want session owner-session-2 and no agent", held.who)
	}
	if body := read(t, adm.abs(ticket.Entry)); !strings.Contains(body, "\nAGENT=\n") {
		t.Errorf("the entry is\n%s\nwant an empty AGENT field", body)
	}
}

// TestEntryWithoutOwnerFieldsReadsAsUnknown reads three entries nobody stamped
// with a usable owner: one written before the fields existed, one whose
// values could not name a file, and one whose values are empty.
//
// VALIDATES: AC-13's survivor. Each is a readable entry, and each owner is
// empty rather than an error or the hostile value.
func TestEntryWithoutOwnerFieldsReadsAsUnknown(t *testing.T) {
	root := fixtureRepo(t)
	adm := admission(t, root)
	older, _ := heldEntry(t, root, "older", 4001)

	hostile, _ := heldEntry(t, root, "hostile", 4002)
	appendFields(t, hostile, "SESSION=../../etc/passwd\nAGENT=an agent; rm -rf\n")

	empty, _ := heldEntry(t, root, "empty", 4003)
	appendFields(t, empty, "SESSION=\nAGENT=\n")

	for _, path := range []string{older, hostile, empty} {
		held, ok := adm.readEntry(path)
		if !ok {
			t.Errorf("%s is not readable; an entry without owner fields is still an entry", path)
			continue
		}
		if held.who.session != "" || held.who.agent != "" {
			t.Errorf("%s reads owner %+v, want both fields unknown", path, held.who)
		}
		if held.label == "" || held.pid == 0 {
			t.Errorf("%s lost its label or pid: %+v", path, held)
		}
	}
}

// TestCommandTextCannotForgeTheOwner admits a job whose argv holds line
// breaks followed by owner fields, and reads the entry back through readEntry.
//
// CMD is written before SESSION and AGENT and the reader takes the first line
// of each name, so a line break that reached the file would hand the owner to
// whoever chose the argv.
//
// VALIDATES: the writer's invariant, no caller-controlled value opens a field
// line. The owner is the real caller's and the command is one line.
// PREVENTS: a report that names a session and agent an argv word supplied.
func TestCommandTextCannotForgeTheOwner(t *testing.T) {
	detach(t)
	identify(t, "owner-session-3", "owner-agent-3")
	adm := admission(t, fixtureRepo(t))

	ticket, err := adm.Admit("forged", []string{"true", "\nSESSION=evil\nAGENT=evil", "a\r\nPID=1"})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	defer ticket.Release(0)

	held, ok := adm.readEntry(adm.abs(ticket.Entry))
	if !ok {
		t.Fatal("the entry admission wrote cannot be read")
	}
	if held.who.session != "owner-session-3" || held.who.agent != "owner-agent-3" {
		t.Errorf("the entry records owner %+v, want the caller's owner-session-3 and owner-agent-3", held.who)
	}
	if held.pid != os.Getpid() {
		t.Errorf("the entry records pid %d, want this process %d", held.pid, os.Getpid())
	}
	if want := "true  SESSION=evil AGENT=evil a  PID=1"; held.command != want {
		t.Errorf("the entry records command %q, want the whole text on one line: %q", held.command, want)
	}

	body := read(t, adm.abs(ticket.Entry))
	if strings.Contains(body, "\r") {
		t.Errorf("the entry holds a carriage return an argv word supplied:\n%q", body)
	}
	for _, name := range []string{"SESSION=", "AGENT=", "PID=", "CMD="} {
		if lines := linesOpening(body, name); lines != 1 {
			t.Errorf("the entry holds %d lines opening with %s, want 1:\n%s", lines, name, body)
		}
	}
}

// linesOpening counts the lines of a registry file that open with one prefix.
func linesOpening(body, prefix string) int {
	count := 0
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

// appendFields adds lines to a fabricated registry file.
func appendFields(t *testing.T, path, lines string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644) //nolint:gosec // a registry file this test fabricated
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if _, err := file.WriteString(lines); err != nil {
		t.Fatalf("append to %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}
