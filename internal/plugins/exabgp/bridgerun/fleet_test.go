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
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
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

	// flushGate holds a flush open until the test closes it, so a test can ask
	// what the script has been told WHILE its route is still being flushed. A
	// nil gate answers every command at once.
	flushGate chan struct{}
}

func (r *recorder) DispatchCommand(_ context.Context, command string) (string, json.RawMessage, error) {
	r.mu.Lock()
	r.commands = append(r.commands, command)
	gate := r.flushGate
	r.mu.Unlock()

	if gate != nil && strings.HasSuffix(command, " flush") {
		<-gate
	}
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
		{Name: "first", Argv: []string{first}, Encoder: bridge.EncoderJSON},
		{Name: "second", Argv: []string{second}, Encoder: bridge.EncoderJSON},
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
		{Name: "first", Argv: []string{first}, Encoder: bridge.EncoderJSON},
		{Name: "second", Argv: []string{second}, Encoder: bridge.EncoderJSON},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// A ze event, in the shape docs/architecture/api/json-format.md declares.
	// The ExaBGP-shaped object this line carried until 2026-09-06 is what the
	// bridge WRITES, never what it reads, and it only reached a script because
	// an unrecognizable envelope was named an UPDATE.
	fleet.Broadcast(peerStateEvent("127.0.0.1"))
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
		{Name: "one-shot", Argv: []string{oneShot}, Encoder: bridge.EncoderJSON, Respawn: false},
		{Name: "respawn", Argv: []string{respawner}, Encoder: bridge.EncoderJSON, Respawn: true},
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
		{Name: "respawn", Argv: []string{script}, Encoder: bridge.EncoderJSON, Respawn: true},
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
		{Name: "good", Argv: []string{good}, Encoder: bridge.EncoderJSON},
		{Name: "missing", Argv: []string{filepath.Join(dir, "not-here.sh")}, Encoder: bridge.EncoderJSON},
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
		{Name: "eor", Argv: []string{script}, Encoder: bridge.EncoderJSON},
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

// readFile answers a file's whole content, and the empty string when it does
// not exist yet.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: a path this test wrote.
	if err != nil {
		return ""
	}
	return string(data)
}

// TestFleetFeedsEachScriptOnlyItsOwnPeers is the second defect this package
// exists to remove. An ExaBGP neighbor names the processes it feeds in
// `api { processes [ ... ] }`, and until 2026-09-06 the bridge fanned every
// event to every child, so a two-process config delivered each neighbor's
// events to both scripts.
//
// test/exabgp-compat/api/api-multiple-api.ci is the same shape: two neighbors,
// each naming one process, and the case fails when either script sees the
// other's routes.
func TestFleetFeedsEachScriptOnlyItsOwnPeers(t *testing.T) {
	dir := t.TempDir()
	publicOut := filepath.Join(dir, "public.events")
	privateOut := filepath.Join(dir, "private.events")
	publicScript := writeScript(t, dir, "public.sh", "cat > "+publicOut+"\n")
	privateScript := writeScript(t, dir, "private.sh", "cat > "+privateOut+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "public", Argv: []string{publicScript}, Encoder: bridge.EncoderJSON,
			Feeds: []ScriptFeed{{Peer: "127.0.0.1", Events: []string{"neighbor-changes"}}}},
		{Name: "private", Argv: []string{privateScript}, Encoder: bridge.EncoderJSON,
			Feeds: []ScriptFeed{{Peer: "192.168.0.1", Events: []string{"neighbor-changes"}}}},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	fleet.Broadcast(peerStateEvent("127.0.0.1"))
	fleet.Broadcast(peerStateEvent("192.168.0.1"))

	waitFor(t, "each script to receive its own peer's event", func() bool {
		return strings.Contains(readFile(t, publicOut), "127.0.0.1") &&
			strings.Contains(readFile(t, privateOut), "192.168.0.1")
	})

	cancel()
	fleet.Stop()

	if got := readFile(t, publicOut); strings.Contains(got, "192.168.0.1") {
		t.Errorf("the public script received the private peer's event: %q", got)
	}
	if got := readFile(t, privateOut); strings.Contains(got, "127.0.0.1") {
		t.Errorf("the private script received the public peer's event: %q", got)
	}
}

