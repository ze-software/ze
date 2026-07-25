// Design: plan/learned/1055-config-apply-ordering.md -- BGP-owned operation decomposition
// Related: register.go -- SDK operation callback wiring

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	configtx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootBGP = "bgp"

var (
	errBGPOperationNoReactor          = errors.New("bgp operation: no reactor available")
	errBGPOperationUnsupportedReactor = errors.New("bgp operation: reactor does not support operation callbacks")
)

type bgpOperationReactor interface {
	VerifyConfigOperation(*rpc.ConfigOperation) error
	ApplyConfigOperation(*rpc.ConfigOperation, registry.ConfigJournal) (*rpc.ConfigOperationApplyOutput, error)
}

func init() {
	if err := configtx.RegisterOperationDecomposer(configRootBGP, decomposeBGPOperations); err != nil {
		slog.Error("register bgp operation decomposer", "error", err)
		panic("BUG: register bgp operation decomposer failed")
	}
	if err := configtx.RegisterConstraintRule(configtx.ConstraintRule{
		ID:       "bgp-add-address-before-peer",
		Before:   configtx.OperationSelector{Type: configtx.OperationAddAddress, ResourceKind: configtx.ResourceAddress},
		After:    configtx.OperationSelector{Type: configtx.OperationAddPeer, ResourceKind: configtx.ResourcePeer},
		Relation: configtx.ResourceRelationAddressUsedBy,
	}); err != nil {
		slog.Error("register bgp constraint rule", "error", err)
	}
	if err := configtx.RegisterConstraintRule(configtx.ConstraintRule{
		ID:       "bgp-remove-peer-before-address",
		Before:   configtx.OperationSelector{Type: configtx.OperationRemovePeer, ResourceKind: configtx.ResourcePeer},
		After:    configtx.OperationSelector{Type: configtx.OperationRemoveAddress, ResourceKind: configtx.ResourceAddress},
		Relation: configtx.ResourceRelationAddressUsedBy,
	}); err != nil {
		slog.Error("register bgp constraint rule", "error", err)
	}
	if err := configtx.RegisterConstraintRule(configtx.ConstraintRule{
		ID:       "bgp-add-address-before-listener",
		Before:   configtx.OperationSelector{Type: configtx.OperationAddAddress, ResourceKind: configtx.ResourceAddress},
		After:    configtx.OperationSelector{Type: configtx.OperationAddListener, ResourceKind: configtx.ResourceListener},
		Relation: configtx.ResourceRelationAddressUsedBy,
	}); err != nil {
		slog.Error("register bgp constraint rule", "error", err)
	}
	if err := configtx.RegisterConstraintRule(configtx.ConstraintRule{
		ID:       "bgp-remove-listener-before-address",
		Before:   configtx.OperationSelector{Type: configtx.OperationRemoveListener, ResourceKind: configtx.ResourceListener},
		After:    configtx.OperationSelector{Type: configtx.OperationRemoveAddress, ResourceKind: configtx.ResourceAddress},
		Relation: configtx.ResourceRelationAddressUsedBy,
	}); err != nil {
		slog.Error("register bgp constraint rule", "error", err)
	}
	if err := configtx.RegisterSettlementRule(configtx.SettlementRule{
		ID:           "bgp-add-peer-settles-listener-ready",
		Operation:    configtx.OperationSelector{Type: configtx.OperationAddPeer, ResourceKind: configtx.ResourcePeer},
		Readiness:    configtx.ConfigOperationReadiness{Namespace: "bgp", EventType: "listener-ready"},
		ResourceFrom: configtx.SettlementResourceAddress,
		Timeout:      10 * time.Second,
	}); err != nil {
		slog.Error("register bgp settlement rule", "error", err)
	}
}

