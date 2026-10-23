package bridgerun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
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

	// answers is the payload a command is answered with, keyed by the exact
	// command. A command it does not hold is answered with no payload.
	answers map[string]json.RawMessage

	// refused is the commands ze refuses, keyed by the exact command, each
	// answered with its error. A command it does not hold succeeds.
	refused map[string]error
}

func (r *recorder) DispatchCommand(_ context.Context, command string) (string, json.RawMessage, error) {
	r.mu.Lock()
	r.commands = append(r.commands, command)
	gate := r.flushGate
	r.mu.Unlock()

	if gate != nil && strings.HasSuffix(command, " flush") {
		<-gate
	}
	if err := r.refused[command]; err != nil {
		return "", nil, err
	}
	return "ok", r.answers[command], nil
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

// lines answers the number of complete lines in a file, and zero when it does
// not exist. It counts newlines, not words: an answer such as `error: <why>`
// is one line, and counting its words let a wait end before the last answers
// were written.
func lines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: a path this test wrote.
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n")
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

	wantFirst := "send bgp * update text nhop 11.11.11.11 nlri add 1.1.1.1/32"
	wantSecond := "send bgp * update text nhop 22.22.22.22 nlri add 2.2.2.2/32"
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

// TestFleetLogsTheScriptItCannotStart proves that a script whose program is
// not on PATH is reported at ERROR, with the run command, the exec error, why
// it matters and what to do. Method: start a fleet whose one script names a
// program no PATH holds, and read the line the fleet's logger received.
// PREVENTS: a `run` command naming a program the daemon cannot find failing in
// silence. Start answers the error to the engine's startup callback, which no
// operator reads, so conf-watchdog's helper never ran and nothing said why.
func TestFleetLogsTheScriptItCannotStart(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(log, testFamilies, []Script{
		{Name: "watchdog", Argv: []string{"ze-no-such-helper", "watchdog"}, Encoder: bridge.EncoderJSON},
	})
	if err := fleet.Start(ctx, &recorder{}); err == nil {
		t.Fatalf("Start answered no error for a program that is not on PATH")
	}
	cancel()
	fleet.Stop()

	line := logged.String()
	for _, want := range []string{
		"level=ERROR",
		"exabgp-bridge script could not start",
		"no route it announces or withdraws reaches any peer",
		"script=watchdog",
		`run="ze-no-such-helper watchdog"`,
		"executable file not found",
		"put the directory that holds ze-no-such-helper on ze's PATH",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("log does not carry %q:\n%s", want, line)
		}
	}
}

