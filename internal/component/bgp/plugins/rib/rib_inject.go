// Design: docs/architecture/plugin/rib-storage-design.md — BMP wire route injection
// RFC: rfc/short/rfc7606.md — path attribute validation before storing
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_structured.go — handleReceivedStructured (BGP UPDATE parsing precedent)

package rib

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var (
	errShowProtocolRequiresProtocol               = errors.New("show-protocol requires <protocol>")
	errWithdrawProtocolRequiresProtocolPeerKey    = errors.New("withdraw-protocol requires <protocol> <peer-key>")
	errWithdrawRouterRequiresProtocolRouterPrefix = errors.New("withdraw-router requires <protocol> <router-prefix>")
	errInjectWireRouteBGPProtocol                 = errors.New("inject-wire-route rejected: protocol \"bgp\" must use the BGP UPDATE path, not protocol injection")
)

// handleInjectWireRoute stores BGP UPDATE routes under a named protocol's
// namespace in the two-level ribInPool. Used by BMP Route Monitoring to
// inject monitored routes without entering best-path selection.
//
// updateBody is the BGP UPDATE payload (RFC 4271 Section 4.3, without
// the 19-byte BGP header). Parsing uses the same WireUpdate path as
// handleReceivedStructured. No encoding context is available (BMP does
// not negotiate capabilities), so add-path is always false.
func (r *RIBManager) handleInjectWireRoute(protocol, peerKey string, updateBody []byte) error {
	if len(updateBody) < 4 {
		logger().Warn("inject-wire-route: UPDATE body too short", "protocol", protocol, "peer", peerKey, "len", len(updateBody))
		return nil
	}

	protoID, ok := redistevents.ProtocolIDOf(protocol)
	if !ok {
		logger().Warn("inject-wire-route: unknown protocol", "protocol", protocol)
		return nil
	}
	if protoID == bgpProtocolID {
		// Authenticated received BGP lives in the owner-keyed bgpPeers.
		// Its BGP protocol slot is reserved for explicit manual forwarding
		// input, never an alternate raw receive path.
		logger().Warn("inject-wire-route rejected: protocol \"bgp\" must use the BGP UPDATE path, not protocol injection", "peer", peerKey)
		return errInjectWireRouteBGPProtocol
	}

	wu := wireu.NewWireUpdate(updateBody, 0)

	var attrBytes []byte
	attrs, err := wu.Attrs()
	if err == nil && attrs != nil {
		attrBytes = attrs.Packed()
	}

	// RFC 7606: validate path attributes before storing.
	// BMP does not negotiate capabilities, so assume eBGP + ASN4.
	nlriData, err := wu.NLRI()
	hasNLRI := len(nlriData) > 0
	if len(attrBytes) > 0 || hasNLRI {
		result := message.ValidateUpdateRFC7606(attrBytes, hasNLRI, false, true)
		if result.Action >= message.RFC7606ActionTreatAsWithdraw {
			logger().Debug("inject-wire-route: RFC 7606 treat-as-withdraw",
				"protocol", protocol, "peer", peerKey, "description", result.Description)
			return nil
		}
	}

	r.peerMu.Lock()
	protoPeers := r.ribInPool[protoID]
	if protoPeers == nil {
		protoPeers = make(map[string]*storage.PeerRIB)
		r.ribInPool[protoID] = protoPeers
	}
	peerRIB := protoPeers[peerKey]
	if peerRIB == nil {
		peerRIB = storage.NewPeerRIB(peerKey)
		protoPeers[peerKey] = peerRIB
	}
	r.peerMu.Unlock()

	ipv4Family := family.Family{AFI: 1, SAFI: 1}

	// Withdrawals first, for the reason handleReceivedStructured spells out: RFC 4271
	// Section 4.3 says an UPDATE naming the same prefix in WITHDRAWN ROUTES and NLRI is
	// treated as though WITHDRAWN did not name it, so the announce has to land last
	// (RFC4271-4.3-5, RFC4271-4.3-7). This is the injection sibling of that path and had
	// the same ordering.
	//
	// The withdrawal blocks carry their OWN error variables. `err` here still holds the
	// result of the wu.NLRI() call above, which the announce block below tests; reusing
	// it would silently make that block read the withdrawal's error instead.
	wdData, wdErr := wu.Withdrawn()
	if wdErr == nil && len(wdData) > 0 && nlrisplit.Supported(ipv4Family) {
		withdrawns, _ := nlrisplit.Split(ipv4Family, wdData, false)
		for _, wd := range withdrawns {
			peerRIB.Remove(ipv4Family, wd)
		}
	}

	mpUnreach, unreachErr := wu.MPUnreach()
	if unreachErr == nil && mpUnreach != nil {
		fam := mpUnreach.Family()
		if nlrisplit.Supported(fam) {
			wdBytes := mpUnreach.WithdrawnBytes()
			if len(wdBytes) > 0 {
				withdrawns, _ := nlrisplit.SplitWithdrawn(fam, wdBytes, false)
				for _, wd := range withdrawns {
					peerRIB.Withdraw(fam, wd)
				}
			}
		}
	}

	// One route clock reading for every route this injected UPDATE announces,
	// as handleReceivedStructured takes one per received UPDATE.
	changedAt := r.routeClock.now()

	// nlriData already fetched above for validation.
	if err == nil && len(nlriData) > 0 && nlrisplit.Supported(ipv4Family) {
		prefixes, _ := nlrisplit.Split(ipv4Family, nlriData, false)
		for _, wirePrefix := range prefixes {
			peerRIB.InsertAt(ipv4Family, attrBytes, wirePrefix, changedAt)
		}
	}

	mpReach, reachErr := wu.MPReach()
	if reachErr == nil && mpReach != nil {
		fam := mpReach.Family()
		if nlrisplit.Supported(fam) {
			nlriBytes := mpReach.NLRIBytes()
			if len(nlriBytes) > 0 {
				prefixes, _ := nlrisplit.Split(fam, nlriBytes, false)
				for _, wirePrefix := range prefixes {
					peerRIB.InsertAt(fam, attrBytes, wirePrefix, changedAt)
				}
			}
		}
	}

	return nil
}

