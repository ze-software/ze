// Design: docs/architecture/api/architecture.md -- Attaching a Process to a Peer.
// Related: bgp_best_route_transit.go -- ordinary transit on original sockets.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	attachIsolationUnattached = "attach-isolation-unattached"
	attachIsolationSource     = "attach-isolation-source"
	attachIsolationFirst      = "10.252.1.0/24"
	attachIsolationSecond     = "10.252.2.0/24"
)

func init() {
	Register("plugin/bgp-best-route-attach-isolation", bgpBestRouteAttachIsolation, StopsDaemon)
	Register("plugin/bgp-best-route-attach-observer", bgpBestRouteAttachObserver)
}

func bgpBestRouteAttachIsolation(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("selected attachment isolation requires <BGP listen address>")
	}
	return Observe(ctx, "attach-isolation-driver", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		source, destination, err := bgpBestRouteConnect(ctx, plugin, args[0], 65001, 65000)
		if err != nil {
			return err
		}
		defer source.Close()      //nolint:errcheck // All assertions use these original sockets.
		defer destination.Close() //nolint:errcheck // All assertions use these original sockets.
		if err := attachIsolationExchange(ctx, plugin, source, destination, 1, attachIsolationFirst); err != nil {
			return err
		}
		// The first settlement joins actual delivery from both peers, then each
		// observer attempts its own authenticated sends before it acknowledges.
		if err := quiesce(ctx, plugin); err != nil {
			return err
		}
		// The next exact UPDATE on each original socket also rejects any stray
		// wire output from a refused send. Join both observers a second time.
		if err := attachIsolationExchange(ctx, plugin, source, destination, 2, attachIsolationSecond); err != nil {
			return err
		}
		if err := quiesce(ctx, plugin); err != nil {
			return err
		}
		if err := policyFinalSessions(ctx, plugin, 2); err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stderr, "OK: attachment isolation kept both original selected-route sessions useful")
		return err
	})
}

func attachIsolationExchange(ctx context.Context, plugin *sdk.Plugin, source, destination net.Conn, phase byte, sourcePrefix string) error {
	sourceNLRI := []byte{24, 10, 252, phase}
	if err := bgpBestRouteSend(source, sourceNLRI, false, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, sourcePrefix, addrLoopbackSecond); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(destination, sourceNLRI, nil, []byte{2, 1, 0xfd, 0xe9}); err != nil {
		return fmt.Errorf("source to destination, round %d: %w", phase, err)
	}
	// The other peer sends a different route and its selected output reaches
	// the first peer. Its ingress is real even though the source-only observer
	// may see only the corresponding outgoing UPDATE at its attached peer.
	destinationNLRI := []byte{24, 10, 251, phase}
	if err := bgpBestRouteSend(destination, destinationNLRI, true, false); err != nil {
		return err
	}
	if err := rfc8212ReloadBest(ctx, plugin, fmt.Sprintf("10.251.%d.0/24", phase), addrLoopbackThird); err != nil {
		return err
	}
	if err := bgpBestRouteExpect(source, destinationNLRI, nil, []byte{2, 1, 0xfd, 0xe8}); err != nil {
		return fmt.Errorf("destination to source, round %d: %w", phase, err)
	}
	return nil
}

// Both observers ask for the same UPDATE events. Only the source observer has
// a peer attachment. The settlement worker runs after the engine has joined
// applied callbacks, and can make the authenticated send requests of its own.
func bgpBestRouteAttachObserver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("selected attachment observer requires its configured process name")
	}
	name := args[0]
	switch name {
	case attachIsolationUnattached, attachIsolationSource:
	default:
		return fmt.Errorf("unknown selected attachment observer %q", name)
	}
	plugin, err := newObserver(name)
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // Run owns the transport verdict.
	events := attachIsolationEvents{sourceOnly: name == attachIsolationSource, announced: make(map[string]bool)}
	plugin.SetStartupSubscriptions([]string{eventUpdate}, nil, "")
	plugin.OnEvent(events.record)
	settlements := 0
	plugin.OnEventSettlement(func(ctx context.Context) error {
		if err := events.verify(settlements != 0); err != nil {
			failure := fmt.Errorf("%s: %w", name, err)
			ReportFailure(failure)
			return failure
		}
		settlements++
		switch settlements {
		case 1:
			if err := attachIsolationRefusedSends(ctx, plugin, name); err != nil {
				ReportFailure(err)
				return err
			}
		case 2:
			_, err := fmt.Fprintf(os.Stderr, "OK: %s kept its UPDATE scope and both sends were refused\n", name)
			return err
		}
		return nil
	})
	return plugin.Run(ctx, &sdk.Registration{})
}

func attachIsolationRefusedSends(ctx context.Context, plugin *sdk.Plugin, name string) error {
	for _, peer := range []string{addrLoopbackSecond, addrLoopbackThird} {
		_, _, err := plugin.UpdateRoute(ctx, peer, "update text nhop 192.0.2.1 nlri add 203.0.113.0/24")
		if err == nil {
			return fmt.Errorf("%s: send to %s was accepted", name, peer)
		}
		refusal := err.Error()
		for _, expected := range []string{"send refused", name, peer} {
			if !strings.Contains(refusal, expected) {
				return fmt.Errorf("%s: send to %s refusal omitted %q: %w", name, peer, expected, err)
			}
		}
	}
	return nil
}

type attachIsolationEvents struct {
	mu         sync.Mutex
	sourceOnly bool
	unexpected string
	announced  map[string]bool
}

func (events *attachIsolationEvents) record(raw string) error {
	name, kind, direction := eventFacts02(raw)
	if kind != eventUpdate {
		return nil
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	switch {
	case !events.sourceOnly, name != peerNameSource:
		if events.unexpected == "" {
			events.unexpected = fmt.Sprintf("UPDATE for peer %q (%s)", name, direction)
		}
		return nil
	}
	if direction != directionReceived {
		return nil
	}
	update, _ := eventBody02(raw)["update"].(map[string]any)
	nlri, _ := update["nlri"].(map[string]any)
	entries, _ := nlri[familyIPv4Unicast].([]any)
	for _, rawEntry := range entries {
		entry, _ := rawEntry.(map[string]any)
		if entry["action"] != eventActionAdd {
			continue
		}
		prefixes, _ := entry["nlri"].([]any)
		for _, rawPrefix := range prefixes {
			prefix, _ := rawPrefix.(string)
			events.announced[prefix] = true
		}
	}
	return nil
}

func (events *attachIsolationEvents) verify(second bool) error {
	events.mu.Lock()
	defer events.mu.Unlock()
	if events.unexpected != "" {
		return fmt.Errorf("received an unauthorized %s", events.unexpected)
	}
	if !events.sourceOnly {
		return nil
	}
	if !events.announced[attachIsolationFirst] {
		return fmt.Errorf("missing received source announcement %s, got %v", attachIsolationFirst, events.announced)
	}
	if second {
		if !events.announced[attachIsolationSecond] {
			return fmt.Errorf("missing received source announcement %s, got %v", attachIsolationSecond, events.announced)
		}
	}
	return nil
}