// TestFleetSendsABareEORForIPv4UnicastOnly proves a bare `announce eor`
// reaches the engine as the IPv4 unicast End-of-RIB alone, even when the
// bridge declared more families. ExaBGP at the pinned commit reads the bare
// form as IPv4 unicast (b0b734ec2e made the bridge match it), so an IPv6
// marker here would be a marker ExaBGP never sends.
//
// It also pins the ack: one line the script wrote is one answer.
func TestFleetSendsABareEORForIPv4UnicastOnly(t *testing.T) {
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

	wantV4 := "send bgp * update text nlri eor"
	notV6 := "send bgp * update text mp ipv6/unicast eor"
	waitFor(t, "the IPv4 unicast End-of-RIB to reach the engine", func() bool {
		return rec.held(wantV4)
	})
	waitFor(t, "the script to be acked", func() bool {
		return lines(t, acks) > 0
	})

	cancel()
	fleet.Stop()

	// The ack follows the dispatch of every command the line became, so an
	// IPv6 marker would be held by now.
	if rec.held(notV6) {
		t.Errorf("a bare eor sent the IPv6 unicast End-of-RIB %q; ExaBGP sends IPv4 unicast only", notV6)
	}
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

	want := "send bgp 192.168.0.1 update text nhop 127.0.0.1 nlri add 2.2.2.2/32"
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
	if !strings.HasPrefix(got, "{") || !strings.Contains(got, `"exabgp": "6.0.0"`) {
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

// TestFleetRenderOncePerFormat: a fan-out renders each format once and hands
// the same bytes to every script reading it, and the two JSON formats differ by
// the envelope their API version declares.
//
// VALIDATES: X-5, X-6: v4 JSON is `4.0.1`, v6 JSON `6.0.0`, one render each.
// PREVENTS: a v4 JSON helper and a v6 JSON helper sharing one cached line.
func TestFleetRenderOncePerFormat(t *testing.T) {
	fleet := New(slogutil.DiscardLogger(), testFamilies, nil)
	event := bridge.Event{Kind: "state", Peer: "127.0.0.1", Payload: map[string]any{"state": "up"}}
	var cache [bridge.FormatCount][]byte
	v6, ok := fleet.render(&cache, bridge.FormatV6JSON, bridge.FormParsed, event)
	if !ok || !json.Valid(v6) || !bytes.Contains(v6, []byte(`"exabgp": "6.0.0"`)) {
		t.Fatalf("v6 render = %q, %v; want a 6.0.0 JSON event", v6, ok)
	}
	v4, ok := fleet.render(&cache, bridge.FormatV4JSON, bridge.FormParsed, event)
	if !ok || !json.Valid(v4) || !bytes.Contains(v4, []byte(`"exabgp": "4.0.1"`)) {
		t.Fatalf("v4 render = %q, %v; want a 4.0.1 JSON event", v4, ok)
	}
	again, ok := fleet.render(&cache, bridge.FormatV6JSON, bridge.FormParsed, event)
	if !ok || &again[0] != &v6[0] {
		t.Fatalf("second v6 render was rendered again rather than reused")
	}
}

// TestFleetRenderUnknownTextEvent preserves the safe text-encoder answer for
// an event kind supplied by a newer event producer, including its cached no-line result.
func TestFleetRenderUnknownTextEvent(t *testing.T) {
	fleet := New(slogutil.DiscardLogger(), testFamilies, nil)
	event := bridge.Event{Kind: "future-event"}
	var cache [bridge.FormatCount][]byte
	for range 2 {
		line, ok := fleet.render(&cache, bridge.FormatV4Text, bridge.FormParsed, event)
		if ok || len(line) != 0 {
			t.Fatalf("unknown text event = %q, %v; want no line", line, ok)
		}
	}
	if cache[bridge.FormatV4Text.Slot()] == nil {
		t.Fatal("unknown text event was not cached")
	}
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

// TestFleetAnswersARouteErrorWhenItsFlushIsRefused proves a refused flush
// reaches the script. The route command itself is accepted; only the flush
// that would put it on the wire is refused. `done` would tell the script a
// barrier held that did not, so the route line is answered `error`.
//
// The flush refusal travels scriptRunner.Flush -> bridge.LineRunner.Flush ->
// failSelectorLines. A Flush that dropped the error (logged it and answered
// nothing) acks the line `done`, and this test goes red.
func TestFleetAnswersARouteErrorWhenItsFlushIsRefused(t *testing.T) {
	dir := t.TempDir()
	acked := filepath.Join(dir, "acked")
	script := writeScript(t, dir, "refused.sh",
		"echo 'announce route 1.1.1.1/32 next-hop 11.11.11.11'\n"+
			"read ack\n"+
			"echo \"$ack\" > "+acked+"\n"+
			"read never\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{refused: map[string]error{
		"request peer * flush": errors.New("peer * not established"),
	}}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "refused", Argv: []string{script}, Encoder: bridge.EncoderText},
	})
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitFor(t, "the script to be answered", func() bool {
		return lines(t, acked) > 0
	})
	if got := strings.TrimSpace(readFile(t, acked)); !strings.HasPrefix(got, "error") {
		t.Errorf("a route whose flush ze refused was answered %q, want an error", got)
	}
	if !rec.held("request peer * flush") {
		t.Errorf("the route was never flushed: %v", rec.seen())
	}

	cancel()
	fleet.Stop()
}

// TestFleetAnswersStatusFromZesPeerList proves a script's `status` line is
// answered from the peers ze lists, through the plugin runner's dispatcher: the
// peer key is ExaBGP's Neighbor.name() and the state is the fsm-state ze
// reported, then the line is closed with done. A runner that answered from
// anything but ze's peer list would not print this peer or this state.
func TestFleetAnswersStatusFromZesPeerList(t *testing.T) {
	dir := t.TempDir()
	answered := filepath.Join(dir, "answered")
	script := writeScript(t, dir, "status.sh",
		"echo 'status'\n"+
			"while IFS= read -r line; do echo \"$line\" >> "+answered+"; [ \"$line\" = done ] && break; done\n"+
			"read never\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{answers: map[string]json.RawMessage{
		bridge.StatusPeerCommand: json.RawMessage(`{"peers":{"192.0.2.9":{"local-ip":"192.0.2.1",` +
			`"local-as":65001,"remote-as":65002,"router-id":"10.0.0.1","fsm-state":"OPENCONFIRM"}}}`),
	}}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "status", Argv: []string{script}, Encoder: bridge.EncoderText},
	})
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitFor(t, "the status answer to close with done", func() bool {
		return strings.HasSuffix(strings.TrimSpace(readFile(t, answered)), "done")
	})
	got := readFile(t, answered)
	if !rec.held(bridge.StatusPeerCommand) {
		t.Errorf("the runner never asked ze for %q; commands: %q", bridge.StatusPeerCommand, rec.seen())
	}
	for _, want := range []string{
		"ExaBGP Daemon Status",
		"Peers: 1",
		"  - neighbor 192.0.2.9 local-ip 192.0.2.1 local-as 65001 peer-as 65002 router-id 10.0.0.1 family-allowed in-open: OPENCONFIRM",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("status answer lacks %q; got:\n%s", want, got)
		}
	}

	cancel()
	fleet.Stop()
}

