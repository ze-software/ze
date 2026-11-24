// Design: docs/architecture/api/architecture.md -- the send half of a participant bus filter.
// Related: reactor_api.go -- getMatchingPeersSel, the one resolver every send command uses.
// Related: delivery_graph.go -- the receive half, indexed in the plugin server.

package reactor

import (
	"errors"
	"fmt"
	"net/netip"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
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
	// soft clear), bgpevents.SendRaw for a raw injection.
	sendType string
}

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
//
// Gated on `send [ raw ]` like every other rail is gated on the message it
// generates. It used to be gated on ATTACHMENT alone, because the send list had
// no word for a message of the caller's choosing; the owner added the word on
// 2026-08-30, so the peer now states the permission instead of implying it, and
// a raw-capable binding is countable by the initial-sync barrier that holds this
// peer's End-of-RIB (peer_run.go, Peer.initialUpdateReporters).
func rawOrigin(sender plugin.Sender) sendOrigin {
	return sendOrigin{sender: sender, sendType: bgpevents.SendRaw}
}

// errSendNotPermitted is returned when a selector matched peers and every one
// of them refused the process. The peers, the process and the message type are
// in the wrapped message, because the process that issued the command is the
// one reader who can fix it (ai/rules/cli.md).
var errSendNotPermitted = errors.New("send refused: no peer this selector names attaches this process with the required send permission")

// sendPermissionWhy is the reason every send-permission refusal gives: the
// grant lives in the compiled bus filter, so the operator knows where to look.
const sendPermissionWhy = "a peer takes a message from a process only when the compiled bus filter permits the endpoint to send that message type to the peer"

// sendNotPermitted builds the refusal a rail answers when every peer it
// addressed refused origin. target is what the caller addressed, in its own
// words ("selector 10.0.0.2", "3 destinations", "peer 10.0.0.2"), because only
// the caller holds it.
//
// errSendNotPermitted and its detail stay the cause, their text unchanged: a
// plugin receives the refusal as an RPC error string and recognizes it by that
// text (filter_irr sendNotPermittedText), and errors.Is still finds the
// sentinel in process. The operator error adds what the bare refusal never
// said: which process may not send what, that the compiled bus filter is what
// grants it, and the bus configuration that does.
func sendNotPermitted(target string, origin sendOrigin) error {
	cause := fmt.Errorf("%w: %s, process %s, send type %s",
		errSendNotPermitted, target, origin.sender, origin.sendType)
	process := origin.sender.String()
	var problem, fix textbuf.Buffer
	problem.Str("process ").Str(process).Str(" may not send ").Str(origin.sendType).
		Str(" messages to ").Str(target)
	fix.Str("permit bgp:").Str(origin.sendType).Str(" from /bus/endpoint/").Str(process).
		Str(" in the peer's bus receive filter or the endpoint's bus send filter")
	return opererr.Wrap(cause, problem.String(), sendPermissionWhy, fix.String())
}

// errSendNoSender is returned when a send command reaches the resolver and
// nothing said who issued it. There is no bus filter to consult for an
// unnamed issuer, and the operator's exemption belongs to the operator, so the
// command is refused. It is a defect in the dispatch path that produced the
// command, not an operator or process mistake, so the message names the field
// to set (ai/rules/evidence.md).
var errSendNoSender = errors.New("send refused: the command names no sender, so no bus filter can permit it; the dispatch path must set CommandContext.Sender")

