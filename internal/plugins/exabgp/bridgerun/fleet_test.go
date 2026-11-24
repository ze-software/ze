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
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: a Fleet runs each script once, translates commands and events in
// the script's encoder, applies BusParticipantFilter scope/negotiated grants,
// flushes before acking, answers status from Ze's peer list, and reports a
// script's exit code without respawning it (E-6).
// PREVENTS: silent drop of commands/events, wrong peer scope, stale negotiated
// blocks, or internal respawn loops after E-6.

// testFamilies is the family set these tests declare.
var testFamilies = []string{"ipv4/unicast"}

// recorder is the engine a test gives a Fleet. It answers every command with
// success and remembers what it was asked.
type recorder struct {
	mu       sync.Mutex
	commands []string

	// flushGate holds a flush open until the test closes it, so a test can ask
	// what the script has been told WHILE its route is still being flushed.
	flushGate chan struct{}

	// answers is the payload a command is answered with, keyed by the exact
	// command.
	answers map[string]json.RawMessage

	// refused is the commands ze refuses, keyed by the exact command.
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
// not exist.
func lines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: a path this test wrote.
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n")
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

// TestFleetRunsEveryScript checks that declared scripts run and dispatch their
// ExaBGP commands to the engine.
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

// TestFleetFansEventsToEveryScript checks that one ze event reaches every
// script's stdin with identical bytes.
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

	fleet.Broadcast(peerStateEvent("127.0.0.1"))
	waitFor(t, "both scripts to receive the event", func() bool {
		return lines(t, firstOut) > 0 && lines(t, secondOut) > 0
	})

	cancel()
	fleet.Stop()

	got := readFile(t, firstOut)
	want := readFile(t, secondOut)
	if got != want {
		t.Errorf("the two scripts received different events:\nfirst  = %q\nsecond = %q", got, want)
	}
	if len(got) == 0 {
		t.Errorf("no event reached either script")
	}
}

// TestFleetReportsScriptExitCodeWithoutRespawn verifies E-6: a script that
// exits reports its exact exit status via SetOnExit and is never forked again.
//
// VALIDATES: E-6 single-run watcher reports exit status without respawning.
// PREVENTS: internal respawn loops or lost child exit codes.
func TestFleetReportsScriptExitCodeWithoutRespawn(t *testing.T) {
	dir := t.TempDir()
	countFile := filepath.Join(dir, "forks")
	scriptPath := writeScript(t, dir, "exit3.sh", "echo fork >> "+countFile+"\nexit 3\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var exitCalls atomic.Int32
	exitCh := make(chan int, 2)

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "exiter", Argv: []string{scriptPath}, Encoder: bridge.EncoderText},
	})
	fleet.SetOnExit(func(code int) {
		exitCalls.Add(1)
		exitCh <- code
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case code := <-exitCh:
		if code != 3 {
			t.Errorf("exit code = %d, want 3", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for script exit callback")
	}

	cancel()
	fleet.Stop()

	if got := lines(t, countFile); got != 1 {
		t.Errorf("script forked %d times, want 1", got)
	}
	if got := exitCalls.Load(); got != 1 {
		t.Errorf("onExit invoked %d times, want 1", got)
	}
}

// TestFleetStartNamesTheScriptItCannotFork refuses by name rather than starting
// a subset in silence.
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
// it matters, and what to do.
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
// bridge declared more families.
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

	if rec.held(notV6) {
		t.Errorf("a bare eor sent the IPv6 unicast End-of-RIB %q; ExaBGP sends IPv4 unicast only", notV6)
	}
	if got := lines(t, acks); got != 1 {
		t.Errorf("the script was acked %d times, want 1: one line it wrote is one answer", got)
	}
}

// TestFleetSendsAnUnaddressedCommandToTheScriptsOwnPeers proves ApplyFilter
// narrows unaddressed ExaBGP commands to the peers permitted by `send > to`.
func TestFleetSendsAnUnaddressedCommandToTheScriptsOwnPeers(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "private.sh",
		"echo 'announce route 2.2.2.2/32 next-hop 127.0.0.1'\nread ack\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "private", Argv: []string{script}, Encoder: bridge.EncoderJSON},
	})
	fleet.ApplyFilter(&rpc.BusParticipantFilter{
		SendRules: []rpc.BusFilterRule{{
			Message: "cli:send bgp",
			Allowed: true,
			Targets: []rpc.BusFilterTarget{{Path: "/bgp/peer/p1", Address: "192.168.0.1"}},
		}},
		ReceiveAll:     true,
		ReceiveFromAll: true,
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
// whose format nobody set.
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

// peerStateEvent is one ze state event for a named peer.
func peerStateEvent(peer string) string {
	var tb textbuf.Buffer
	return tb.Str(`{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"`).
		Str(peer).Str(`","as":65001}},"state":"up"}}`).String()
}

// TestFleetAcksARouteOnlyAfterItsFlush pins the order of flush before ack.
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
// reaches the script as an error.
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
// answered from the peers ze lists.
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

// TestFleetPutsTheNegotiatedBlockOnlyWhereGranted checks that an UPDATE line
// carries the session's negotiated block when the participant filter grants
// `bgp:negotiated`, and omits it when `bgp:negotiated` is not granted.
//
// VALIDATES: bridge.Sessions + ApplyFilter negotiated grant per endpoint.
// PREVENTS: the block missing when granted, or leaking when not granted.
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

	grantedFleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "granted", Argv: []string{grantedScript}, Encoder: bridge.EncoderJSON},
	})
	grantedFleet.ApplyFilter(&rpc.BusParticipantFilter{
		ReceiveRules: []rpc.BusFilterRule{
			{Message: "bgp:update", Allowed: true, Targets: []rpc.BusFilterTarget{{Address: "127.0.0.1"}}},
			{Message: "bgp:negotiated", Allowed: true, Targets: []rpc.BusFilterTarget{{Address: "127.0.0.1"}}},
		},
	})

	plainFleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "plain", Argv: []string{plainScript}, Encoder: bridge.EncoderJSON},
	})
	plainFleet.ApplyFilter(&rpc.BusParticipantFilter{
		ReceiveRules: []rpc.BusFilterRule{
			{Message: "bgp:update", Allowed: true, Targets: []rpc.BusFilterTarget{{Address: "127.0.0.1"}}},
			{Message: "bgp:negotiated", Allowed: false},
		},
	})

	if err := grantedFleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("granted Start: %v", err)
	}
	if err := plainFleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("plain Start: %v", err)
	}

	peer := `"peer":{"remote":{"address":"127.0.0.1","as":65001}}`
	negEvent := `{"type":"bgp","bgp":{"message":{"type":"negotiated"},` + peer +
		`,"negotiated":{"timer":{"hold-time":90},"message-size":4096,"route-refresh":"normal","asn4":true,"families":["ipv6/unicast","ipv4/unicast"]}}}`
	updEvent := `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},` + peer +
		`,"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`

	grantedFleet.Broadcast(negEvent)
	grantedFleet.Broadcast(updEvent)
	plainFleet.Broadcast(updEvent)

	waitFor(t, "both scripts to receive the UPDATE", func() bool {
		return strings.Contains(readFile(t, grantedOut), "10.0.0.0/24") &&
			strings.Contains(readFile(t, plainOut), "10.0.0.0/24")
	})

	cancel()
	grantedFleet.Stop()
	plainFleet.Stop()

	block := negotiatedOfUpdate(t, readFile(t, grantedOut))
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

