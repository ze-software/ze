package bridgerun

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// announced translates one announce line into the command the runner notes.
func announced(t *testing.T, line string) bridge.Command {
	t.Helper()
	translation, err := bridge.TranslateLine(line)
	if err != nil || len(translation.Commands) != 1 {
		t.Fatalf("translate %q: %v %v", line, translation.Commands, err)
	}
	return translation.Commands[0]
}

// exitWithdrawal is the command an exit sends for a route whose builder wrote
// withdrawal: the same, asking ze to give back a configured route
// (bridge.OwnerLedger, restoringWithdrawal; its own test is in the bridge).
func exitWithdrawal(withdrawal string) string {
	return strings.Replace(withdrawal, " update text ", " update text restore-configured ", 1)
}

// TestExitWithdrawsOwnedRoutes runs a v6 and a v4 script that each announce a
// route and exit (X-21, X-22, AC-22, AC-23): the v6 script's route is withdrawn
// with its own withdrawal command and flushed, the v4 script's is not.
func TestExitWithdrawsOwnedRoutes(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "exited")
	v6 := writeScript(t, dir, "v6.sh",
		"echo 'peer * announce route 10.6.0.0/24 next-hop 1.2.3.4'\nread done\n")
	v4 := writeScript(t, dir, "v4.sh",
		"echo 'announce route 10.4.0.0/24 next-hop 1.2.3.4'\nread done\ntouch "+marker+"\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &recorder{}
	fleet := New(slogutil.DiscardLogger(), testFamilies, []Script{
		{Name: "v6", Argv: []string{v6}, Encoder: bridge.EncoderJSON, Respawn: false},
		{Name: "v4", Argv: []string{v4}, Encoder: bridge.EncoderJSON, Respawn: false},
	})
	if err := fleet.Start(ctx, rec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	v6Withdrawal := exitWithdrawal(announced(t, "peer * announce route 10.6.0.0/24 next-hop 1.2.3.4").Withdrawal)
	v4Withdrawal := exitWithdrawal(announced(t, "announce route 10.4.0.0/24 next-hop 1.2.3.4").Withdrawal)
	waitFor(t, "the v6 script's route to be withdrawn on exit", func() bool { return rec.held(v6Withdrawal) })
	waitFor(t, "the v4 script to exit", func() bool {
		_, err := os.Stat(marker)
		return err == nil
	})
	time.Sleep(100 * time.Millisecond) // the v4 exit is settled after its child is reaped

	cancel()
	fleet.Stop()

	seen := rec.seen()
	if slices.Contains(seen, v4Withdrawal) {
		t.Errorf("the v4 script's route was withdrawn; API 4 keeps it: %v", seen)
	}
	at := slices.Index(seen, v6Withdrawal)
	if at < 0 || at+1 >= len(seen) || !strings.HasSuffix(seen[at+1], " flush") {
		t.Errorf("the exit withdrawal is not followed by its flush: %v", seen)
	}
}
