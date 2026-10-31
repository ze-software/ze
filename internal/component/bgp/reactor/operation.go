// Design: docs/architecture/config/apply-ordering.md -- BGP peer operation handlers
// Related: reactor_peers.go -- peer add/remove primitives

package reactor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errPeerNotInReloadConfig = errors.New("peer not found in reload config")

// VerifyConfigOperation validates one BGP config operation of transaction txID
// without mutating state. The transaction's operations share one resolution
// of the candidate config (operationCandidateSlot) with its apply.
func (r *Reactor) VerifyConfigOperation(txID string, op *rpc.ConfigOperation) error {
	return (&reactorAPIAdapter{r: r}).verifyConfigOperation(txID, op)
}

// ApplyConfigOperation applies one BGP config operation and records its
// inverse. The journal names the operation's transaction when it carries one
// (sourcePolicyTransaction), and the transaction's operations then share one
// resolution of the candidate config with its verify.
func (r *Reactor) ApplyConfigOperation(op *rpc.ConfigOperation, j registry.ConfigJournal) (*rpc.ConfigOperationApplyOutput, error) {
	return (&reactorAPIAdapter{r: r}).applyConfigOperation(op, j)
}

func (a *reactorAPIAdapter) verifyConfigOperation(txID string, op *rpc.ConfigOperation) error {
	if op == nil {
		return errors.New("bgp operation verify requires an operation")
	}
	switch op.Type {
	case configop.AddPeer:
		_, _, err := a.resolveOperationCandidate(txID, op)
		return err
	case configop.RemovePeer:
		_, err := a.runningPeerSettings(op)
		return err
	case configop.ModifyPeer:
		if _, err := a.runningPeerSettings(op); err != nil {
			return err
		}
		settings, policyResolved, err := a.resolveOperationCandidate(txID, op)
		if err != nil {
			return err
		}
		// RFC 8212 Section 3: D-1 refuses an import relaxation the session
		// cannot recover from, before any plugin applies the transaction.
		return a.verifyImportRelaxation(settings, policyResolved)
	case configop.ReconcilePeers:
		tree, err := operationBGPTree(op)
		if err != nil {
			return err
		}
		loaded, err := a.loadPeersFullOrTree(tree)
		if err != nil {
			return err
		}
		// RFC 8212 Section 3
		return a.verifyImportRelaxations(loaded)
	default:
		return fmt.Errorf("bgp operation %s not supported", op.Type)
	}
}

func (a *reactorAPIAdapter) applyConfigOperation(op *rpc.ConfigOperation, j configJournal) (*rpc.ConfigOperationApplyOutput, error) {
	if op == nil {
		return nil, errors.New("bgp operation apply requires an operation")
	}
	if j == nil {
		return nil, errors.New("bgp operation apply requires a journal")
	}
	// The journal names the operation's transaction when it carries one, and
	// the transaction's operations then share one candidate resolution with
	// its verify (operationCandidateSlot).
	var txID string
	if tx, ok := j.(sourcePolicyTransaction); ok {
		txID, _ = tx.SourcePolicyTransaction()
	}
	// A config operation is a config APPLY, so it discards the operator's live
	// `subscribe` overrides, as the section apply does at the end of
	// reconcilePeersJournaled (R-10). It runs BEFORE the peer changes: a session
	// this operation restarts reaches Established inside the apply, and its
	// first events MUST already be delivered under the document's grant.
	if a.r.api != nil {
		a.r.api.DiscardRuntimeSubscriptions()
	}
	switch op.Type {
	case configop.AddPeer:
		settings, _, err := a.resolveOperationCandidate(txID, op)
		if err != nil {
			return nil, err
		}
		if err := j.Record(
			func() error { return a.r.AddPeer(settings) },
			func() error { return a.r.RemovePeer(settings.Address) },
		); err != nil {
			return nil, err
		}
		a.emitOperationListenerReady(settings)
		return bgpOperationApplyOutput(settings), nil
	case configop.RemovePeer:
		settings, err := a.runningPeerSettings(op)
		if err != nil {
			return nil, err
		}
		remove := a.removePeerForOperation
		if len(op.Params.Config) > 0 {
			// The remove half of a remove and add pair (bgpPeerOperation):
			// the peer stays configured, under the config it carries.
			remove = a.restartPeerForOperation
		}
		if err := j.Record(
			func() error { return remove(settings) },
			func() error { return a.r.AddPeer(settings) },
		); err != nil {
			return nil, err
		}
		return &rpc.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
	case configop.ModifyPeer:
		oldSettings, err := a.runningPeerSettings(op)
		if err != nil {
			return nil, err
		}
		newSettings, policyResolved, err := a.resolveOperationCandidate(txID, op)
		if err != nil {
			return nil, err
		}
		swapped, err := a.swapPeerForOperation(newSettings, policyResolved, j)
		if err != nil {
			return nil, err
		}
		if swapped {
			a.emitOperationListenerReady(newSettings)
			return bgpOperationApplyOutput(newSettings), nil
		}
		if err := j.Record(
			func() error { return a.restartPeerForOperation(oldSettings) },
			func() error { return a.r.AddPeer(oldSettings) },
		); err != nil {
			return nil, err
		}
		if err := j.Record(
			func() error { return a.r.AddPeer(newSettings) },
			func() error { return a.r.RemovePeer(newSettings.Address) },
		); err != nil {
			return nil, err
		}
		a.emitOperationListenerReady(newSettings)
		return bgpOperationApplyOutput(newSettings), nil
	case configop.ReconcilePeers:
		// The rest of the diff the peer operations before it did not carry
		// (bgpResidualOperation): the section apply's reconcile, under this
		// operation's journal, so its undo and its transaction are this one's.
		tree, err := operationBGPTree(op)
		if err != nil {
			return nil, err
		}
		loaded, err := a.loadPeersFullOrTree(tree)
		if err != nil {
			return nil, fmt.Errorf("bgp operation %s load peers: %w", op.Type, err)
		}
		if err := a.reconcilePeersJournaled(loaded.peers, loaded.globals, loaded.groups, "apply config operation", j); err != nil {
			return nil, err
		}
		return &rpc.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
	default:
		return nil, fmt.Errorf("bgp operation %s not supported", op.Type)
	}
}

