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

// VerifyConfigOperation validates one BGP config operation without mutating state.
func (r *Reactor) VerifyConfigOperation(op *rpc.ConfigOperation) error {
	return (&reactorAPIAdapter{r: r}).verifyConfigOperation(op)
}

// ApplyConfigOperation applies one BGP config operation and records its inverse.
func (r *Reactor) ApplyConfigOperation(op *rpc.ConfigOperation, j registry.ConfigJournal) (*rpc.ConfigOperationApplyOutput, error) {
	return (&reactorAPIAdapter{r: r}).applyConfigOperation(op, j)
}

func (a *reactorAPIAdapter) verifyConfigOperation(op *rpc.ConfigOperation) error {
	if op == nil {
		return errors.New("bgp operation verify requires an operation")
	}
	switch op.Type {
	case configop.AddPeer:
		_, err := a.candidatePeerSettingsFromOperationConfig(op)
		return err
	case configop.RemovePeer:
		_, err := a.peerSettingsFromOperationConfig(op, op.Params.OldConfig)
		return err
	case configop.ModifyPeer:
		if _, err := a.peerSettingsFromOperationConfig(op, op.Params.OldConfig); err != nil {
			return err
		}
		_, err := a.candidatePeerSettingsFromOperationConfig(op)
		return err
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
	switch op.Type {
	case configop.AddPeer:
		settings, err := a.candidatePeerSettingsFromOperationConfig(op)
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
		settings, err := a.peerSettingsFromOperationConfig(op, op.Params.OldConfig)
		if err != nil {
			return nil, err
		}
		if err := j.Record(
			func() error { return a.removePeerForOperation(settings) },
			func() error { return a.r.AddPeer(settings) },
		); err != nil {
			return nil, err
		}
		return &rpc.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
	case configop.ModifyPeer:
		oldSettings, err := a.peerSettingsFromOperationConfig(op, op.Params.OldConfig)
		if err != nil {
			return nil, err
		}
		newSettings, err := a.candidatePeerSettingsFromOperationConfig(op)
		if err != nil {
			return nil, err
		}
		swapped, err := a.swapPeerForOperation(newSettings, j)
		if err != nil {
			return nil, err
		}
		if swapped {
			a.emitOperationListenerReady(newSettings)
			return bgpOperationApplyOutput(newSettings), nil
		}
		if err := j.Record(
			func() error { return a.removePeerForOperation(oldSettings) },
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
	default:
		return nil, fmt.Errorf("bgp operation %s not supported", op.Type)
	}
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

func (a *reactorAPIAdapter) candidatePeerSettingsFromOperationConfig(op *rpc.ConfigOperation) (*PeerSettings, error) {
	settings, err := a.peerSettingsFromReloadConfig(op)
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, errPeerNotInReloadConfig) {
		return nil, err
	}
	return a.peerSettingsFromOperationConfig(op, op.Params.Config)
}

func (a *reactorAPIAdapter) peerSettingsFromReloadConfig(op *rpc.ConfigOperation) (*PeerSettings, error) {
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
	peers, err := reloadFn(configPath)
	if err != nil {
		return nil, fmt.Errorf("bgp operation %s load candidate peer config: %w", op.Type, err)
	}
	for _, peer := range peers {
		if peer != nil && peer.Name == peerName {
			return peer, nil
		}
	}
	return nil, errPeerNotInReloadConfig
}

func (a *reactorAPIAdapter) peerSettingsFromOperationConfig(op *rpc.ConfigOperation, raw json.RawMessage) (*PeerSettings, error) {
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
	if peerSection, ok := bgpRoot["peer"].(map[string]any); ok {
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

func (a *reactorAPIAdapter) removePeerForOperation(settings *PeerSettings) error {
	return a.r.RemovePeer(settings.Address)
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
func (a *reactorAPIAdapter) swapPeerForOperation(next *PeerSettings, j configJournal) (bool, error) {
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
	if err := a.swapPeerSettingsJournaled([]peerSettingsSwap{{key: key, next: next, apply: apply}}, j); err != nil {
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
