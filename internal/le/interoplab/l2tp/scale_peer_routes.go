// Design: docs/labs/l2tp-interop.md -- independent subscriber return paths at the LAC.
// Related: scale_observe.go -- exact peer interfaces, traffic and process identities.
package l2tp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// prepareScalePeers MUST run once, after waitScalePeers, in fresh Suite-owned
// namespaces. Suite cleanup MUST remove those namespaces and their rules.
// Each shared LNS address needs a source-specific unbound reply route at the
// LAC; this changes no Ze route and preserves both interface-bound probes.
func prepareScalePeers(ctx context.Context, lab labOperations, population scalePopulation) error {
	for lac := 1; lac <= scaleLACCount; lac++ {
		peer := "kl2tpd-" + strconv.Itoa(lac)
		for number := 1; number <= scaleSessionsPerLAC; number++ {
			user := scaleUsername(lac, number)
			session, exists := population.byUser[user]
			if !exists {
				return fmt.Errorf("peer fixture baseline lacks %s", user)
			}
			if err := scaleCheckPeerPPP(ctx, lab, user); err != nil {
				return fmt.Errorf("peer fixture PID for %s: %w", user, err)
			}
			name := scaleInterface(lac, number)
			if _, err := fmt.Fprintln(os.Stdout, "L2TP-SCALE-PEER-PID", user, name, "validated"); err != nil {
				return err
			}
			lookup := []string{"ip", "-j", "-4", commandRoute, "get", localPPPAddress, "from", session.address.String()}
			before, err := execOutput(ctx, lab, peer, lookup)
			if err != nil {
				return fmt.Errorf("peer fixture route before %s: %w", user, err)
			}
			if _, err := fmt.Fprintln(os.Stdout, "L2TP-SCALE-PEER-ROUTE-BEFORE", user, before); err != nil {
				return err
			}
			// IDs 2001..2048 are unique across the finite 3 x 16 baseline;
			// their priorities precede main (32766), but follow local (0).
			table := 2000 + (lac-1)*scaleSessionsPerLAC + number
			for _, command := range [][]string{
				{"ip", "-4", commandRoute, "add", localPPPAddress + "/32", "dev", name, "src", session.address.String(), "table", strconv.Itoa(table)},
				{"ip", "-4", "rule", "add", "priority", strconv.Itoa(table), "from", netip.PrefixFrom(session.address, 32).String(), "lookup", strconv.Itoa(table)},
			} {
				result, err := lab.Exec(ctx, peer, command, nil)
				if err != nil {
					return interoplab.WithSearchedOutput(fmt.Errorf("peer fixture route setup %s: %w", user, err), result.Stdout+result.Stderr)
				}
			}
			after, err := execOutput(ctx, lab, peer, lookup)
			if err != nil {
				return fmt.Errorf("peer fixture route after %s: %w", user, err)
			}
			if _, err := fmt.Fprintln(os.Stdout, "L2TP-SCALE-PEER-ROUTE-AFTER", user, after); err != nil {
				return err
			}
			if err := requireScalePeerRoute([]byte(after), session.address, name, table); err != nil {
				return interoplab.WithSearchedOutput(fmt.Errorf("peer fixture route for %s: %w", user, err), after)
			}
		}
	}
	return nil
}

// requireScalePeerRoute checks an unbound kernel lookup, not a lookup forced
// onto the desired device with oif. Missing and ambiguous evidence refuse.
func requireScalePeerRoute(raw []byte, address netip.Addr, name string, table int) error {
	if len(raw) > 16384 {
		return errors.New("subscriber return-route answer exceeds 16 KiB")
	}
	var routes []struct {
		Destination netip.Addr      `json:"dst"`
		Source      netip.Addr      `json:"from"`
		Device      string          `json:"dev"`
		Table       json.RawMessage `json:"table"`
	}
	if err := json.Unmarshal(raw, &routes); err != nil {
		return fmt.Errorf("decode subscriber return route: %w", err)
	}
	if len(routes) != 1 {
		return fmt.Errorf("subscriber return-route answer has %d routes, want one", len(routes))
	}
	route := routes[0]
	if route.Destination != netip.MustParseAddr(localPPPAddress) {
		return fmt.Errorf("subscriber return-route destination %s, want %s", route.Destination, localPPPAddress)
	}
	if route.Source != address {
		return fmt.Errorf("subscriber return-route source %s, want %s", route.Source, address)
	}
	if route.Device != name {
		return fmt.Errorf("subscriber return-route device %q, want %s", route.Device, name)
	}
	actual := string(route.Table)
	if len(route.Table) > 0 {
		if route.Table[0] == '"' {
			if err := json.Unmarshal(route.Table, &actual); err != nil {
				return fmt.Errorf("decode subscriber return-route table: %w", err)
			}
		}
	}
	identifier, err := strconv.Atoi(actual)
	if err != nil {
		return fmt.Errorf("subscriber return-route table %q is not a decimal ID: %w", actual, err)
	}
	if identifier != table {
		return fmt.Errorf("subscriber return-route table %d, want %d", identifier, table)
	}
	return nil
}