// operationBGPTree reads the candidate bgp root a reconcile-peers operation
// carries.
func operationBGPTree(op *rpc.ConfigOperation) (map[string]any, error) {
	if len(op.Params.Config) == 0 {
		return nil, fmt.Errorf("bgp operation %s carries no candidate bgp root", op.Type)
	}
	var tree map[string]any
	if err := json.Unmarshal(op.Params.Config, &tree); err != nil {
		return nil, fmt.Errorf("bgp operation %s candidate bgp root: %w", op.Type, err)
	}
	return tree, nil
}

func (a *reactorAPIAdapter) emitOperationListenerReady(settings *PeerSettings) {
	if settings == nil || !settings.LocalAddress.IsValid() || a.r.eventBus == nil {
		return
	}
	readyPayload, err := json.Marshal(bgpListenerReadyPayload{Address: settings.LocalAddress.String()})
	if err != nil {
		return
	}
	if _, emitErr := a.r.eventBus.Emit(bgpevents.Namespace, bgpevents.EventListenerReady, string(readyPayload)); emitErr != nil {
		reactorLogger().Debug("bgp operation: emit listener-ready", "address", settings.LocalAddress, "error", emitErr)
	}
}

// runningPeerSettings answers the settings the reactor is RUNNING for the peer
// this operation names. That state is what a remove-peer destroys, so it is
// also what the inverse recorded beside it restores.
//
// The reactor's own peer is the only complete source for it. The operation
// carries the peer's active config subtree in Params.OldConfig, but that
// subtree is the config FILE's, and this package's parser reads config leaves
// alone: the routes a peer originates, its filter chains, its loop-detection
// policy and its redistribution bindings are added afterwards by the full
// loader (peersAndDynamicGroups, ../config/peers.go), which lives in a package
// this one cannot import. Rebuilding the peer from the subtree here restored a
// session that announced nothing, and every field the loader adds was lost the
// same way (plan/journal/announced-state-never-replayed.md, 2026-09-08).
//
// A peer the reactor is not running has no state to destroy or to restore, and
// RemovePeer refuses an address it does not hold, so the operation is refused
// here rather than allowed to fail half way through the apply.
func (a *reactorAPIAdapter) runningPeerSettings(op *rpc.ConfigOperation) (*PeerSettings, error) {
	peerName := firstOperationString(op.Params.Peer, op.Target.Peer, op.Params.Name, op.Target.Name)
	if peerName == "" {
		return nil, fmt.Errorf("bgp operation %s requires peer name", op.Type)
	}

	a.r.mu.RLock()
	var found *Peer
	for _, peer := range a.r.peers {
		settings := peer.Settings()
		if settings.Name != peerName {
			continue
		}
		// A dynamic member is named after the address it connected from
		// (buildDynamicPeerSettings, reactor_dynamic.go), never after a config
		// entry, so it is never what a config operation names -- not even when
		// an operator gives a configured peer that same name.
		if settings.IsDynamic {
			continue
		}
		found = peer
		break
	}
	a.r.mu.RUnlock()

	if found == nil {
		return nil, fmt.Errorf("bgp operation %s peer %q is not running", op.Type, peerName)
	}
	return found.settingsSnapshot(), nil
}

