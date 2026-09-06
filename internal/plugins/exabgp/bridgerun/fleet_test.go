package bridgerun

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: a Fleet runs EVERY script an ExaBGP config declares -- it forks one
// child per script, fans each event out to all of them, reads commands from all
// of them, and restarts only the ones whose respawn leaf asks for it.
// PREVENTS: a config naming two processes running one of them in silence, which
// is what the bridge did until 2026-09-06; a one-shot script being restarted
// forever; a crashing script being restarted forever.

// testFamilies is the family set these tests declare. A bare `announce eor`
// expands over it, so the tests carry one rather than the zero Translator that
// refuses that line.
var testFamilies = []string{"ipv4/unicast"}

// recorder is the engine a test gives a Fleet. It answers every command with
// success and remembers what it was asked.
type recorder struct {
	mu       sync.Mutex
	commands []string
}

func (r *recorder) DispatchCommand(_ context.Context, command string) (string, json.RawMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = append(r.commands, command)
	return "ok", nil, nil
}

func (r *recorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.commands...)
}

func (r *recorder) held(want string) bool {
	return slices.Contains(r.seen(), want)
}

// writeScript writes an executable /bin/sh script and answers its path.
func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// waitFor polls until ready answers true, and fails the test at the deadline.
// The scripts under test fork and exit in milliseconds, so the deadline is a
// bound on a hang rather than a wait anything normally reaches.
func waitFor(t *testing.T, what string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// lines answers the number of lines in a file, and zero when it does not exist.
func lines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: a path this test wrote.
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(data)))
}

// TestFleetRunsEveryScript is the defect this package exists to remove. Two
// declared scripts each write one ExaBGP line, and BOTH lines have to reach the
// engine. Running one of them and saying nothing is the failure.
func TestFleetRunsEveryScript(t *testing.T) {
	dir := t.TempDir()
	first := writeScript(t, dir, "first.sh",
		"echo 'announce route 1.1.1.1/32 next-hop 11.11.11.11'\nread ack\n")
	second := writeScript(t, dir, "second.sh",
		"echo 'announce route 2.2.2.2/32 next-hop 22.22.22.22'\nread ack\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "first", Argv: []string{first}},
		{Name: "second", Argv: []string{second}},
	})
	if fleet.Count() != 2 {
		t.Fatalf("Count = %d, want 2", fleet.Count())
	}
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wantFirst := "send bgp * update text nhop 11.11.11.11 nlri ipv4/unicast add 1.1.1.1/32"
	wantSecond := "send bgp * update text nhop 22.22.22.22 nlri ipv4/unicast add 2.2.2.2/32"
	waitFor(t, "both scripts to reach the engine", func() bool {
		return rec.held(wantFirst) && rec.held(wantSecond)
	})

	cancel()
	fleet.Stop()
}