// TestFleetGivesTheBlockOnlyWhereGranted checks for API 4 and API 6 that
// Event.ForGrant follows the compiled filter's `bgp:negotiated` grant.
//
// VALIDATES: Event.ForGrant through ApplyFilter for both API versions.
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

	grantedFleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "granted", Argv: []string{script}, Encoder: bridge.EncoderJSON},
	})
	grantedFleet.ApplyFilter(&rpc.BusParticipantFilter{
		ReceiveRules: []rpc.BusFilterRule{
			{Message: "bgp:update", Allowed: true, Targets: []rpc.BusFilterTarget{{Address: "127.0.0.1"}}},
			{Message: "bgp:negotiated", Allowed: true, Targets: []rpc.BusFilterTarget{{Address: "127.0.0.1"}}},
		},
	})

	plainFleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "plain", Argv: []string{plainScript}, Encoder: bridge.EncoderJSON},
	})
	plainFleet.ApplyFilter(&rpc.BusParticipantFilter{
		ReceiveRules: []rpc.BusFilterRule{
			{Message: "bgp:update", Allowed: true, Targets: []rpc.BusFilterTarget{{Address: "127.0.0.1"}}},
			{Message: "bgp:negotiated", Allowed: false},
		},
	})

	if err := grantedFleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("granted Start: %v", err)
	}
	if err := plainFleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("plain Start: %v", err)
	}

	peer := `"peer":{"remote":{"address":"127.0.0.1","as":65001}}`
	update := `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},` + peer +
		`,"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.0.0/24"]}]}}}}`
	grantedFleet.Broadcast(update)
	waitFor(t, "the UPDATE before the session's negotiated event", func() bool {
		return strings.Contains(readFile(t, out), "10.0.0.0/24")
	})
	unset := negotiatedOfUpdate(t, readFile(t, out))

	neg := `{"type":"bgp","bgp":{"message":{"type":"negotiated"},` + peer +
		`,"negotiated":{"timer":{"hold-time":90},"message-size":4096,"route-refresh":"normal","asn4":true,"families":["ipv4/unicast"]}}}`
	update2 := strings.Replace(update, "10.0.0.0/24", "10.0.1.0/24", 1)
	grantedFleet.Broadcast(neg)
	grantedFleet.Broadcast(update2)
	plainFleet.Broadcast(update2)
	waitFor(t, "the UPDATE after it", func() bool {
		return strings.Contains(readFile(t, out), "10.0.1.0/24") &&
			strings.Contains(readFile(t, plainOut), "10.0.1.0/24")
	})
	cancel()
	grantedFleet.Stop()
	plainFleet.Stop()

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
