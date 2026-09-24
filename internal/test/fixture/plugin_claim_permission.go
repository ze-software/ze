package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/peer-up-replay-claim-permission", eventObserver02("claim-permission", []string{eventState}, observeReplayClaimPermissions))
}

// observeReplayClaimPermissions consumes the actual onPeerStateChange JSON, not
// a caller-supplied UnheldRoles fixture. Both denied and authorized destinations
// are required so missing claim advertisement cannot make the check pass.
func observeReplayClaimPermissions(ctx context.Context, p *sdk.Plugin, events <-chan string, shutdown <-chan struct{}) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	wantRetracted := map[string]bool{"observer": true, "raw-only": true, "writer": false}
	seen := make(map[string]bool, len(wantRetracted))
	for len(seen) != len(wantRetracted) {
		raw, ok := nextEvent02(ctx, events, shutdown, 25*time.Second)
		if !ok {
			return fmt.Errorf("replay claim state events incomplete: observed %v", seen)
		}
		var event struct {
			BGP struct {
				Message struct {
					Type string `json:"type"`
				} `json:"message"`
				Peer struct {
					Name string `json:"name"`
				} `json:"peer"`
				State       string   `json:"state"`
				UnheldRoles []string `json:"unheld-roles"`
			} `json:"bgp"`
		}
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return fmt.Errorf("decode replay claim state event: %w", err)
		}
		if event.BGP.Message.Type != eventState || event.BGP.State != "up" {
			continue
		}
		name := event.BGP.Peer.Name
		want, known := wantRetracted[name]
		if !known {
			return fmt.Errorf("unexpected replay claim peer %q", name)
		}
		got := slices.Contains(event.BGP.UnheldRoles, "bgp-peer-up-replay")
		if got != want {
			return fmt.Errorf("peer %s: replay claim retracted=%t, want %t; delivered roles=%v", name, got, want, event.BGP.UnheldRoles)
		}
		seen[name] = true
	}
	// The peer carriers assert their EOR. Do not shut down after observing only
	// our metadata while those independent wire assertions are still pending.
	for _, name := range []string{"observer", "raw-only", "writer"} {
		if !waitPeerEORSent(ctx, p, name, 50) {
			return fmt.Errorf("peer %s did not receive its initial-sync EOR", name)
		}
	}
	fmt.Fprintf(os.Stderr, "CLAIM-PERMISSION: checked=%d observer=true raw-only=true writer=false\n", len(seen))
	requestShutdownAsync02(ctx, p)
	return waitForShutdown02(ctx, shutdown, 10*time.Second)
}