// TestFleetSendsAnUnaddressedCommandToTheScriptsOwnPeers is the same relation
// in the other direction. An ExaBGP line that names no neighbor reaches the
// peers of the process that wrote it, which is what Reactor.peers(service)
// answers, so a two-process config must not put one script's route on the
// other's session.
func TestFleetSendsAnUnaddressedCommandToTheScriptsOwnPeers(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "private.sh",
		"echo 'announce route 2.2.2.2/32 next-hop 127.0.0.1'\nread ack\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "private", Argv: []string{script}, Encoder: bridge.EncoderJSON,
			Feeds: []ScriptFeed{{Peer: "192.168.0.1"}}},
	})
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	want := "send bgp 192.168.0.1 update text nhop 127.0.0.1 nlri ipv4/unicast add 2.2.2.2/32"
	waitFor(t, "the route to reach the engine addressed to this script's peer", func() bool {
		return rec.held(want)
	})

	cancel()
	fleet.Stop()

	for _, command := range rec.seen() {
		if strings.HasPrefix(command, "send bgp * ") {
			t.Errorf("command %q went to every peer, and this script serves one", command)
		}
	}
}

// TestFleetWritesEachScriptInItsOwnEncoder proves the encoder is per script.
// ExaBGP declares it inside the process block, so one bridge can feed a text
// script and a JSON script from the same event.
func TestFleetWritesEachScriptInItsOwnEncoder(t *testing.T) {
	dir := t.TempDir()
	textOut := filepath.Join(dir, "text.events")
	jsonOut := filepath.Join(dir, "json.events")
	textScript := writeScript(t, dir, "text.sh", "cat > "+textOut+"\n")
	jsonScript := writeScript(t, dir, "json.sh", "cat > "+jsonOut+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "text", Argv: []string{textScript}, Encoder: bridge.EncoderText},
		{Name: "json", Argv: []string{jsonScript}, Encoder: bridge.EncoderJSON},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	fleet.Broadcast(peerStateEvent("127.0.0.1"))

	wantText := "neighbor 127.0.0.1 up\n"
	waitFor(t, "both scripts to receive the event in their own format", func() bool {
		return readFile(t, textOut) != "" && readFile(t, jsonOut) != ""
	})

	cancel()
	fleet.Stop()

	if got := readFile(t, textOut); got != wantText {
		t.Errorf("the text script received %q, want %q", got, wantText)
	}
	got := readFile(t, jsonOut)
	if !strings.HasPrefix(got, "{") || !strings.Contains(got, `"exabgp":"6.0.0"`) {
		t.Errorf("the json script received %q, want one ExaBGP JSON object", got)
	}
}

// TestFleetRefusesAScriptWithNoEncoder checks the fan-out cannot reach a script
// whose format nobody set. The zero Encoder names no format, so writing it in
// whichever one the constant zero happens to select is the silently-wrong
// answer `ai/rules/principles.md` bans.
func TestFleetRefusesAScriptWithNoEncoder(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "quiet.sh", "read line\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "quiet", Argv: []string{script}},
	})
	err := fleet.Start(ctx, &recorder{})
	if err == nil {
		t.Fatalf("Start accepted a script with no encoder")
	}
	if !strings.Contains(err.Error(), "quiet") {
		t.Errorf("error = %v, want it to name the script quiet", err)
	}

	cancel()
	fleet.Stop()
}

// peerStateEvent is one ze state event for a named peer, in the shape
// docs/architecture/api/json-format.md declares.
func peerStateEvent(peer string) string {
	var tb textbuf.Buffer
	return tb.Str(`{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"`).
		Str(peer).Str(`","as":65001}},"state":"up"}}`).String()
}