func decomposeBGPOperations(_ context.Context, req configtx.DecomposeRequest) ([]configtx.ConfigOperation, error) {
	if req.Root != configRootBGP || !bgpDiffTouchesPeer(req.Diff) {
		return nil, nil
	}
	activeRoot, err := parseBGPOperationRoot(req.ActiveRoot)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose active: %w", err)
	}
	candidateRoot, err := parseBGPOperationRoot(req.CandidateRoot)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose candidate: %w", err)
	}
	activePeers, err := collectBGPOperationPeers(activeRoot)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose active peers: %w", err)
	}
	candidatePeers, err := collectBGPOperationPeers(candidateRoot)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose candidate peers: %w", err)
	}

	var ops []configtx.ConfigOperation
	var sameAddressChanges []bgpPeerChange
	for _, name := range sortedBGPPeerNames(activePeers) {
		activePeer := activePeers[name]
		candidatePeer, exists := candidatePeers[name]
		if !exists {
			ops = append(ops, bgpPeerOperation(configtx.OperationRemovePeer, name, activePeer.localAddress, nil, activePeer.raw))
			continue
		}
		if string(activePeer.raw) == string(candidatePeer.raw) {
			continue
		}
		if activePeer.localAddress != candidatePeer.localAddress {
			ops = append(ops,
				bgpPeerOperation(configtx.OperationRemovePeer, name, activePeer.localAddress, nil, activePeer.raw),
				bgpPeerOperation(configtx.OperationAddPeer, name, candidatePeer.localAddress, candidatePeer.raw, nil),
			)
			continue
		}
		sameAddressChanges = append(sameAddressChanges, bgpPeerChange{name: name, active: activePeer, candidate: candidatePeer})
	}
	if peerRouterIDRotation(sameAddressChanges) {
		for _, change := range sameAddressChanges {
			ops = append(ops, bgpPeerOperation(configtx.OperationRemovePeer, change.name, change.active.localAddress, nil, change.active.raw))
		}
		for _, change := range sameAddressChanges {
			ops = append(ops, bgpPeerOperation(configtx.OperationAddPeer, change.name, change.candidate.localAddress, change.candidate.raw, nil))
		}
	} else {
		for _, change := range sameAddressChanges {
			ops = append(ops, bgpModifyPeerOperation(change.name, change.candidate.localAddress, change.candidate.raw, change.active.raw))
		}
	}
	for _, name := range sortedBGPPeerNames(candidatePeers) {
		if _, exists := activePeers[name]; exists {
			continue
		}
		candidatePeer := candidatePeers[name]
		ops = append(ops, bgpPeerOperation(configtx.OperationAddPeer, name, candidatePeer.localAddress, candidatePeer.raw, nil))
	}
	return ops, nil
}

func decomposeBGPOperationInput(ctx context.Context, input sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
	ops, err := decomposeBGPOperations(ctx, configtx.DecomposeRequest{
		TransactionID: input.TransactionID,
		Root:          input.Root,
		ActiveRoot:    input.Active.Data,
		CandidateRoot: input.Candidate.Data,
		Diff: configtx.DiffSection{
			Root:    input.Diff.Root,
			Added:   input.Diff.Added,
			Removed: input.Diff.Removed,
			Changed: input.Diff.Changed,
		},
	})
	if err != nil {
		return nil, err
	}
	return &sdk.ConfigOperationDecomposeOutput{Status: rpc.StatusOK, Operations: ops}, nil
}

type bgpOperationPeer struct {
	localAddress string
	routerID     string
	raw          json.RawMessage
}

type bgpPeerChange struct {
	name      string
	active    bgpOperationPeer
	candidate bgpOperationPeer
}

func parseBGPOperationRoot(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, err
	}
	if bgp, ok := root[configRootBGP].(map[string]any); ok {
		return bgp, nil
	}
	return root, nil
}

func collectBGPOperationPeers(root map[string]any) (map[string]bgpOperationPeer, error) {
	peerSection, ok := root["peer"]
	if !ok || peerSection == nil {
		return map[string]bgpOperationPeer{}, nil
	}
	peerMap, ok := peerSection.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("peer section has type %T", peerSection)
	}
	peers := make(map[string]bgpOperationPeer, len(peerMap))
	for name, rawPeer := range peerMap {
		peer, ok := rawPeer.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("peer %s has type %T", name, rawPeer)
		}
		merged, err := cloneStringAnyMap(peer)
		if err != nil {
			return nil, fmt.Errorf("clone peer %s: %w", name, err)
		}
		injectBGPGlobalPeerDefaults(root, merged)
		data, err := json.Marshal(merged)
		if err != nil {
			return nil, fmt.Errorf("marshal peer %s: %w", name, err)
		}
		peers[name] = bgpOperationPeer{
			localAddress: bgpPeerLocalAddress(merged),
			routerID:     nestedString(merged, "session", "router-id"),
			raw:          data,
		}
	}
	return peers, nil
}

func injectBGPGlobalPeerDefaults(root, peer map[string]any) {
	if localAS := nestedString(root, "session", "asn", "local"); localAS != "" && nestedString(peer, "session", "asn", "local") == "" {
		ensureNestedMap(peer, "session", "asn")["local"] = localAS
	}
	if routerID, ok := root["router-id"].(string); ok && routerID != "" && nestedString(peer, "session", "router-id") == "" {
		ensureNestedMap(peer, "session")["router-id"] = routerID
	}
}