// registerInjectCommands registers commands for protocol-scoped route management.
// Called from doRegisterBuiltinCommands.
func registerInjectCommands() {
	cmds := []struct {
		name    string
		help    string
		handler CommandHandler
	}{
		{"request bgp rib withdraw-protocol", "Withdraw all routes for a peer under a protocol",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				if len(args) < 2 {
					return statusError, "", errWithdrawProtocolRequiresProtocolPeerKey
				}
				r.withdrawAllForPeer(args[0], args[1])
				return statusDone, map[string]any{jsonKeyWithdrawn: true}, nil
			}},
		{"request bgp rib withdraw-router", "Withdraw all routes for a router under a protocol",
			func(r *RIBManager, _ string, args []string) (string, any, error) {
				if len(args) < 2 {
					return statusError, "", errWithdrawRouterRequiresProtocolRouterPrefix
				}
				r.withdrawAllForRouter(args[0], args[1])
				return statusDone, map[string]any{jsonKeyWithdrawn: true}, nil
			}},
	}
	for _, c := range cmds {
		if err := registerCommand(c.name, c.help, c.handler); err != nil {
			logger().Warn("inject command registration failed", "command", c.name, "error", err)
		}
	}
	// The grammar is the model's, not this sentence's: ze-rib-cmd.yang
	// declares the protocol leaf, which is the one argument, on both routes
	// that reach here, the forwarder in cmd/rib and the plugin's own dispatch.
	// The selection (`peer <selector>` and the filters `show bgp rib` takes)
	// arrives as the operations the engine declared for the line.
	if err := registerPipelineCommand(cmdShowRIBProtocol, "Show the routes one protocol feeds into the RIB",
		func(r *RIBManager, _ string, args []string, pipe []sdk.PipeOperation) (string, any, error) {
			if len(args) < 1 {
				return statusError, "", errShowProtocolRequiresProtocol
			}
			if refusal, refused := refuseRouteReadArguments(cmdShowRIBProtocol, args[1:]); refused {
				return commandAnswer(refusal)
			}
			return r.showProtocolPipeline(args[0], pipe)
		}); err != nil {
		panic("BUG: rib command " + cmdShowRIBProtocol + " registered twice: " + err.Error())
	}
}