// candidatePeerSettingsFromOperationConfig resolves op's candidate peer outside
// any transaction, so nothing is shared: resolveOperationCandidate with no
// transaction ID, without the import-policy report.
func (a *reactorAPIAdapter) candidatePeerSettingsFromOperationConfig(op *rpc.ConfigOperation) (*PeerSettings, error) {
	settings, _, err := a.resolveOperationCandidate("", op)
	return settings, err
}

// resolveOperationCandidate answers op's candidate peer, resolved once per
// transaction txID (operationCandidateSlot; empty resolves alone), and
// reports whether the settings carry the peer's effective import policy:
// false when they were built from the operation's config leaves alone, which
// carry no filter chain (peerSettingsFromOperationConfig).
func (a *reactorAPIAdapter) resolveOperationCandidate(txID string, op *rpc.ConfigOperation) (*PeerSettings, bool, error) {
	settings, err := a.reloadCandidatePeer(txID, op)
	if err == nil {
		return settings, true, nil
	}
	if !errors.Is(err, errPeerNotInReloadConfig) {
		return nil, false, err
	}
	settings, err = a.peerSettingsFromOperationConfig(op)
	return settings, false, err
}

// peerSettingsFromReloadConfig answers op's peer from the loaded candidate
// outside any transaction: reloadCandidatePeer with no transaction ID.
func (a *reactorAPIAdapter) peerSettingsFromReloadConfig(op *rpc.ConfigOperation) (*PeerSettings, error) {
	return a.reloadCandidatePeer("", op)
}

// reloadCandidatePeer answers a copy of op's peer from the candidate the
// reload function loads, loaded once per transaction txID
// (operationCandidateSlot), and errPeerNotInReloadConfig when the reactor has
// no reload function or the candidate does not name the peer.
func (a *reactorAPIAdapter) reloadCandidatePeer(txID string, op *rpc.ConfigOperation) (*PeerSettings, error) {
	if a == nil || a.r == nil || op == nil {
		return nil, errPeerNotInReloadConfig
	}
	a.r.mu.RLock()
	reloadFn := a.r.reloadFunc
	configPath := a.r.config.ConfigPath
	a.r.mu.RUnlock()
	if reloadFn == nil || configPath == "" {
		return nil, errPeerNotInReloadConfig
	}
	peerName := firstOperationString(op.Params.Peer, op.Target.Peer, op.Params.Name, op.Target.Name)
	if peerName == "" {
		return nil, fmt.Errorf("bgp operation %s requires peer name", op.Type)
	}
	peers, err := a.r.operationCandidate.peers(txID, configPath, reloadFn)
	if err != nil {
		return nil, fmt.Errorf("bgp operation %s load candidate peer config: %w", op.Type, err)
	}
	return operationCandidatePeer(peers, peerName, txID != "")
}

// peerSettingsFromOperationConfig builds a peer from the candidate config the
// operation embeds. It is the second half of candidatePeerSettingsFromOperationConfig
// and has one caller, because it reads the config LEAVES alone: a peer this
// parser builds carries no route, no filter chain and no redistribution
// binding, which the full loader adds and the reload route above therefore
// answers with. It stands for the peer that the loaded candidate does not name.
func (a *reactorAPIAdapter) peerSettingsFromOperationConfig(op *rpc.ConfigOperation) (*PeerSettings, error) {
	raw := op.Params.Config
	if len(raw) == 0 {
		return nil, fmt.Errorf("bgp operation %s requires peer config", op.Type)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("bgp operation %s unmarshal peer config: %w", op.Type, err)
	}

	peerName := firstOperationString(op.Params.Peer, op.Target.Peer, op.Params.Name, op.Target.Name)
	if peerName == "" {
		return nil, fmt.Errorf("bgp operation %s requires peer name", op.Type)
	}

	bgpRoot := root
	if wrapped, ok := root["bgp"].(map[string]any); ok {
		bgpRoot = wrapped
	}
	localAS := operationRootLocalAS(bgpRoot)
	routerID := operationRootRouterID(bgpRoot)

	peerTree := root
	if peerSection, ok := bgpRoot[configListNamePeer].(map[string]any); ok {
		rawPeer, exists := peerSection[peerName]
		if !exists {
			return nil, fmt.Errorf("bgp operation %s peer %q not found in config", op.Type, peerName)
		}
		m, ok := rawPeer.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("bgp operation %s peer %q has type %T", op.Type, peerName, rawPeer)
		}
		peerTree = m
	}
	explicitPort := operationPeerRemotePortExplicit(peerTree)

	settings, err := parsePeerFromTree(peerName, peerTree, localAS, routerID)
	if err != nil {
		return nil, err
	}
	if !explicitPort && a != nil && a.r != nil && a.r.config.Port > 0 && a.r.config.Port <= 65535 {
		settings.Port = uint16(a.r.config.Port)
	}
	return settings, nil
}

