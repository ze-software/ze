// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation

package peer

import "fmt"

// String returns a kebab-case human-readable name for the event type.
// Unknown values return "unknown-N" to prevent panics in logging paths.
func (et EventType) String() string {
	switch et {
	case EventEstablished:
		return "established"
	case EventRouteSent:
		return "route-sent"
	case EventRouteReceived:
		return "route-received"
	case EventRouteWithdrawn:
		return "route-withdrawn"
	case EventEORSent:
		return "eor-sent"
	case EventDisconnected:
		return "disconnected"
	case EventError:
		return "error"
	case EventChaosExecuted:
		return "chaos-executed"
	case EventReconnecting:
		return "reconnecting"
	case EventWithdrawalSent:
		return "withdrawal-sent"
	case EventRouteAction:
		return "route-action"
	default:
		return fmt.Sprintf("unknown-%d", int(et))
	}
}