// TestFleetFansEventsToEveryScript checks the other direction: one ze event
// reaches every script's stdin, with the same bytes.
func TestFleetFansEventsToEveryScript(t *testing.T) {
	dir := t.TempDir()
	firstOut := filepath.Join(dir, "first.events")
	secondOut := filepath.Join(dir, "second.events")
	first := writeScript(t, dir, "first.sh", "cat > "+firstOut+"\n")
	second := writeScript(t, dir, "second.sh", "cat > "+secondOut+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "first", Argv: []string{first}},
		{Name: "second", Argv: []string{second}},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	fleet.Broadcast(`{"type":"state","neighbor":{"address":{"peer":"127.0.0.1"}}}`)
	waitFor(t, "both scripts to receive the event", func() bool {
		return lines(t, firstOut) > 0 && lines(t, secondOut) > 0
	})

	cancel()
	fleet.Stop()

	got, err := os.ReadFile(firstOut) //nolint:gosec // G304: a path this test wrote.
	if err != nil {
		t.Fatalf("read %s: %v", firstOut, err)
	}
	want, err := os.ReadFile(secondOut) //nolint:gosec // G304: a path this test wrote.
	if err != nil {
		t.Fatalf("read %s: %v", secondOut, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the two scripts received different events:\nfirst  = %q\nsecond = %q", got, want)
	}
	if len(got) == 0 {
		t.Errorf("no event reached either script")
	}
}

// TestFleetLeavesAOneShotScriptStopped is api-no-respawn's assertion: the
// script whose block says `respawn false` exits after its work and is NOT
// started again, while its sibling keeps being restarted.
//
// The sibling reaching the restart limit is what marks the passage of time, so
// the test needs no sleep to know the one-shot had every chance to be restarted.
func TestFleetLeavesAOneShotScriptStopped(t *testing.T) {
	dir := t.TempDir()
	oneShotCount := filepath.Join(dir, "one-shot.forks")
	respawnCount := filepath.Join(dir, "respawn.forks")
	oneShot := writeScript(t, dir, "one-shot.sh", "echo fork >> "+oneShotCount+"\n")
	respawner := writeScript(t, dir, "respawn.sh", "echo fork >> "+respawnCount+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "one-shot", Argv: []string{oneShot}, Respawn: false},
		{Name: "respawn", Argv: []string{respawner}, Respawn: true},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitFor(t, "the respawning script to reach its restart limit", func() bool {
		return lines(t, respawnCount) == respawnMax
	})

	cancel()
	fleet.Stop()

	if got := lines(t, oneShotCount); got != 1 {
		t.Errorf("the one-shot script forked %d times, want 1", got)
	}
}

// TestFleetRestartsARespawningScript checks the positive half: a script whose
// block asks for a respawn is started again when it exits, up to ExaBGP's limit
// of respawnMax forks inside one window, and then stays stopped.
func TestFleetRestartsARespawningScript(t *testing.T) {
	dir := t.TempDir()
	count := filepath.Join(dir, "forks")
	script := writeScript(t, dir, "respawn.sh", "echo fork >> "+count+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "respawn", Argv: []string{script}, Respawn: true},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitFor(t, "the script to be restarted", func() bool {
		return lines(t, count) > 1
	})
	waitFor(t, "the script to reach its restart limit", func() bool {
		return lines(t, count) == respawnMax
	})

	cancel()
	fleet.Stop()

	// The limit holds after the supervisor gave up: a script that dies at once
	// does not fork forever.
	if got := lines(t, count); got != respawnMax {
		t.Errorf("the script forked %d times, want %d", got, respawnMax)
	}
}

// TestFleetStartNamesTheScriptItCannotFork refuses by name rather than starting
// the rest and running a subset in silence.
func TestFleetStartNamesTheScriptItCannotFork(t *testing.T) {
	dir := t.TempDir()
	good := writeScript(t, dir, "good.sh", "read line\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "good", Argv: []string{good}},
		{Name: "missing", Argv: []string{filepath.Join(dir, "not-here.sh")}},
	})
	err := fleet.Start(ctx, &recorder{})
	if err == nil {
		t.Fatalf("Start answered no error for a script that cannot be forked")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want it to name the script missing", err)
	}

	cancel()
	fleet.Stop()
}

// TestFleetExpandsABareEOROverTheDeclaredFamilies proves the family list the
// bridge declared reaches the translator. A bare `announce eor` names no
// family, so ExaBGP sends one End-of-RIB per negotiated family, and a Fleet
// that carried no family list would refuse the line by name instead.
//
// It also pins the ack: one line the script wrote is one answer, however many
// ze commands the line became.
func TestFleetExpandsABareEOROverTheDeclaredFamilies(t *testing.T) {
	dir := t.TempDir()
	acks := filepath.Join(dir, "acks")
	script := writeScript(t, dir, "eor.sh", "echo 'announce eor'\ncat > "+acks+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{}
	fleet := New(slogutil.DiscardLogger(), []string{"ipv4/unicast", "ipv6/unicast"}, []Script{
		{Name: "eor", Argv: []string{script}},
	})
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wantV4 := "send bgp * update text nlri ipv4/unicast eor"
	wantV6 := "send bgp * update text nlri ipv6/unicast eor"
	waitFor(t, "both End-of-RIB markers to reach the engine", func() bool {
		return rec.held(wantV4) && rec.held(wantV6)
	})
	waitFor(t, "the script to be acked", func() bool {
		return lines(t, acks) > 0
	})

	cancel()
	fleet.Stop()

	if got := lines(t, acks); got != 1 {
		t.Errorf("the script was acked %d times, want 1: one line it wrote is one answer", got)
	}
}