// maySend reports whether the compiled delivery graph permits one process to
// generate one message type toward this peer.
//
// It FAILS CLOSED: an unnamed sender, a missing reactor or delivery graph, or
// an absent compiled send edge all return false.
func (p *Peer) maySend(origin sendOrigin) bool {
	process, ok := origin.sender.Process()
	if !ok {
		return false
	}
	return p.reactor.DeliveryGraph().MaySend(p.addrString, process, origin.sendType)
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

const (
	// reportSourceBGPReactor identifies peer send-permission refusals raised on
	// the operational report bus alongside plugin-server bus refusals under
	// diagnostic.CodeDoctorPluginRequestRefused.
	reportSourceBGPReactor = "bgp.reactor"

	// sendRefusalFallbackFix keeps the refusal event within report.FixMax when
	// a dynamic fix line containing long participant paths exceeds the bound.
	sendRefusalFallbackFix = "ze show errors"

	// sendNoSenderFix names the dispatch-path defect when a send command
	// carries the zero plugin.Sender.
	sendNoSenderFix = "the dispatch path that built this command must set CommandContext.Sender"
)

// busSendParticipant returns the canonical bus participant path for a named
// process sender, or the bounded sender name ("unset") when no process was
// named.
func busSendParticipant(sender plugin.Sender) string {
	if process, ok := sender.Process(); ok {
		return plugin.EndpointPath(process)
	}
	return sender.String()
}

// busSendMessage formats one BGP send type in the canonical bus message form
// ("bgp:update", "bgp:refresh", "bgp:raw").
func busSendMessage(sendType string) string {
	var tb textbuf.Buffer
	return tb.Str(bgpevents.Namespace).Byte(':').Str(sendType).String()
}

// fitSendRefusalSubject bounds subject to report.SubjectMax at a UTF-8 rune
// boundary so a long participant name never causes report.RaiseErrorFix to drop
// the refusal event.
func fitSendRefusalSubject(subject string) string {
	if len(subject) <= report.SubjectMax {
		return subject
	}
	const mark = "..."
	cut := report.SubjectMax - len(mark)
	for cut > 0 {
		if utf8.RuneStart(subject[cut]) {
			break
		}
		cut--
	}
	var tb textbuf.Buffer
	return tb.Str(subject[:cut]).Str(mark).String()
}

// fitSendRefusalFix returns fix when it fits report.FixMax, or
// sendRefusalFallbackFix otherwise so report.RaiseErrorFix never drops the
// refusal event.
func fitSendRefusalFix(fix string) string {
	if len(fix) <= report.FixMax {
		return fix
	}
	return sendRefusalFallbackFix
}

// sendPermissionDenied reports one refused peer, once per refusal.
//
// A refusal an operator cannot see is a routing bug in disguise: the process
// gets an accepted command back for the peers it MAY reach, the route never
// arrives at the peer it may not, and nothing on either side says which of the
// two happened (ai/rules/evidence.md -- fail closed OR say something; this
// logs at WARN, increments the single bus refusal counter, and raises
// diagnostic.CodeDoctorPluginRequestRefused on the report bus).
func sendPermissionDenied(peer *Peer, origin sendOrigin) {
	process := origin.sender.String()
	participant := busSendParticipant(origin.sender)
	msg := busSendMessage(origin.sendType)
	peerPath := deliveryPeerPathForSettings(peer.settings)
	if peerPath == "" {
		peerPath = peer.settings.Address.String()
	}

	var actionBuf textbuf.Buffer
	action := actionBuf.Str("permit ").Str(msg).Str(" from ").Str(participant).
		Str(" in ").Str(peerPath).Str("'s bus receive filter or ").Str(participant).
		Str("'s bus send filter").String()

	var reportMsg textbuf.Buffer
	reportMsg.Str("bus send refused: ").Str(participant).Str(" may not send ").Str(msg).
		Str(" to ").Str(peerPath).
		Str("; neither ").Str(peerPath).Str(" receive filter nor ").Str(participant).
		Str(" send filter permits this target")

	routesLogger().Warn("send refused: the compiled bus filter does not permit this process to send that message type to this peer",
		"peer", peer.settings.Address,
		"peer-participant", peerPath,
		"participant", participant,
		"process", process,
		"message", msg,
		"send-type", origin.sendType,
		"effect", "nothing was sent to this peer; other peers in the selector are unaffected",
		"action", action,
		"code", diagnostic.CodeDoctorPluginRequestRefused)
	pluginserver.RecordBusSendRefused(participant, msg)
	report.RaiseErrorFix(
		reportSourceBGPReactor,
		diagnostic.CodeDoctorPluginRequestRefused,
		fitSendRefusalSubject(participant),
		report.FitMessage(reportMsg.String()),
		fitSendRefusalFix(action),
	)
}

// sendNoSenderDenied reports one send command refused for naming no sender.
//
// It is reported once per command rather than once per peer: no peer is served,
// so no peer is at fault. target names what the command addressed, in the words
// of the rail that built it -- a selector, a destination list, one peer -- and
// the caller pays for that string only here, on the refused path. The counter
// shares the label set of every other bus refusal, and Sender.String() gives it
// the bounded participant value "unset" (pluginserver.RecordBusSendRefused).
func sendNoSenderDenied(target string, origin sendOrigin) {
	participant := busSendParticipant(origin.sender)
	msg := busSendMessage(origin.sendType)

	var reportMsg textbuf.Buffer
	reportMsg.Str("bus send refused: command addressing ").Str(target).
		Str(" for ").Str(msg).
		Str(" names no sender (").Str(participant).
		Str("), so no bus filter can permit it")

	routesLogger().Warn("send refused: this command names no sender, so no bus filter can permit it",
		"target", target,
		"participant", participant,
		"process", origin.sender.String(),
		"message", msg,
		"send-type", origin.sendType,
		"effect", "nothing was sent to any peer this command names",
		"action", sendNoSenderFix,
		"code", diagnostic.CodeDoctorPluginRequestRefused)
	pluginserver.RecordBusSendRefused(participant, msg)
	report.RaiseErrorFix(
		reportSourceBGPReactor,
		diagnostic.CodeDoctorPluginRequestRefused,
		fitSendRefusalSubject(participant),
		report.FitMessage(reportMsg.String()),
		sendNoSenderFix,
	)
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