// TestFleetPutsTheNegotiatedBlockOnlyWhereGranted checks that, for an API 4
// script, an UPDATE line carries the session's negotiated block when the peer
// grants `negotiated`, and none when it does not: ExaBGP 5.0.1 hands such a
// process None (src/exabgp/reactor/protocol.py, read_message).
//
// VALIDATES: bridge.Sessions through the internal runner, per script grant.
// PREVENTS: the block missing from a 5.0 helper's UPDATE, or reaching a
// process whose api block never asked for it.
func TestFleetPutsTheNegotiatedBlockOnlyWhereGranted(t *testing.T) {
	t.Setenv("exabgp.api.version", "4")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	dir := t.TempDir()
	grantedOut := filepath.Join(dir, "granted.events")
	plainOut := filepath.Join(dir, "plain.events")
	grantedScript := writeScript(t, dir, "granted.sh", "cat > "+grantedOut+"\n")
	plainScript := writeScript(t, dir, "plain.sh", "cat > "+plainOut+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "granted", Argv: []string{grantedScript}, Encoder: bridge.EncoderJSON,
			Feeds: []ScriptFeed{{Peer: "127.0.0.1", Events: []string{"receive-update", "negotiated"}}}},
		{Name: "plain", Argv: []string{plainScript}, Encoder: bridge.EncoderJSON,
			Feeds: []ScriptFeed{{Peer: "127.0.0.1", Events: []string{"receive-update"}}}},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	peer := `"peer":{"remote":{"address":"127.0.0.1","as":65001}}`
	fleet.Broadcast(`{"type":"bgp","bgp":{"message":{"type":"negotiated"},` + peer +
		`,"negotiated":{"timer":{"hold-time":90},"message-size":4096,"route-refresh":"normal","asn4":true,"families":["ipv6/unicast","ipv4/unicast"]}}}`)
	fleet.Broadcast(`{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},` + peer +
		`,"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`)

	waitFor(t, "both scripts to receive the UPDATE", func() bool {
		return strings.Contains(readFile(t, grantedOut), "10.0.0.0/24") &&
			strings.Contains(readFile(t, plainOut), "10.0.0.0/24")
	})

	cancel()
	fleet.Stop()

	block := negotiatedOfUpdate(t, readFile(t, grantedOut))
	// The families keep the event's order, the peer's OPEN order, as ExaBGP
	// lists them (negotiated.py, Negotiated.received); this test expected AFI
	// then SAFI order while the bridge sorted, which ExaBGP never did.
	want := map[string]any{
		"add_path": map[string]any{"receive": []any{}, "send": []any{}},
		"asn4":     true, "families": []any{"ipv6 unicast", "ipv4 unicast"},
		"hold_time": float64(90), "message_size": float64(4096), "multisession": false,
		"nexthop": []any{}, "operational": false, "refresh": "normal",
	}
	if !reflect.DeepEqual(block, want) {
		t.Errorf("the granted script's negotiated block:\n got %v\nwant %v", block, want)
	}
	if got := readFile(t, plainOut); strings.Contains(got, `"negotiated"`) {
		t.Errorf("a script not granted negotiated received the block: %q", got)
	}
}

