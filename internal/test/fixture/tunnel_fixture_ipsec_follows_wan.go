// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the interface-bound local address
// Related: register_ipsec_follows_wan.go -- the registration
// Related: tunnel_fixture_ipsec.go -- tunnelIPsecWait, tunnelIPsecSPIs, tunnelIPsecCommand

package fixture

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// The dummy WAN interface and the two addresses it holds in turn. The names match
// test/ipsec/ipsec-follows-wan-address.ci, which configures `vpn ipsec interface` with
// followsWANLink and gives the responder one peer for each address.
const (
	followsWANLink   = "zewantest0"
	followsWANFirst  = "198.18.250.1"
	followsWANSecond = "198.18.250.7"
	followsWANPeer   = "127.0.0.2"
	followsWANLit    = "127.0.0.1"
)

// tunnelIPsecFollowsWAN drives the interface through arrival, change and removal and
// reads the kernel's ESP states after each step. Every wait polls the state it needs, and
// the literal peer's SPIs are compared after every step, so a rebind that bounced it is
// caught at the step that did. The interface is created here, after both daemons run, so
// the engine starts with an interface that has no address, and it is deleted on every
// exit.
func tunnelIPsecFollowsWAN(ctx context.Context, _ []string) error {
	states := func(local string) func() ([]string, error) {
		return func() ([]string, error) {
			return tunnelIPsecSPIs(ctx, map[string]bool{local: true, followsWANPeer: true})
		}
	}
	installed := func(values []string) bool { return len(values) >= 2 }
	gone := func(values []string) bool { return len(values) == 0 }

	literal, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, states(followsWANLit), installed)
	if err != nil {
		return fmt.Errorf("the literal peer never installed its Child SA: %w", err)
	}
	fmt.Println("LITERAL:", strings.Join(literal, " "))
	keepLiteral := func(step string) error {
		now, readErr := states(followsWANLit)()
		if readErr != nil {
			return readErr
		}
		if !tunnelSameStrings(now, literal) {
			return fmt.Errorf("BOUNCED at %s: the literal peer's ESP states moved: before=%v after=%v", step, literal, now)
		}
		return nil
	}

	// AC-1: no address, so no Child SA can exist for the interface-bound peer.
	early, err := states(followsWANFirst)()
	if err != nil {
		return err
	}
	if len(early) != 0 {
		return fmt.Errorf("an ESP state exists for %s before the interface has the address: %v", followsWANFirst, early)
	}

	if _, err := tunnelIPsecCommand(ctx, "ip", "link", "add", followsWANLink, "type", "dummy"); err != nil {
		return err
	}
	defer func() {
		if _, delErr := tunnelIPsecCommand(context.Background(), "ip", "link", "del", followsWANLink); delErr != nil {
			fmt.Println("CLEANUP-FAILED:", delErr)
		}
	}()
	if _, err := tunnelIPsecCommand(ctx, "ip", "link", "set", followsWANLink, "up"); err != nil {
		return err
	}

	// AC-2: the address appears, and the waiting peer starts from it.
	if _, err := tunnelIPsecCommand(ctx, "ip", "addr", "add", followsWANFirst+"/32", "dev", followsWANLink); err != nil {
		return err
	}
	arrived, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, states(followsWANFirst), installed)
	if err != nil {
		return fmt.Errorf("no Child SA from %s after the address appeared: %w", followsWANFirst, err)
	}
	fmt.Println("ARRIVED:", strings.Join(arrived, " "))
	if err := keepLiteral("arrival"); err != nil {
		return err
	}

	// AC-3: the address changes; the peer restarts from the new one and the old states go.
	if _, err := tunnelIPsecCommand(ctx, "ip", "addr", "add", followsWANSecond+"/32", "dev", followsWANLink); err != nil {
		return err
	}
	if _, err := tunnelIPsecCommand(ctx, "ip", "addr", "del", followsWANFirst+"/32", "dev", followsWANLink); err != nil {
		return err
	}
	moved, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, states(followsWANSecond), installed)
	if err != nil {
		return fmt.Errorf("no Child SA from %s after the address changed: %w", followsWANSecond, err)
	}
	if _, err := tunnelIPsecWait(ctx, 200, 250*time.Millisecond, states(followsWANFirst), gone); err != nil {
		return fmt.Errorf("the Child SA from the departed %s stayed in the kernel: %w", followsWANFirst, err)
	}
	fmt.Println("MOVED:", strings.Join(moved, " "))
	if err := keepLiteral("change"); err != nil {
		return err
	}

	// AC-4: the address leaves; the peer stops and removes its Child SA.
	if _, err := tunnelIPsecCommand(ctx, "ip", "addr", "del", followsWANSecond+"/32", "dev", followsWANLink); err != nil {
		return err
	}
	if _, err := tunnelIPsecWait(ctx, 200, 250*time.Millisecond, states(followsWANSecond), gone); err != nil {
		return fmt.Errorf("the Child SA from %s stayed after the address was removed: %w", followsWANSecond, err)
	}
	fmt.Println("REMOVED:", followsWANSecond)
	if err := keepLiteral("removal"); err != nil {
		return err
	}
	fmt.Println("LITERAL-STABLE:", strings.Join(literal, " "))
	return nil
}
