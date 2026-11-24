// Design: docs/architecture/api/architecture.md -- complete compiled-bus dissemination validation.
// Related: dissemination.go -- the same immutable graph resolves effective owners.
package server

import (
	"slices"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ValidateDisseminationBindings refuses partial transit configurations against
// the compiled bus filters. Source and destination grants MUST belong to one
// configured endpoint. The graph and declarations MUST remain immutable during
// this call.
func (g *DeliveryGraph) ValidateDisseminationBindings(processes []DisseminationProcess) error {
	for _, process := range processes {
		if err := registry.CheckDisseminationDeclarations(process.Name, process.Declarations); err != nil {
			return err
		}
		if err := registry.CheckDisseminationEvents(process.Name, process.Declarations); err != nil {
			return err
		}
		for _, declaration := range process.Declarations {
			if declaration.Namespace != g.namespace {
				continue
			}
			if err := g.validateDisseminationProcess(process.Name, process.ClaimSessionScope, &declaration); err != nil {
				return err
			}
			for _, peer := range g.order {
				if _, err := g.ResolveDisseminationOwner(peer, declaration.Role, processes); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (g *DeliveryGraph) validateDisseminationProcess(process string, scope []rpc.FilterSessionFact, declaration *rpc.DisseminationDecl) error {
	for _, peer := range g.order {
		if !g.peerMatchesSessionFacts(peer, scope) {
			continue
		}
		if !g.peerHasProcess(peer, process) {
			continue
		}
		name := peer
		if g.peers[peer].name != "" {
			name = g.peers[peer].name
		}
		destination := false
		for _, required := range declaration.DestinationSend {
			if g.MaySend(peer, process, required) {
				destination = true
				break
			}
		}
		if destination {
			fix := disseminationBusFixLine(process, declaration.Namespace, declaration.DestinationReceive, declaration.DestinationSend)
			if !g.disseminationDestination(peer, process, declaration) {
				if g.peerOptedOutOfProcess(peer, process) {
					continue
				}
				var problem textbuf.Buffer
				problem.Str("participant ").Quoted(name).Str(" endpoint ").Quoted(process).
					Str(" has incomplete destination grants for dissemination role ").Quoted(declaration.Role)
				return opererr.New(
					problem.String(),
					"the compiled bus filter narrows destination messages required by this dissemination role",
					fix)
			}
			if !g.disseminationSource(process, declaration) {
				sourceFix := disseminationBusFixLine(process, declaration.Namespace, declaration.SourceReceive, nil)
				var problem textbuf.Buffer
				problem.Str("participant ").Quoted(name).Str(" destination endpoint ").Quoted(process).
					Str(" has no complete source on this same instance for dissemination role ").Quoted(declaration.Role)
				return opererr.New(
					problem.String(),
					"dissemination requires at least one source feeding all declared source messages to the same endpoint",
					sourceFix)
			}
			continue
		}
		if !g.disseminationReceive(peer, process, declaration.SourceReceive) {
			if g.peerOptedOutOfProcess(peer, process) {
				continue
			}
			fix := disseminationBusFixLine(process, declaration.Namespace, declaration.SourceReceive, nil)
			var problem textbuf.Buffer
			problem.Str("participant ").Quoted(name).Str(" endpoint ").Quoted(process).
				Str(" has incomplete source grants for dissemination role ").Quoted(declaration.Role)
			return opererr.New(
				problem.String(),
				"the compiled bus filter narrows source messages required by this dissemination role",
				fix)
		}
	}
	return nil
}

// peerOptedOutOfProcess reports whether peer intentionally sends zero events to
// process via peer's own send > to restriction while process itself kept its
// full declared receive set. Such a peer simply does not use process for
// dissemination (S2, S3, S26) unless an active feature requirement demands it.
func (g *DeliveryGraph) peerOptedOutOfProcess(peerAddr, process string) bool {
	if len(g.receiveTokens(peerAddr, process)) > 0 {
		return false
	}
	if g.compiled == nil {
		return false
	}
	pd := g.peers[peerAddr]
	if pd == nil {
		return false
	}
	peerFilter := g.compiled.Filter(pd.path)
	if peerFilter == nil {
		return false
	}
	if !peerFilter.SendAllowNone && !peerFilter.SendToNone && len(peerFilter.SendToPaths) == 0 {
		return false
	}
	procFilter := g.compiled.Filter(plugin.EndpointPath(process))
	if procFilter == nil {
		for _, cand := range g.compiled.filters {
			if cand != nil && cand.EndpointName == process {
				procFilter = cand
				break
			}
		}
	}
	if procFilter == nil {
		return false
	}
	return procFilter.ReceiveAllowAllDeclared && !procFilter.ReceiveFromNone && procFilter.ReceiveFromOrigin.Origin != BusOriginWritten
}

func disseminationBusFixLine(process, namespace string, receive, send []string) string {
	var recvMsgs []string
	for _, tok := range receive {
		name, _, ok := events.SplitTypeToken(namespace, tok)
		if !ok {
			name = tok
		}
		var tb textbuf.Buffer
		msg := tb.Str(namespace).Byte(':').Str(name).String()
		if !slices.Contains(recvMsgs, msg) {
			recvMsgs = append(recvMsgs, msg)
		}
	}
	var sendMsgs []string
	for _, tok := range send {
		var tb textbuf.Buffer
		msg := tb.Str(namespace).Byte(':').Str(tok).String()
		if !slices.Contains(sendMsgs, msg) {
			sendMsgs = append(sendMsgs, msg)
		}
	}

	var tb textbuf.Buffer
	tb.Str("bus { endpoint ").Str(process).Str(" { ")
	if len(recvMsgs) > 0 {
		tb.Str("receive { allow [ ").Join(recvMsgs, " ").Str(" ]; } ")
	}
	if len(sendMsgs) > 0 {
		tb.Str("send { allow [ ").Join(sendMsgs, " ").Str(" ]; } ")
	}
	tb.Str("} }")
	return tb.String()
}

// NewDisseminationGraph builds an immutable DeliveryGraph compiled from peers,
// busCfg, and extraEndpoints for offline config validation as well as runtime
// delivery.
func NewDisseminationGraph(
	namespace string,
	peers []DeliveryPeer,
	busCfg plugin.BusConfig,
	extraEndpoints []BusParticipantInput,
) (*DeliveryGraph, error) {
	return BuildDeliveryGraph(namespace, peers, busCfg, extraEndpoints)
}