// TestFleetFeedsAScriptOnlyTheEventsItsNeighborGranted is the third half of
// the same relation. An ExaBGP api block names the message kinds it feeds as
// well as the processes, and a block naming `receive { update; }` alone must
// not deliver the OPEN and the KEEPALIVE of the same session.
//
// test/exabgp-compat/etc/api-check.conf is that shape, and its script exits 1
// on the first line that is not the announce it expects.
func TestFleetFeedsAScriptOnlyTheEventsItsNeighborGranted(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "watcher.events")
	script := writeScript(t, dir, "watcher.sh", "cat > "+out+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "watcher", Argv: []string{script}, Encoder: bridge.EncoderText,
			Feeds: []ScriptFeed{{Peer: "127.0.0.1", Events: []string{"receive-update"}}}},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	fleet.Broadcast(peerStateEvent("127.0.0.1"))
	fleet.Broadcast(peerKeepaliveEvent("127.0.0.1"))
	fleet.Broadcast(peerUpdateEvent("127.0.0.1"))

	want := "neighbor 127.0.0.1 receive update announced 10.0.0.0/24 next-hop 10.0.0.1\n"
	waitFor(t, "the granted event to reach the script", func() bool {
		return strings.Contains(readFile(t, out), want)
	})

	cancel()
	fleet.Stop()

	got := readFile(t, out)
	for _, ungranted := range []string{" up\n", " keepalive\n"} {
		if strings.Contains(got, ungranted) {
			t.Errorf("the script received %q, which its neighbor did not grant:\n%s", ungranted, got)
		}
	}
}

// peerKeepaliveEvent is one ze KEEPALIVE event for a named peer.
func peerKeepaliveEvent(peer string) string {
	var tb textbuf.Buffer
	return tb.Str(`{"type":"bgp","bgp":{"message":{"type":"keepalive","direction":"received"},`).
		Str(`"peer":{"remote":{"address":"`).Str(peer).Str(`","as":65001}},"keepalive":{}}}`).String()
}

// peerUpdateEvent is one ze UPDATE event for a named peer, announcing one
// prefix with no path attributes.
func peerUpdateEvent(peer string) string {
	var tb textbuf.Buffer
	return tb.Str(`{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},`).
		Str(`"peer":{"remote":{"address":"`).Str(peer).Str(`","as":65001}},`).
		Str(`"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}}}`).String()
}

// TestFleetAcksARouteOnlyAfterItsFlush pins the order of the two answers a
// route command owes. `done` means the command is DONE, and a route is not
// done until it is on the wire, so the per-peer flush comes first.
//
// ExaBGP orders them the same way: announce_route awaits every peer's flush
// event and calls answer_done after it
// (src/exabgp/reactor/api/command/announce.py).
//
// Acking first let a script send its next command while the previous route's
// flush was still running. The two then reached the wire in whichever order
// they finished, and api-ipv4, api-ipv6, api-mvpn and api-vpnv4 each caught it
// by announcing and withdrawing one NLRI and reading the frames in order.
//
// The test HOLDS the flush open, because a flush that returns at once is
// acked-then-flushed and flushed-then-acked in the same observable order. With
// the flush held, the script is acked only if the ack does not wait for it.
func TestFleetAcksARouteOnlyAfterItsFlush(t *testing.T) {
	dir := t.TempDir()
	acked := filepath.Join(dir, "acked")
	script := writeScript(t, dir, "paced.sh",
		"echo 'announce route 1.1.1.1/32 next-hop 11.11.11.11'\n"+
			"read ack\n"+
			"echo \"$ack\" > "+acked+"\n"+
			"read done\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{flushGate: make(chan struct{})}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "paced", Argv: []string{script}, Encoder: bridge.EncoderJSON},
	})
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitFor(t, "the flush to start", func() bool {
		return rec.held("request peer * flush")
	})

	// The flush is held open here. An ack written before it would already be
	// on the script's stdin, and the script writes what it read.
	time.Sleep(50 * time.Millisecond)
	if got := readFile(t, acked); got != "" {
		t.Errorf("the script was acked with %q while its route was still being flushed", got)
	}

	close(rec.flushGate)
	waitFor(t, "the script to be acked once the flush completed", func() bool {
		return strings.TrimSpace(readFile(t, acked)) == "done"
	})

	cancel()
	fleet.Stop()
}
