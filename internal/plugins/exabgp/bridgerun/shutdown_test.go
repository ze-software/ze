package bridgerun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// TestFleetShutdownReachesTheScriptsBeforeTheCancel is the internal runner's
// stop order: Shutdown, then the cancel that kills every child
// (exec.CommandContext), then Stop. Each script copies its stdin to a file, so
// the file holds what the child read before it died. Shutdown that only queues
// the line loses the race to the kill and the files come back empty; Shutdown
// MUST return only once each child has read the line and exited on its EOF.
//
// Each script's stdout goes to its file, so its stdout pipe closes at once
// while it still reads stdin. That is the second thing the test holds: reap
// once forgot stdin as soon as stdout closed, so every later line, this one
// included, was dropped and the script never saw its EOF.
func TestFleetShutdownReachesTheScriptsBeforeTheCancel(t *testing.T) {
	dir := t.TempDir()
	textOut := filepath.Join(dir, "text.events")
	jsonOut := filepath.Join(dir, "json.events")
	textScript := writeScript(t, dir, "text.sh", "exec cat > "+textOut+"\n")
	jsonScript := writeScript(t, dir, "json.sh", "exec cat > "+jsonOut+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "text", Argv: []string{textScript}, Encoder: bridge.EncoderText},
		{Name: "json", Argv: []string{jsonScript}, Encoder: bridge.EncoderJSON},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// A daemon stops scripts that are already reading. Each `cat >` creates its
	// file when it starts, so both files existing is that precondition; without
	// it a loaded host can spend Shutdown's bound starting the shells.
	waitFor(t, "both scripts running", func() bool {
		_, textErr := os.Stat(textOut)
		_, jsonErr := os.Stat(jsonOut)
		return textErr == nil && jsonErr == nil
	})

	fleet.Shutdown()
	cancel()
	fleet.Stop()

	wantText, err := bridge.ShutdownLine(bridge.FormatV4Text)
	if err != nil {
		t.Fatalf("ShutdownLine: %v", err)
	}
	if got := readFile(t, textOut); got != string(wantText) {
		t.Errorf("the text script read %q before it was killed, want %q", got, wantText)
	}
	if got := readFile(t, jsonOut); !strings.Contains(got, `"notification": "shutdown"`) {
		t.Errorf("the json script read %q before it was killed, want ExaBGP's JSON shutdown notification", got)
	}
}

// TestFleetShutdownIsBoundedForAScriptThatIgnoresItsEOF checks the wait has a
// limit. The script never reads its stdin and never exits on its own, so only
// the bound ends Shutdown; without one, the daemon's stop would wait for it
// forever.
func TestFleetShutdownIsBoundedForAScriptThatIgnoresItsEOF(t *testing.T) {
	dir := t.TempDir()
	deaf := writeScript(t, dir, "deaf.sh", "exec sleep 30\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "deaf", Argv: []string{deaf}, Encoder: bridge.EncoderText},
	})
	if err := fleet.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	began := time.Now()
	fleet.Shutdown()
	took := time.Since(began)

	cancel()
	fleet.Stop()

	if took < shutdownWait {
		t.Errorf("Shutdown returned after %v for a script that never exits, want the %v bound", took, shutdownWait)
	}
	if took > 3*shutdownWait {
		t.Errorf("Shutdown took %v, want about the %v bound", took, shutdownWait)
	}
}

// TestFleetShutdownSkipsAScriptThatNeverStarted checks a fleet whose Start
// failed: a script Start never reached has no writer and no child, so Shutdown
// has nothing to wait for and MUST NOT sit out the bound for it.
func TestFleetShutdownSkipsAScriptThatNeverStarted(t *testing.T) {
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "missing", Argv: []string{filepath.Join(t.TempDir(), "absent")}, Encoder: bridge.EncoderText},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := fleet.Start(ctx, &recorder{}); err == nil {
		t.Fatal("Start of a script that does not exist answered no error")
	}

	began := time.Now()
	fleet.Shutdown()
	if took := time.Since(began); took >= shutdownWait {
		t.Errorf("Shutdown took %v for a fleet with no started script, want no wait", took)
	}

	cancel()
	fleet.Stop()
}
