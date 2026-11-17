// Design: docs/architecture/behavior/peer-lifecycle.md -- selected OPEN publication.
// Related: role_incoming_open.go -- original-socket and bounded handshake helpers.
// RFC 4271 Sections 6.8 and 8.2.1 -- an unselected connection cannot replace its sibling's learned state.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/incoming-open-learned-isolation", incomingOpenLearnedIsolation, StopsDaemon)
}

// incomingOpenLearnedIsolation contrasts one losing IPv6-only OPEN with useful
// IPv4 forwarding on both original connections. The route server's ordinary
// plugin rail and its operator-visible family set must retain the selected OPEN.
func incomingOpenLearnedIsolation(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("incoming-open-learned-isolation requires the runner's listener address")
	}
	return Observe(ctx, "incoming-open-watch", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := roleIncomingWaitActive(ctx, plugin); err != nil {
			return err
		}
		source, err := roleIncomingConnect(ctx, args[0], roleIncomingSource, 65001)
		if err != nil {
			return err
		}
		defer source.Close() //nolint:errcheck // The owned socket outlives every assertion.
		dest, err := roleIncomingConnect(ctx, args[0], roleIncomingDest, 65002)
		if err != nil {
			return err
		}
		defer dest.Close() //nolint:errcheck // The owned socket outlives every assertion.
		if err := p12WaitPeersEOR(ctx, plugin, "source-peer", "dest-peer"); err != nil {
			return err
		}
		for _, conn := range []net.Conn{source, dest} {
			initial, err := atomicAggregatePolicyRead(conn)
			if err != nil {
				return err
			}
			if len(initial.NLRI)+len(initial.WithdrawnRoutes)+len(initial.PathAttributes) != 0 {
				return errors.New("original peer did not receive its empty initial End-of-RIB")
			}
		}
		if err := roleIncomingUseful(source, dest, 0); err != nil {
			return fmt.Errorf("baseline selected IPv4 export: %w", err)
		}
		if err := incomingOpenSelectedFamilies(ctx, plugin); err != nil {
			return fmt.Errorf("baseline route-server state: %w", err)
		}
		if err := incomingOpenIPv6Loser(ctx, args[0]); err != nil {
			return err
		}
		// The losing OPEN is delivered before this source UPDATE on the same
		// route-server event queue. Forwarding fences that earlier publication;
		// checking an unchanged query alone could run before a queued bad OPEN.
		if err := roleIncomingUseful(source, dest, 1); err != nil {
			return fmt.Errorf("selected IPv4 export after losing IPv6-only OPEN: %w", err)
		}
		if err := incomingOpenSelectedFamilies(ctx, plugin); err != nil {
			return fmt.Errorf("losing OPEN changed route-server learned state: %w", err)
		}
		if err := roleIncomingUnchanged(ctx, plugin); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: losing IPv6-only OPEN received collision6/7 and EOF; route server retained selected IPv4 family and ASN; fresh IPv4 export reached the original Customer socket")
		return nil
	})
}

func incomingOpenSelectedFamilies(ctx context.Context, plugin *sdk.Plugin) error {
	data, err := plugin01RequireDone(ctx, plugin, "show bgp rs peers")
	if err != nil {
		return err
	}
	rows, ok := data["peers"].([]any)
	if !ok {
		return errors.New("show bgp rs peers did not return peer rows")
	}
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return errors.New("route-server peer row is not an object")
		}
		if row["address"] != roleIncomingDest {
			continue
		}
		if row["up"] != true {
			return fmt.Errorf("original destination is not up in route server: %v", row)
		}
		remote, ok := row["remote"].(map[string]any)
		if !ok {
			return errors.New("route-server destination has no remote AS object")
		}
		if plugin01Number(remote["as"]) != 65002 {
			return fmt.Errorf("route-server destination AS changed: %v", remote)
		}
		values, ok := row["families"].([]any)
		if !ok {
			return fmt.Errorf("route-server destination has no recorded family list: %v", row)
		}
		families := make([]string, 0, len(values))
		for _, value := range values {
			name, ok := value.(string)
			if !ok {
				return errors.New("route-server family is not text")
			}
			families = append(families, name)
		}
		if !slices.Equal(families, []string{familyIPv4Unicast}) {
			return fmt.Errorf("route-server destination families=%v, want only ipv4/unicast", families)
		}
		return nil
	}
	return errors.New("original destination is missing from route-server peer rows")
}

func incomingOpenIPv6Loser(ctx context.Context, address string) error {
	conn, err := roleIncomingDial(ctx, address, roleIncomingDest)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // Remote EOF is observed before local release.
	// MP IPv6/unicast plus Customer role: local configuration permits both
	// families, so this valid OPEN loses specifically to the Established sibling.
	open := &message.Open{Version: 4, MyAS: 65002, HoldTime: 0,
		BGPIdentifier: 0x09090909, OptionalParams: []byte{2, 9, 1, 4, 0, 2, 0, 1, 9, 1, 3}}
	packet := message.PackTo(open, nil)
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return fmt.Errorf("read competing connection's local OPEN: %w", err)
	}
	response, err := io.ReadAll(io.LimitReader(conn, int64(message.HeaderLen+3)))
	if err != nil {
		return err
	}
	if !slices.Equal(response, roleIncomingNotification(6, 7)) {
		return fmt.Errorf("competing IPv6-only OPEN received %x, want exact collision6/7 and EOF", response)
	}
	return nil
}
