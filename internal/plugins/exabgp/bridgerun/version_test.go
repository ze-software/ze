package bridgerun

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	// The config component registers exabgp.api.*, and the bridge reads
	// them through env.Get, which refuses an unregistered key.
	_ "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// TestScriptHeldToDetectedVersion runs a helper whose first deciding line is
// v4, through a real Fleet, and reads back what it was answered and fed.
//
// VALIDATES: X-2 (the first deciding command holds the helper), X-3 (a v6
// command from it answers error and reaches no peer), X-5/X-6 (its JSON events
// carry the v4 envelope), X-7 (`api version` answers this helper's version).
// AC-5, AC-9.
// PREVENTS: a 5.x helper fed 6.0 JSON, or its v6 line dispatched.
func TestScriptHeldToDetectedVersion(t *testing.T) {
	dir := t.TempDir()
	answers := filepath.Join(dir, "answers")
	events := filepath.Join(dir, "events")
	script := writeScript(t, dir, "v4.sh", ""+
		"echo 'announce route 1.1.1.1/32 next-hop 11.11.11.11'; read a; echo \"$a\" >> "+answers+"\n"+
		"echo 'peer * announce route 2.2.2.2/32 next-hop 22.22.22.22'; read a; echo \"$a\" >> "+answers+"; read a; echo \"$a\" >> "+answers+"\n"+
		"echo 'api version'; read a; echo \"$a\" >> "+answers+"; read a; echo \"$a\" >> "+answers+"\n"+
		"cat > "+events+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "v4", Argv: []string{script}, Encoder: bridge.EncoderJSON},
	})
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "the five answers", func() bool { return lines(t, answers) >= 5 })
	got := strings.Split(strings.TrimSpace(readFile(t, answers)), "\n")
	if got[0] != "done" {
		t.Errorf("answer to the v4 announce = %q, want done", got[0])
	}
	// X-13, X-14: the reason in the API 4 form, then the marker.
	if !strings.HasPrefix(got[1], "error: ") || got[2] != "error" {
		t.Errorf("answer to the v6 announce from a v4 helper = %q, %q, want error: <why> then error", got[1], got[2])
	}
	if got[3] != "API version: auto (this helper: 4)" || got[4] != "done" {
		t.Errorf("answer to api version = %q, %q", got[3], got[4])
	}
	for _, command := range rec.seen() {
		if strings.Contains(command, "2.2.2.2/32") {
			t.Errorf("the refused v6 line reached the engine: %q", command)
		}
	}

	fleet.Broadcast(`{"type":"state","peer":{"address":"127.0.0.1"},"state":"up"}`)
	waitFor(t, "the event", func() bool { return strings.Contains(readFile(t, events), "exabgp") })
	if line := readFile(t, events); !strings.Contains(line, `"exabgp": "4.0.1"`) {
		t.Errorf("event to a v4 JSON helper = %q, want the 4.0.1 envelope", line)
	}

	cancel()
	fleet.Stop()
}

// TestFleetRefusesAnInvalidAPIVersionSetting: a setting ExaBGP refuses stops
// the fleet at Start, with the reason and the fix.
func TestFleetRefusesAnInvalidAPIVersionSetting(t *testing.T) {
	t.Setenv("exabgp.api.version", "5")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	fleet := New(slogutil.DiscardLogger(), testFamilies, nil)
	err := fleet.Start(context.Background(), &recorder{})
	if err == nil {
		t.Fatalf("Start accepted exabgp.api.version 5")
	}
	if !strings.Contains(err.Error(), "got 5") || !strings.Contains(err.Error(), "version auto|4|6") {
		t.Errorf("error = %v", err)
	}
	fleet.Stop()
}

// TestAPIVersionLineMovesOnlyItsScript verifies E-5: each wrapper owns its own
// Fleet and its own apiSetting, so an `api version <v>` line from script A
// moves script A's Fleet alone and leaves script B's Fleet unaffected.
//
// VALIDATES: E-5 per-wrapper API version isolation across two independent Fleets.
// PREVENTS: a global or shared API version setting letting one script move another.
func TestAPIVersionLineMovesOnlyItsScript(t *testing.T) {
	dir := t.TempDir()
	answersA := filepath.Join(dir, "a.answers")
	answersB := filepath.Join(dir, "b.answers")

	scriptA := writeScript(t, dir, "a.sh", ""+
		"echo 'api version 4'; read a; echo \"$a\" >> "+answersA+"; read a; echo \"$a\" >> "+answersA+"\n"+
		"echo 'api version'; read a; echo \"$a\" >> "+answersA+"; read a; echo \"$a\" >> "+answersA+"\n"+
		"read never\n")
	scriptB := writeScript(t, dir, "b.sh", ""+
		"echo 'system api version'; read a; echo \"$a\" >> "+answersB+"; read a; echo \"$a\" >> "+answersB+"\n"+
		"read never\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleetA := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "script-a", Argv: []string{scriptA}, Encoder: bridge.EncoderJSON},
	})
	if err := fleetA.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("fleetA Start: %v", err)
	}
	waitFor(t, "script A to change its API version", func() bool {
		return lines(t, answersA) >= 4
	})

	fleetB := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "script-b", Argv: []string{scriptB}, Encoder: bridge.EncoderJSON},
	})
	if err := fleetB.Start(ctx, &recorder{}); err != nil {
		t.Fatalf("fleetB Start: %v", err)
	}
	waitFor(t, "script B to answer its API version", func() bool {
		return lines(t, answersB) >= 2
	})

	cancel()
	fleetA.Stop()
	fleetB.Stop()

	gotA := strings.Split(strings.TrimSpace(readFile(t, answersA)), "\n")
	if len(gotA) < 4 || gotA[2] != "API version: 4 (this helper: 4)" {
		t.Errorf("script A api version answer = %q, want API version: 4 (this helper: 4)", gotA)
	}
	gotB := strings.Split(strings.TrimSpace(readFile(t, answersB)), "\n")
	wantB := `{"api_version": "auto", "helper_api_version": "6"}`
	if len(gotB) < 2 || gotB[0] != wantB {
		t.Errorf("script B api version answer = %q, want %s", gotB, wantB)
	}
}