// showProtocolPipeline runs the show pipeline filtered to a single protocol's peers.
// Protocol "bgp" reads exact received owners and explicit synthetic forwarding
// inputs; every other protocol reads its string-keyed ribInPool namespace.
//
// The peer selector arrives as the `peer <selector>` operation the engine
// declared, beside the filters, so nothing here guesses whether a bare token
// is a selector or a keyword (ai/rules/cli.md, keyword before value). A scope
// operation is refused: a protocol RIB holds received routes only, so a scope
// could only be ignored, and an ignored selection answers rows the operator
// did not ask for.
//
// A protocol the registry does not hold is an error rather than an answer,
// and the error names the registered protocols, because an operator who typed
// a name Ze does not know needs the list rather than a verdict.
func (r *RIBManager) showProtocolPipeline(protocol string, operations []sdk.PipeOperation) (string, any, error) {
	protoID, ok := redistevents.ProtocolIDOf(protocol)
	if !ok {
		return statusError, "", fmt.Errorf("show bgp rib protocol: %q is not a registered protocol (registered: %s)",
			protocol, strings.Join(redistevents.ProtocolNames(), ", "))
	}

	// The operations are read before the RIB is, so a refused line is refused
	// on an empty RIB too. A protocol RIB holds received routes only, so a
	// covering or exact walk applies to it as it stands.
	for _, operation := range operations {
		if _, isScope := scopeKeywords[operation.Name]; isScope {
			return statusError, "", refuseProtocolScope(protocol, operation.Name)
		}
	}
	_, selector, walk, stages, errMsg := parsePipelineOperations(operations)
	if errMsg != "" {
		return statusError, "", errors.New(errMsg)
	}

	// peerMu is taken for the map READS and given straight back. Holding it
	// across the drain would nest a reader inside a reader: the sources take it
	// again for themselves when they materialize a peer inside Next, and Go's
	// RWMutex makes a later RLock wait behind a waiting writer, so the two
	// together deadlock the moment an UPDATE arrives between them.
	r.peerMu.RLock()
	empty := len(r.ribInPool[protoID]) == 0
	if protoID == bgpProtocolID {
		empty = empty && len(r.bgpPeers) == 0
	}
	r.peerMu.RUnlock()

	// The empty answer carries the SAME shape as a populated one: flat rows
	// under `routes` (owner ruling, 2026-08-23). It answered
	// `{"adj-rib-in":{}}` here while the populated path answered flat rows, so
	// a caller parsing the empty case saw a shape the command no longer uses.
	if empty {
		return statusDone, json.RawMessage(`{"routes":[]}`), nil
	}

	// Construction reads the peer-keyed maps, so it takes the lock again, for
	// itself and no longer.
	r.peerMu.RLock()
	var source pipelineIterator
	var release func()
	if protoID == bgpProtocolID {
		src := newInboundSource(r, selector, walk)
		source, release = src, src.release
	} else {
		src := newProtocolInboundSource(r, protoID, selector, walk)
		source, release = src, src.release
	}
	r.peerMu.RUnlock()
	defer release()

	current := source
	for _, stage := range stages {
		current = stage.apply(current)
	}

	if !hasTerminal(stages) {
		jt := newRoutesDocument(current)
		return statusDone, json.RawMessage(jt.Meta().JSON), nil
	}

	meta := current.Meta()
	if meta.Refusal != nil {
		return statusError, "", meta.Refusal
	}
	if meta.JSON != "" {
		return statusDone, json.RawMessage(meta.JSON), nil
	}
	return statusDone, map[string]any{jsonKeyCount: meta.Count}, nil
}

// refuseProtocolScope refuses a scope operation on `show bgp rib protocol`. A
// protocol RIB holds received routes only, so the scope would be ignored.
func refuseProtocolScope(protocol, scope string) error {
	var problem, fix textbuf.Buffer
	problem.Str(cmdShowRIBProtocol).Str(" takes no scope, and was given ").Str(scope)
	fix.Str(cmdShowRIBProtocol).Byte(' ').Str(protocol).Str(" | count")
	return opererr.New(problem.String(), "A protocol RIB holds received routes only, so a scope selects nothing.", fix.String())
}

// withdrawAllForPeer removes all routes for a peer under a given protocol.
// Used by BMP Peer Down to clean up monitored routes.
func (r *RIBManager) withdrawAllForPeer(protocol, peerKey string) {
	protoID, ok := redistevents.ProtocolIDOf(protocol)
	if !ok {
		return
	}

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	protoPeers := r.ribInPool[protoID]
	if protoPeers == nil {
		return
	}
	if peerRIB := protoPeers[peerKey]; peerRIB != nil {
		peerRIB.Release()
		delete(protoPeers, peerKey)
	}
}

// withdrawAllForRouter removes all routes for all peers of a given router
// under a protocol. The router prefix is matched against composite keys
// that start with "<router>:".
func (r *RIBManager) withdrawAllForRouter(protocol, routerPrefix string) {
	protoID, ok := redistevents.ProtocolIDOf(protocol)
	if !ok {
		return
	}

	r.peerMu.Lock()
	defer r.peerMu.Unlock()

	protoPeers := r.ribInPool[protoID]
	if protoPeers == nil {
		return
	}

	prefix := routerPrefix + ":"
	for key, peerRIB := range protoPeers {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			peerRIB.Release()
			delete(protoPeers, key)
		}
	}
}