// TestFleetGivesTheBlockOnlyWhereGranted checks, for an API 4 script and an
// API 6 one, that a script the peer grants `negotiated` gets the block on
// every UPDATE line, the empty session before the session's negotiated event
// and the session's after it, and that a script it does not grant gets none,
// even once the session is up: the pin f9bfb853b decides so for every API
// version (Protocol._api_negotiated: "return self.negotiated if
// self._api.get('negotiated', False) else Negotiated.UNSET").
//
// VALIDATES: Event.ForGrant through the internal runner, per script grant.
// PREVENTS: the block gated on the API version rather than on the grant.
func TestFleetGivesTheBlockOnlyWhereGranted(t *testing.T) {
	for _, version := range []string{"4", "6"} {
		t.Run("api "+version, func(t *testing.T) { fleetGivesTheBlockOnlyWhereGranted(t, version) })
	}
}

func fleetGivesTheBlockOnlyWhereGranted(t *testing.T, version string) {
	t.Setenv("exabgp.api.version", version)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	dir := t.TempDir()
	out := filepath.Join(dir, "granted.events")
	plainOut := filepath.Join(dir, "plain.events")
	script := writeScript(t, dir, "granted.sh", "cat > "+out+"\n")
	plainScript := writeScript(t, dir, "plain.sh", "cat > "+plainOut+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "granted", Argv: []string{script}, Encoder: bridge.EncoderJSON,
			Feeds: []ScriptFeed{{Peer: "127.0.0.1", Events: []string{"receive-update", "negotiated"}}}},
		{Name: "plain", Argv: []string{plainScript}, Encoder: bridge.EncoderJSON,
			Feeds: []ScriptFeed{{Peer: "127.0.0.1", Events: []string{"receive-update"}}}},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	peer := `"peer":{"remote":{"address":"127.0.0.1","as":65001}}`
	update := `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},` + peer +
		`,"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`
	fleet.Broadcast(update)
	waitFor(t, "the UPDATE before the session's negotiated event", func() bool {
		return strings.Contains(readFile(t, out), "10.0.0.0/24")
	})
	unset := negotiatedOfUpdate(t, readFile(t, out))

	fleet.Broadcast(`{"type":"bgp","bgp":{"message":{"type":"negotiated"},` + peer +
		`,"negotiated":{"timer":{"hold-time":90},"message-size":4096,"route-refresh":"normal","asn4":true,"families":["ipv4/unicast"]}}}`)
	fleet.Broadcast(strings.Replace(update, "10.0.0.0/24", "10.0.1.0/24", 1))
	waitFor(t, "the UPDATE after it", func() bool {
		return strings.Contains(readFile(t, out), "10.0.1.0/24") &&
			strings.Contains(readFile(t, plainOut), "10.0.1.0/24")
	})
	cancel()
	fleet.Stop()

	if got := readFile(t, plainOut); strings.Contains(got, `"negotiated"`) {
		t.Errorf("an API %s script not granted negotiated received the block: %q", version, got)
	}
	wantUnset := map[string]any{
		"add_path": map[string]any{"receive": []any{}, "send": []any{}},
		"asn4":     false, "families": []any{},
		"hold_time": float64(0), "message_size": float64(4096), "multisession": false,
		"nexthop": []any{}, "operational": false, "refresh": "absent",
	}
	if !reflect.DeepEqual(unset, wantUnset) {
		t.Errorf("the block before the OPENs:\n got %v\nwant %v", unset, wantUnset)
	}
	var session any
	for line := range strings.SplitSeq(readFile(t, out), "\n") {
		if !strings.Contains(line, "10.0.1.0/24") {
			continue
		}
		var document map[string]any
		if err := json.Unmarshal([]byte(line), &document); err != nil {
			t.Fatalf("the UPDATE line does not parse: %v", err)
		}
		neighbor, _ := document["neighbor"].(map[string]any)
		session = neighbor["negotiated"]
	}
	block, _ := session.(map[string]any)
	if block["hold_time"] != float64(90) {
		t.Errorf("an API %s script granted negotiated got no session block: %v", version, session)
	}
}

// negotiatedOfUpdate answers the neighbor.negotiated member of the UPDATE line
// among a script's lines.
func negotiatedOfUpdate(t *testing.T, lines string) any {
	t.Helper()
	for line := range strings.SplitSeq(lines, "\n") {
		if !strings.Contains(line, "10.0.0.0/24") {
			continue
		}
		var document map[string]any
		if err := json.Unmarshal([]byte(line), &document); err != nil {
			t.Fatalf("the UPDATE line does not parse: %v: %s", err, line)
		}
		neighbor, _ := document["neighbor"].(map[string]any)
		return neighbor["negotiated"]
	}
	t.Fatalf("no UPDATE line in %q", lines)
	return nil
}