func bgpPeerOperation(opType configtx.ConfigOperationType, name, localAddress string, config, oldConfig json.RawMessage) configtx.ConfigOperation {
	verb := "add"
	if opType == configtx.OperationRemovePeer {
		verb = "remove"
	}
	params := configtx.ConfigOperationParams{Peer: name, Address: localAddress}
	if len(config) > 0 {
		params.Config = config
	}
	if len(oldConfig) > 0 {
		params.OldConfig = oldConfig
	}
	return configtx.ConfigOperation{
		ID:    "bgp-" + verb + "-peer-" + sanitizeBGPOperationID(name),
		Root:  configRootBGP,
		Owner: "bgp",
		Type:  opType,
		Target: configtx.ResourceRef{
			Kind:    configtx.ResourcePeer,
			Peer:    name,
			Address: localAddress,
		},
		Params: params,
	}
}

func bgpModifyPeerOperation(name, localAddress string, config, oldConfig json.RawMessage) configtx.ConfigOperation {
	return configtx.ConfigOperation{
		ID:    "bgp-modify-peer-" + sanitizeBGPOperationID(name),
		Root:  configRootBGP,
		Owner: "bgp",
		Type:  configtx.OperationModifyPeer,
		Target: configtx.ResourceRef{
			Kind:    configtx.ResourcePeer,
			Peer:    name,
			Address: localAddress,
		},
		Params: configtx.ConfigOperationParams{
			Peer:      name,
			Address:   localAddress,
			Config:    config,
			OldConfig: oldConfig,
		},
	}
}

func bgpDiffTouchesPeer(diff configtx.DiffSection) bool {
	for _, raw := range []string{diff.Added, diff.Removed, diff.Changed} {
		if raw == "" {
			continue
		}
		var entries map[string]any
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return false
		}
		for key := range entries {
			if key == "bgp/peer" || strings.HasPrefix(key, "bgp/peer/") || key == "peer" || strings.HasPrefix(key, "peer/") {
				return true
			}
		}
	}
	return false
}

func peerRouterIDRotation(changes []bgpPeerChange) bool {
	if len(changes) < 2 {
		return false
	}
	activeOwner := make(map[string]string, len(changes))
	for _, change := range changes {
		if change.active.routerID != "" {
			activeOwner[change.active.routerID] = change.name
		}
	}
	for _, change := range changes {
		if change.candidate.routerID == "" || change.candidate.routerID == change.active.routerID {
			continue
		}
		if owner := activeOwner[change.candidate.routerID]; owner != "" && owner != change.name {
			return true
		}
	}
	return false
}

func bgpPeerLocalAddress(peer map[string]any) string {
	addr := nestedString(peer, "connection", "local", "ip")
	if addr == "auto" {
		return ""
	}
	return addr
}

func sortedBGPPeerNames(peers map[string]bgpOperationPeer) []string {
	names := make([]string, 0, len(peers))
	for name := range peers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cloneStringAnyMap(in map[string]any) (map[string]any, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func nestedString(root map[string]any, path ...string) string {
	var current any = root
	for _, part := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[part]
	}
	value, _ := current.(string)
	return value
}

func ensureNestedMap(root map[string]any, path ...string) map[string]any {
	current := root
	for _, part := range path {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	return current
}

func sanitizeBGPOperationID(value string) string {
	replacer := strings.NewReplacer("/", "_", ":", "_", " ", "_")
	return replacer.Replace(value)
}

func operationJournalKey(txID, opID string) string {
	return txID + "\x00" + opID
}

func bgpOperationAdapter(handle registry.BGPReactorHandle) (bgpOperationReactor, error) {
	if handle == nil {
		return nil, errBGPOperationNoReactor
	}
	adapter, ok := handle.(bgpOperationReactor)
	if !ok {
		return nil, errBGPOperationUnsupportedReactor
	}
	return adapter, nil
}

func verifyBGPOperation(op *sdk.ConfigOperation, handle registry.BGPReactorHandle) error {
	adapter, err := bgpOperationAdapter(handle)
	if err != nil {
		return err
	}
	return adapter.VerifyConfigOperation(op)
}

func applyBGPOperation(op *sdk.ConfigOperation, handle registry.BGPReactorHandle, journal registry.ConfigJournal) (*sdk.ConfigOperationApplyOutput, error) {
	adapter, err := bgpOperationAdapter(handle)
	if err != nil {
		return nil, err
	}
	return adapter.ApplyConfigOperation(op, journal)
}
