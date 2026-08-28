// Design: docs/architecture/api/architecture.md -- the send half of an attach block
// Related: reactor_api.go -- getMatchingPeersSel, the one resolver every send command uses
// Related: delivery_graph.go -- the receive half, indexed in the plugin server

package reactor

import (
	"errors"
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// sendOrigin says who is about to generate a BGP message toward a peer, and
// what kind of message it is. A peer's
// `attach process <name> { send [ ... ] }` block is the permission, so the
// resolver needs both halves before it can apply it.
type sendOrigin struct {
	// sender is who issued the command, in the three states plugin.Sender
	// distinguishes.
	//
	// A named PROCESS is gated on the peer's attach block. The OPERATOR is
	// exempt: an operator command carries the operator's own authority, checked
	// by AAA before dispatch (dispatcher.IsAuthorized), and `send` grants
	// authority to a process, never to a person, so an operator who may run
	// `bgp peer 192.0.2.1 refresh` is not stopped by the absence of an attach
	// block. The zero Sender says NOBODY named the issuer, and the resolver
	// refuses it: reading it as the operator would hand operator authority to
	// any dispatch path that forgot to name its sender, on the one guard whose
	// purpose is to stop a process reaching a peer that never attached it
	// (ai/rules/evidence.md).
	//
	// Each dispatch path states the sender where the command enters the daemon:
	// plugin.ProcessSender on the plugin server's own paths
	// (plugin/server/dispatch.go, dispatch_registry.go), plugin.OperatorSender
	// on the operator surfaces (cmd/ze/hub/main_servers.go, service_ssh.go,
	// api.go).
	sender plugin.Sender
	// sendType is the message type the command is about to put on the wire:
	// bgpevents.SendUpdate for anything that builds an UPDATE (an announce, a
	// withdrawal, an End-of-RIB marker, a cache forward, a stored-route relay),
	// bgpevents.SendRefresh for a ROUTE-REFRESH (a refresh, a BoRR, an EoRR, a
	// soft clear), sendTypeRaw for a raw injection.
	sendType string
	// attachOnly asks the weaker question: does this peer attach the process at
	// all, whatever its send list grants. Raw injection asks it, because the
	// bytes it carries are a whole BGP message of any type and the send list has
	// no word for that (see sendTypeRaw).
	attachOnly bool
}

// sendTypeRaw labels a refused raw injection in the log line and in the
// ze_bgp_send_refused_total counter.
//
// It is NOT a token the `send` list accepts, and the YANG validator does not
// know it. A raw injection carries a whole BGP message chosen by the caller, an
// OPEN or a NOTIFICATION included, so no existing send type describes it and
// minting one is a vocabulary decision for the owner rather than a decision this
// guard may take. What raw is gated on instead is the weaker permission the
// vocabulary already implies: the peer must attach the process at all.
const sendTypeRaw = "raw"

// announceOrigin is the origin of a command that puts an UPDATE on the wire.
func announceOrigin(sender plugin.Sender) sendOrigin {
	return sendOrigin{sender: sender, sendType: bgpevents.SendUpdate}
}

// refreshOrigin is the origin of a command that puts a ROUTE-REFRESH on the wire.
func refreshOrigin(sender plugin.Sender) sendOrigin {
	return sendOrigin{sender: sender, sendType: bgpevents.SendRefresh}
}

// rawOrigin is the origin of a raw injection: caller-supplied bytes written to
// one peer's socket with no message built around them.
func rawOrigin(sender plugin.Sender) sendOrigin {
	return sendOrigin{sender: sender, sendType: sendTypeRaw, attachOnly: true}
}

// errSendNotPermitted is returned when a selector matched peers and every one
// of them refused the process. The peers, the process and the message type are
// in the wrapped message, because the process that issued the command is the
// one reader who can fix it (ai/rules/cli.md).
var errSendNotPermitted = errors.New("send refused: no peer this selector names attaches this process with the required send permission")

// errSendNoSender is returned when a send command reaches the resolver and
// nothing said who issued it. There is no attach block to consult for an
// unnamed issuer, and the operator's exemption belongs to the operator, so the
// command is refused. It is a defect in the dispatch path that produced the
// command, not an operator or process mistake, so the message names the field
// to set (ai/rules/evidence.md).
var errSendNoSender = errors.New("send refused: the command names no sender, so no attach block can permit it; the dispatch path must set CommandContext.Sender")

// maySend reports whether this peer's config permits one process to generate
// one message type toward it.
//
// It FAILS CLOSED, and every branch that returns false is a deliberate no: a
// sender that names no process, a peer that attaches nothing, a peer that
// attaches other processes only, and a binding whose `send` list omits the type
// all deny. A missing block is the operator saying nothing about this process,
// and saying nothing is not a grant (ai/rules/evidence.md).
//
// An origin marked attachOnly stops at the block: the process must be attached,
// and the send list is not read. Only rawOrigin sets it.
//
// Reads p.settings directly rather than through settingsSnapshot, which copies
// the struct under p.mu. The fields read here are written once, when the peer is
// built, and no later path writes them, so the copy would buy nothing and this
// runs once per send command per matched peer. Settings() returns the same
// pointer and would read identically; the direct field is used because the
// caller already holds r.mu and the intent is a field read, not a handle.
func (p *Peer) maySend(origin sendOrigin) bool {
	process, ok := origin.sender.Process()
	if !ok {
		return false
	}
	for i := range p.settings.ProcessBindings {
		b := &p.settings.ProcessBindings[i]
		if b.PluginName != process {
			continue
		}
		if origin.attachOnly || b.MaySend(origin.sendType) {
			return true
		}
	}
	return false
}

// filterPermittedPeers keeps the peers whose config permits origin, reports
// every refusal, and returns how many it refused.
//
// It is the ONE place the send permission is applied. Every rail that puts a
// message on a peer's wire calls it, so all of them inherit the same three
// answers: the operator is exempt, a process reaches the peers whose attach
// block grants it the type, and a peer that grants nothing is dropped and
// reported. A rail that filtered its own destinations would be a second guard
// with its own bugs, and the four rails that had NO guard between them are why
// this one is shared (spec-fixit-peer-process-event-filter, Review Gate round 1).
//
// The caller builds the error, because only the caller can name what it
// addressed: a selector, a destination list, one peer. That also keeps the
// permitted path free of the string, which matters on the route-server flush
// rail where this runs once per batch.
//
// The caller MUST have refused an unset sender already: this reads a sender that
// names no process as a process nobody attaches, which is the right answer for a
// process and the wrong one for a command with no issuer at all.
//
// Filters IN PLACE, so the caller must not reuse the slice it passed. The caller
// must hold r.mu (read or write): this reads each peer's settings in place.
func filterPermittedPeers(matched []*Peer, origin sendOrigin) (permitted []*Peer, refused int) {
	if origin.sender.IsOperator() {
		return matched, 0
	}
	permitted = matched[:0]
	for _, peer := range matched {
		if peer.maySend(origin) {
			permitted = append(permitted, peer)
			continue
		}
		refused++
		sendPermissionDenied(peer, origin)
	}
	return permitted, refused
}

// sendPermissionDenied reports one refused peer, once per refusal.
//
// A refusal an operator cannot see is a routing bug in disguise: the process
// gets an accepted command back for the peers it MAY reach, the route never
// arrives at the peer it may not, and nothing on either side says which of the
// two happened (ai/rules/evidence.md -- fail closed OR say something; this
// does both).
func sendPermissionDenied(peer *Peer, origin sendOrigin) {
	var tb textbuf.Buffer
	process := origin.sender.String()
	// An attachOnly refusal is fixed by attaching the process, not by adding a
	// send type: the send list has no word for a raw message, so naming one here
	// would send the operator to a token the YANG validator refuses.
	action := tb.Str("add `attach process ").Str(process).Str(" { send [ ").Str(origin.sendType).Str(" ] }` to this peer").String()
	if origin.attachOnly {
		action = tb.Reset().Str("add `attach process ").Str(process).Str(" { }` to this peer").String()
	}
	routesLogger().Warn("send refused: this peer does not attach that process with the send permission it needs",
		"peer", peer.settings.Address,
		"process", process,
		"send-type", origin.sendType,
		"effect", "nothing was sent to this peer; other peers in the selector are unaffected",
		"action", action)
	recordSendRefused(process, origin.sendType)
}

// sendNoSenderDenied reports one send command refused for naming no sender.
//
// It is reported once per command rather than once per peer: no peer is served,
// so no peer is at fault. target names what the command addressed, in the words
// of the rail that built it -- a selector, a destination list, one peer -- and
// the caller pays for that string only here, on the refused path. The counter
// shares the label set of every other refusal, and Sender.String() gives it the
// bounded value "unset" (recordSendRefused).
func sendNoSenderDenied(target string, origin sendOrigin) {
	routesLogger().Warn("send refused: this command names no sender, so no attach block can permit it",
		"target", target,
		"process", origin.sender.String(),
		"send-type", origin.sendType,
		"effect", "nothing was sent to any peer this command names",
		"action", "the dispatch path that built this command must set CommandContext.Sender")
	recordSendRefused(origin.sender.String(), origin.sendType)
}

// selectorTarget names a selector for a refusal message.
func selectorTarget(sel *selector.Selector) string {
	var tb textbuf.Buffer
	return tb.Str("selector ").Str(sel.String()).String()
}

// peerTarget names one peer for a refusal message, for the rails that address a
// peer directly instead of resolving a selector.
func peerTarget(addr netip.Addr) string {
	var tb textbuf.Buffer
	return tb.Str("peer ").Str(addr.String()).String()
}

// destinationsTarget names a destination list for a refusal message. The count
// rather than the addresses: a route-server flush carries thousands, and the
// addresses that were refused are already one WARN each (sendPermissionDenied).
func destinationsTarget(n int) string {
	return textbuf.StrInt("destinations ", int64(n))
}

// sendPermissionMetrics holds the counter for sends the permission refused.
type sendPermissionMetrics struct {
	refused metrics.CounterVec // labels: process, type
}

// sendPermissionMetricsPtr stays nil until the reactor wires a registry. A
// build with metrics disabled leaves it nil and the refusal is then reported by
// its log line and its error alone. Same shape and same reason as
// announceMetricsPtr: the recorder is reached from a free function.
var sendPermissionMetricsPtr atomic.Pointer[sendPermissionMetrics]

// setSendPermissionMetricsRegistry creates the send-permission counter from the
// given registry. A nil registry is a no-op, leaving the recorder disabled.
func setSendPermissionMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	sendPermissionMetricsPtr.Store(&sendPermissionMetrics{
		refused: reg.CounterVec(
			"ze_bgp_send_refused_total",
			"Messages a process was refused permission to generate toward a peer, because the peer's config does not attach that process with that send type. Nothing reached the peer.",
			[]string{"process", "type"},
		),
	})
}

// recordSendRefused counts one refused peer.
//
// The peer is NOT a label. It is in the log line and in the error the issuing
// process receives, and both of those name one refusal; the counter answers a
// different question, which is whether a process is being refused at all. That
// is the same reasoning announce_metrics.go records for its own error path.
// The two labels are bounded: a process name comes from the config's plugin
// block, and the type is one of the built-in send types.
func recordSendRefused(process, sendType string) {
	if m := sendPermissionMetricsPtr.Load(); m != nil {
		m.refused.With(process, sendType).Inc()
	}
}
