// Design: docs/architecture/api/architecture.md -- the two halves of one edge
// Related: delivery_graph.go -- the config's half, indexed per peer and type
// Related: subscribe.go -- the plugin's half, declared at ready

package server

import (
	"slices"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
)

// deliveryDisagreement is one peer, one process and one event type the two
// halves of an edge do not agree about. Nothing is delivered for it, and
// neither half can tell the operator why (R-9).
type deliveryDisagreement struct {
	peer    string
	process string
	// reason is the operator-facing sentence. The three below are the whole
	// set: a grant nobody declared, a declaration nobody granted, and two
	// directions that never meet.
	reason string
	// granted and declared are the tokens as an operator writes them. Either
	// is empty when that half names nothing.
	granted  string
	declared string
}

const (
	reasonUndeclared = "event delivery: the config grants an event the plugin never declared"
	reasonUngranted  = "event delivery: the plugin declared an event the peer does not grant it"
	reasonDirection  = "event delivery: the config and the plugin disagree about direction"
	// reasonUnattached is the whole-process case of reasonUngranted, and it is
	// reported separately because it names no peer: NO peer attaches the
	// process, so no peer-scoped event of any type reaches it rather than it
	// being fed less than it asked for. It carries the largest consequence of
	// the four and was the only one with no voice: the three above are found by
	// walking the index's edges, and a process nobody attaches has no edge to
	// walk.
	//
	// PEER-SCOPED is the whole claim, and it is deliberately not "fed nothing":
	// an event emitted with no peer address is not peer-scoped, no attach block
	// can describe it, and it keeps flowing to every subscriber (dispatch.go,
	// the `peerAddress == ""` branch). A process that declared only such an
	// event is still served, and this line must not tell its operator otherwise.
	reasonUnattached = "event delivery: the plugin declared events and no peer attaches it, so no peer-scoped event reaches it"
)

// reconcileDelivery reports every disagreement between what a peer's config
// grants a process and what that process declared it can handle.
//
// Delivery is the OVERLAP of the two halves, so a disagreement is silent by
// construction: the event is simply not delivered, and neither half can tell
// the operator the other one disagrees. This is the voice for that (R-9).
//
// Config is parsed before an external program can declare anything, so the two
// halves become known at different moments and this runs at both of them: when
// a process declares (registerSubscriptions, with only set to it) and when the
// config publishes a new index (UpdateDeliveryGraph, with only nil).
//
// One line per disagreeing peer, process and event type, plus one line for each
// process no peer attaches at all, and silence when the two halves agree (R-12).
func (s *Server) reconcileDelivery(only *process.Process) {
	for _, d := range s.deliveryDisagreements(only) {
		// No peer to name: the finding is that the process is in no attach
		// block at all, so a `peer=` key would be an empty answer to a question
		// this row does not ask.
		if d.peer == "" {
			logger().Warn(d.reason,
				"process", d.process,
				"effect", "no peer-scoped event of any type reaches this process",
				"action", "name it in an `attach process` block on each peer it serves")
			continue
		}
		logger().Warn(d.reason,
			"peer", d.peer, "process", d.process,
			"granted", d.granted, "declared", d.declared)
	}
}

// deliveryDisagreements is reconcileDelivery's whole judgement, as data. It is
// separate so a test asserts the finding rather than the log line, and so a
// later operator surface can print the same set.
//
// A process that has declared nothing is skipped rather than reported: its half
// is not known yet, and a report about it would be a guess.
func (s *Server) deliveryDisagreements(only *process.Process) []deliveryDisagreement {
	if s.subscriptions == nil {
		return nil
	}
	g := s.DeliveryGraph()
	if g.Len() == 0 {
		return nil
	}
	var out []deliveryDisagreement
	attached := make(map[string]struct{}, g.Len())
	for _, peer := range g.Inspect() {
		for _, edges := range peer.Processes {
			attached[edges.Process] = struct{}{}
			proc := s.subscriptions.processNamed(edges.Process)
			if proc == nil || (only != nil && proc != only) {
				continue
			}
			out = append(out, s.edgeDisagreements(g, peer, edges, proc)...)
		}
	}
	// Reported after the edges, because it is a statement about the whole index
	// rather than about one peer: the process is absent from every attach block
	// the config holds.
	for _, name := range s.subscriptions.declaredUnattached(g.ns, attached, only) {
		out = append(out, deliveryDisagreement{process: name, reason: reasonUnattached})
	}
	return out
}

// edgeDisagreements judges one peer-to-process edge, in both directions: a type
// the config grants that the plugin never declared, and a type the plugin
// declared that this peer's attach block does not grant.
func (s *Server) edgeDisagreements(g *DeliveryGraph, peer PeerEdges, edges ProcessEdges, proc *process.Process) []deliveryDisagreement {
	var out []deliveryDisagreement
	granted := make([]events.EventTypeID, 0, len(edges.Receive))

	for _, token := range edges.Receive {
		eventType, dir, ok := events.SplitTypeToken(g.namespace, token)
		if !ok {
			continue // the index already names an unresolvable token, in Unresolved
		}
		etID := events.LookupEventTypeID(eventType)
		granted = append(granted, etID)

		declared, declaredDir := s.subscriptions.declaredDirection(proc, g.ns, etID, peer.Peer, peer.Name)
		if !declared {
			out = append(out, deliveryDisagreement{
				peer: peer.Peer, process: edges.Process,
				reason: reasonUndeclared, granted: token,
			})
			continue
		}
		// Both halves name the type and neither direction reaches the other, so
		// nothing is delivered for it. Directions that overlap at all deliver
		// something, and say nothing.
		if !directionsOverlap(dir, declaredDir) {
			out = append(out, deliveryDisagreement{
				peer: peer.Peer, process: edges.Process, reason: reasonDirection,
				granted: token, declared: events.DirectionToken(eventType, declaredDir),
			})
		}
	}

	for _, etID := range s.subscriptions.declaredTypes(proc, g.ns, peer.Peer, peer.Name) {
		if slices.Contains(granted, etID) {
			continue
		}
		out = append(out, deliveryDisagreement{
			peer: peer.Peer, process: edges.Process,
			reason: reasonUngranted, declared: etID.String(),
		})
	}
	return out
}

// directionsOverlap reports whether two directions have any direction in
// common. Both and unspecified are every direction: an event carrying no
// direction of its own matches any grant of its type (Subscription.Matches).
func directionsOverlap(a, b events.Direction) bool {
	if a == events.DirBoth || a == events.DirUnspecified {
		return true
	}
	if b == events.DirBoth || b == events.DirUnspecified {
		return true
	}
	return a == b
}
