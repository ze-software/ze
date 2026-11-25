// Design: docs/architecture/chaos-web-dashboard.md -- route propagation validation

package validation

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/peer"
)

type peerSynchronization struct {
	eor             bool
	expectFault     bool
	recoveryPending bool
	recovered       bool
}

// EventProcessor routes peer events to the validation model, tracker,
// and convergence tracker. It also maintains aggregate counters.
type EventProcessor struct {
	Model           *Model
	Tracker         *Tracker
	Convergence     *Convergence
	sessions        []peerSynchronization
	expectsReceipts bool

	// Aggregate counters for the exit summary.
	Announced     int
	Received      int
	ChaosEvents   int
	Reconnections int
	Withdrawn     int
	DroppedEvents int
}

// NewEventProcessor starts an empty observation of the configured peers.
func NewEventProcessor(peers int, deadline time.Duration, reachable Reachability) *EventProcessor {
	return &EventProcessor{
		Model:       NewModelWithReachability(peers, reachable),
		Tracker:     NewTracker(peers),
		Convergence: NewConvergenceWithReachability(peers, deadline, reachable),
		sessions:    make([]peerSynchronization, peers),
	}
}

// ScenarioReachability applies the scenario's family and forwarding scope.
// When ibgp is supplied, the generated Ze route server only forwards its
// clients' routes to external destinations.
func ScenarioReachability(ibgp []bool, families [][]string) Reachability {
	if len(ibgp) == 0 && len(families) == 0 {
		return nil
	}
	return func(source, destination int, family string) bool {
		if len(ibgp) != 0 && ibgp[source] && !ibgp[destination] {
			return false
		}
		if family == "" {
			return true
		}
		if len(families) == 0 {
			return true
		}
		supported := families[destination]
		return len(supported) == 0 || slices.Contains(supported, family)
	}
}

// ScenarioProperties resolves the properties selected through the CLI or an event log.
func ScenarioProperties(selection string, peers int, deadline time.Duration, reachable Reachability) (*PropertyEngine, error) {
	all := AllPropertiesWithReachability(peers, deadline, reachable)
	if selection == "all" {
		return NewPropertyEngine(all), nil
	}
	var names []string
	for name := range strings.SplitSeq(selection, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, errors.New("--properties requires at least one property name")
	}
	selected, err := SelectProperties(all, names)
	if err != nil {
		return nil, err
	}
	return NewPropertyEngine(selected), nil
}

// UnsynchronizedPeers returns peers without a synchronized live session.
// A peer can finish during a later intentional disconnect after it has already
// demonstrated recovery from an earlier one.
func (ep *EventProcessor) UnsynchronizedPeers() []int {
	var incomplete []int
	for index, session := range ep.sessions {
		up := ep.Model.Established(index)
		if up {
			if !session.eor {
				incomplete = append(incomplete, index)
			}
			continue
		}
		if !session.recovered || !session.recoveryPending {
			incomplete = append(incomplete, index)
		}
	}
	return incomplete
}

// MissingAllReceipts reports whether the scenario exercised a forwarding path
// without observing any deliveries.
func (ep *EventProcessor) MissingAllReceipts() bool {
	return ep.expectsReceipts && ep.Received == 0
}

// Process handles a single peer event, updating all validation components.
func (ep *EventProcessor) Process(ev peer.Event) {
	if ev.PlannedShutdown {
		return
	}
	var sync *peerSynchronization
	if ev.PeerIndex >= 0 && ev.PeerIndex < len(ep.sessions) {
		sync = &ep.sessions[ev.PeerIndex]
	}
	switch ev.Type {
	case peer.EventEstablished:
		ep.Model.SetEstablished(ev.PeerIndex, true)

	case peer.EventRouteSent:
		route := makeRouteKey(ev.Prefix, ev.Family)
		ep.Model.announce(ev.PeerIndex, route)
		if ev.Prefix.IsValid() {
			ep.Convergence.recordAnnounce(ev.PeerIndex, route, ev.Time, ev.Family)
		}
		if sync != nil && !ep.expectsReceipts {
			for destination := range ep.sessions {
				if ep.Model.reachable.allows(ev.PeerIndex, destination, ev.Family) {
					ep.expectsReceipts = true
					break
				}
			}
		}
		ep.Announced++

	case peer.EventRouteReceived:
		route := makeRouteKey(ev.Prefix, ev.Family)
		ep.Tracker.recordReceive(ev.PeerIndex, route)
		ep.Convergence.recordReceive(ev.PeerIndex, route, ev.Time)
		ep.Received++

	case peer.EventRouteWithdrawn:
		ep.Tracker.recordWithdraw(ev.PeerIndex, makeRouteKey(ev.Prefix, ev.Family))

	case peer.EventDisconnected:
		ep.Model.Disconnect(ev.PeerIndex)
		ep.Tracker.ClearPeer(ev.PeerIndex)
		ep.Convergence.disconnect(ev.PeerIndex, ev.Time)
		if sync != nil {
			sync.eor = false
			sync.recoveryPending = sync.expectFault
			sync.expectFault = false
		}

	case peer.EventEORSent:
		if sync != nil {
			sync.eor = true
			if sync.recoveryPending {
				sync.recovered = true
				sync.recoveryPending = false
			}
		}

	case peer.EventError:
		// Errors are logged by the caller.

	case peer.EventChaosExecuted:
		ep.ChaosEvents++
		if action, ok := engine.ActionTypeFromString(ev.ChaosAction); ok && sync != nil && action.NeedsReconnect() {
			sync.expectFault = true
		}

	case peer.EventReconnecting:
		ep.Reconnections++

	case peer.EventWithdrawalSent:
		for _, prefix := range ev.WithdrawnPrefixes {
			route := makeRouteKey(prefix, ev.Family)
			ep.Model.withdraw(ev.PeerIndex, route)
			ep.Convergence.recordWithdraw(ev.PeerIndex, route, ev.Time)
		}
		ep.Withdrawn += ev.Count

	case peer.EventRouteAction:
		// Route dynamics actions are informational — counted by the web dashboard.

	case peer.EventDroppedEvents:
		ep.DroppedEvents += ev.Count
	default:
		panic("BUG: invalid validation event type")
	}
}