// operationPeerRemotePortExplicit reports whether the operation's peer config
// names the port Ze dials, which is the one value the daemon's port would
// otherwise supply.
//
// connection > local > port is not that value. It names the port a listener of
// this peer's own answers on (PeerSettings.LocalPort), so a config carrying it
// alone has said nothing about the dial target. It was read here until
// 2026-09-06, when one field carried both endpoints (peer_settings.go).
func operationPeerRemotePortExplicit(peer map[string]any) bool {
	conn, ok := peer["connection"].(map[string]any)
	if !ok {
		return false
	}
	remote, ok := conn["remote"].(map[string]any)
	if !ok {
		return false
	}
	_, exists := remote["port"]
	return exists
}

// removePeerForOperation removes a peer the configuration no longer holds.
func (a *reactorAPIAdapter) removePeerForOperation(settings *PeerSettings) error {
	return a.r.RemovePeer(settings.Address)
}

// restartPeerForOperation removes a peer the configuration still holds, before
// the operation adds it back under its changed configuration.
// RFC 4486 Section 4: the Cease says "Other Configuration Change" (subcode 6),
// not "Peer De-configured" (subcode 3), because the peer was not de-configured.
func (a *reactorAPIAdapter) restartPeerForOperation(settings *PeerSettings) error {
	return a.r.restartPeer(settings.Address)
}

// swapPeerForOperation applies a modify-peer to the RUNNING session where the
// change is one a running session can take, and reports whether it did.
//
// The decision is peerSettingsSwapPlan's, which is the decision
// reconcilePeersJournaled takes for the same peer on the section apply path
// (reactor_api.go). Both paths MUST reach it: a modify-peer that always removed
// and re-added bounced the session for a change the reload was able to swap in
// place, so the operator lost the session, the peer re-learned every route, and
// which path the reload took decided whether that happened.
//
// A dynamic peer is never swapped. Its running settings were resolved at
// establishment rather than read from the configuration
// (resolveDynamicPeerSettings, reactor_dynamic.go), so they are not a candidate
// the config entry can be diffed against, and its establishment goroutine
// writes fields applyHotSwappableSettings would write here.
func (a *reactorAPIAdapter) swapPeerForOperation(next *PeerSettings, policyResolved bool, j configJournal) (bool, error) {
	if next == nil {
		return false, nil
	}
	key := next.PeerKey()

	a.r.mu.RLock()
	peer := a.r.peers[key]
	a.r.mu.RUnlock()
	if peer == nil {
		return false, nil
	}

	current := peer.settingsSnapshot()
	if current == nil || current.IsDynamic {
		return false, nil
	}

	apply, reason := peerSettingsSwapPlan(current, next, peer.currentSession())
	if reason != "" {
		reactorLogger().Info("peer restart required", "phase", "operation", "peer", key, "changed", reason)
		return false, nil
	}
	if err := a.swapPeerSettingsJournaled([]peerSettingsSwap{{key: key, next: next, apply: apply, policyUnresolved: !policyResolved}}, j); err != nil {
		return false, err
	}
	reactorLogger().Debug("peer settings swapped in place", "phase", "operation", "peer", key)
	return true, nil
}

func bgpOperationApplyOutput(settings *PeerSettings) *rpc.ConfigOperationApplyOutput {
	out := &rpc.ConfigOperationApplyOutput{Status: rpc.StatusOK}
	if settings.LocalAddress.IsValid() {
		out.Readiness = []rpc.ConfigOperationReadiness{{
			Namespace: bgpevents.Namespace,
			EventType: bgpevents.EventListenerReady,
			Resource:  settings.LocalAddress.String(),
		}}
	}
	return out
}

func operationRootLocalAS(root map[string]any) uint32 {
	asn, ok := operationNestedString(root, "session", "asn", "local")
	if !ok {
		return 0
	}
	var out uint32
	parseUint32FromString(asn, &out)
	return out
}

func operationRootRouterID(root map[string]any) uint32 {
	raw, ok := root["router-id"].(string)
	if !ok || raw == "" {
		return 0
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return 0
	}
	return ipToUint32(addr)
}

func operationNestedString(root map[string]any, path ...string) (string, bool) {
	var current any = root
	for _, part := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current = m[part]
	}
	value, ok := current.(string)
	return value, ok
}

func firstOperationString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
