// Design: docs/architecture/config/apply-ordering.md -- BGP-owned operation decomposition
// Related: register.go -- SDK operation callback wiring

package plugin

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	configtx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/capture"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootBGP = "bgp"

var (
	errBGPOperationNoReactor          = errors.New("bgp operation: no reactor available")
	errBGPOperationUnsupportedReactor = errors.New("bgp operation: reactor does not support operation callbacks")
)

type bgpOperationReactor interface {
	// VerifyConfigOperation verifies one operation of the named transaction,
	// whose operations share one candidate resolution with its apply.
	VerifyConfigOperation(txID string, op *rpc.ConfigOperation) error
	ApplyConfigOperation(*rpc.ConfigOperation, registry.ConfigJournal) (*rpc.ConfigOperationApplyOutput, error)
	// CheckSourcePolicyTx refuses an operation commit whose in-place import
	// invalidations ran out of admission budget, so the transaction rolls back.
	CheckSourcePolicyTx(txID string) error
	// CompleteSourcePolicyTx finalizes (accepted) or settles (rejected) the
	// transaction's import invalidations and resumes their sessions' input.
	CompleteSourcePolicyTx(ctx context.Context, txID string, accepted bool) error
}

// bgpOperationJournal is one config operation's journal. It carries the
// operation's transaction, so a swap that removes a peer's last accepting
// import policy can hold its undo until the whole reload is accepted or
// rejected (reactor.sourcePolicyTransaction). The section path's plain
// journal carries none, and the reactor refuses that change there.
type bgpOperationJournal struct {
	*sdk.Journal
	txID     string
	deadline time.Time
	// order is the journal's place in apply order, so a receipt that must
	// undo what is still recorded undoes the last operation first.
	order uint64
}

// bgpOperationJournalOrder issues bgpOperationJournal.order. Safe for
// concurrent use.
var bgpOperationJournalOrder atomic.Uint64

func newBGPOperationJournal(txID string, deadlineMS int64) *bgpOperationJournal {
	j := &bgpOperationJournal{Journal: sdk.NewJournal(), txID: txID, order: bgpOperationJournalOrder.Add(1)}
	if deadlineMS > 0 {
		j.deadline = time.UnixMilli(deadlineMS)
	}
	return j
}

// SourcePolicyTransaction names the transaction and its apply deadline; a zero
// deadline means the operation carried none.
func (j *bgpOperationJournal) SourcePolicyTransaction() (string, time.Time) {
	return j.txID, j.deadline
}

func checkBGPSourcePolicy(txID string, handle registry.BGPReactorHandle) error {
	adapter, err := bgpOperationAdapter(handle)
	if err != nil {
		return err
	}
	return adapter.CheckSourcePolicyTx(txID)
}

func completeBGPSourcePolicy(ctx context.Context, txID string, accepted bool, handle registry.BGPReactorHandle) error {
	adapter, err := bgpOperationAdapter(handle)
	if err != nil {
		return err
	}
	return adapter.CompleteSourcePolicyTx(ctx, txID, accepted)
}

// completeBGPTransaction is the BGP plugin's config-complete receipt for
// txID. journals holds every operation journal still recorded, keyed by
// operationJournalKey; the caller holds the lock that guards it.
//
// On rejection the journals of txID still recorded are rolled back first, last
// operation first, so an import invalidation the executor's rollback never
// reached is undone with its settings restore (RFC 8212 Section 3: the
// candidate's rejecting policy is withdrawn before the owners restore the
// source's routes), and only then is the source-policy receipt delivered,
// which resumes the paused input under the restored policy. On acceptance the
// receipt finalizes first and the journals are discarded. Either way no
// journal of txID is left behind. complete is the reactor's source-policy
// receipt (completeBGPSourcePolicy), nil when no reactor runs.
func completeBGPTransaction(ctx context.Context, txID string, accepted bool, journals map[string]*bgpOperationJournal,
	complete func(context.Context, string, bool) error,
) error {
	var held []*bgpOperationJournal
	for key, j := range journals {
		if strings.HasPrefix(key, txID+"\x00") {
			held = append(held, j)
			delete(journals, key)
		}
	}
	slices.SortFunc(held, func(a, b *bgpOperationJournal) int { return cmp.Compare(b.order, a.order) })
	var result error
	if !accepted {
		for _, j := range held {
			if errs := j.Rollback(); len(errs) > 0 {
				result = errors.Join(result, fmt.Errorf("bgp transaction %s rejected: undo of an operation the rollback did not reach failed: %w", txID, errors.Join(errs...)))
			}
		}
	}
	if complete != nil {
		result = errors.Join(result, complete(ctx, txID, accepted))
	}
	if accepted {
		for _, j := range held {
			j.Discard()
		}
	}
	return errors.Join(result, ctx.Err())
}

func init() {
	if err := configtx.RegisterOperationDecomposer(configRootBGP, decomposeBGPOperations); err != nil {
		slog.Error("register bgp operation decomposer", "error", err)
		panic("BUG: register bgp operation decomposer failed")
	}
	// This root registers no constraint rule. Its four rules each said one
	// thing, that a peer or a listener needs its local address to exist, and
	// each had to spell the `interface` root's operation labels to say it.
	// The peer operations declare that address in Consumes instead, and the
	// graph derives the same two edges from the address operation that
	// produces it (BuildOperationGraph in the transaction package).
	if err := configtx.RegisterSettlementRule(configtx.SettlementRule{
		ID:           "bgp-add-peer-settles-listener-ready",
		Operation:    configtx.OperationSelector{Type: configop.AddPeer, ResourceKind: configtx.ResourcePeer},
		Readiness:    configtx.ConfigOperationReadiness{Namespace: bgpevents.Namespace, EventType: "listener-ready"},
		ResourceFrom: configtx.SettlementResourceAddress,
		Timeout:      10 * time.Second,
	}); err != nil {
		slog.Error("register bgp settlement rule", "error", err)
	}
}

func decomposeBGPOperations(_ context.Context, req configtx.DecomposeRequest) ([]configtx.ConfigOperation, error) {
	if req.Root != configRootBGP {
		return nil, nil
	}
	touchesPeer, err := bgpDiffTouchesPeer(req.Diff)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose diff: %w", err)
	}
	activeRoot, err := parseBGPSection(req.ActiveRoot)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose active: %w", err)
	}
	candidateRoot, err := parseBGPSection(req.CandidateRoot)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose candidate: %w", err)
	}
	importChanges, err := bgpImportLayerChanges(activeRoot, candidateRoot)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose import layers: %w", err)
	}
	// A commit that disturbs an address reaches this root with no diff at all,
	// and the sessions bound to that address still have to stop and start
	// (docs/architecture/config/apply-ordering.md, phase 2).
	if !touchesPeer && len(req.DisturbedAddresses) == 0 && len(importChanges) == 0 {
		return nil, nil
	}
	disturbed := req.DisturbedAddresses
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
			ops = append(ops, bgpPeerOperation(configop.RemovePeer, name, activePeer.localAddress, nil, activePeer.raw, disturbed))
			continue
		}
		// Phase 2 and phase 5 of the requirement: the session is stopped
		// before the address it binds leaves the host and started after the
		// address arrives again, even where the peer's own config is byte for
		// byte what it was. The graph derives both edges from the address
		// this pair declares in Consumes.
		if peerBindingDisturbed(disturbed, activePeer.localAddress) {
			ops = append(ops,
				bgpPeerOperation(configop.RemovePeer, name, activePeer.localAddress, candidatePeer.raw, activePeer.raw, disturbed),
				bgpPeerOperation(configop.AddPeer, name, candidatePeer.localAddress, candidatePeer.raw, nil, disturbed),
			)
			continue
		}
		if string(activePeer.raw) == string(candidatePeer.raw) {
			continue
		}
		if activePeer.localAddress != candidatePeer.localAddress {
			ops = append(ops,
				bgpPeerOperation(configop.RemovePeer, name, activePeer.localAddress, candidatePeer.raw, activePeer.raw, disturbed),
				bgpPeerOperation(configop.AddPeer, name, candidatePeer.localAddress, candidatePeer.raw, nil, disturbed),
			)
			continue
		}
		sameAddressChanges = append(sameAddressChanges, bgpPeerChange{name: name, active: activePeer, candidate: candidatePeer})
	}
	if peerRouterIDRotation(sameAddressChanges) {
		for _, change := range sameAddressChanges {
			ops = append(ops, bgpPeerOperation(configop.RemovePeer, change.name, change.active.localAddress, change.candidate.raw, change.active.raw, disturbed))
		}
		for _, change := range sameAddressChanges {
			ops = append(ops, bgpPeerOperation(configop.AddPeer, change.name, change.candidate.localAddress, change.candidate.raw, nil, disturbed))
		}
	} else {
		for _, change := range sameAddressChanges {
			ops = append(ops, bgpModifyPeerOperation(change.name, change.candidate.localAddress, change.candidate.raw, change.active.raw, disturbed))
		}
	}
	for _, name := range sortedBGPPeerNames(candidatePeers) {
		if _, exists := activePeers[name]; exists {
			continue
		}
		candidatePeer := candidatePeers[name]
		ops = append(ops, bgpPeerOperation(configop.AddPeer, name, candidatePeer.localAddress, candidatePeer.raw, nil, disturbed))
	}
	// RFC 8212 Section 3: a change to an import layer the peer inherits (bgp,
	// group) or to a grouped peer's own policy is a modify-peer, so the swap
	// runs under this operation's journal and transaction and a removed
	// accepting policy is invalidated in place. The whole-section apply
	// carries no transaction and refuses that change.
	for _, name := range slices.Sorted(maps.Keys(importChanges)) {
		if slices.ContainsFunc(ops, func(op configtx.ConfigOperation) bool { return op.Params.Peer == name }) {
			continue
		}
		change := importChanges[name]
		ops = append(ops, bgpModifyPeerOperation(name, change.candidate.localAddress, change.candidate.raw, change.active.raw, disturbed))
	}
	residual, err := bgpResidualOperation(activeRoot, candidateRoot, ops, disturbed)
	if err != nil {
		return nil, fmt.Errorf("bgp operation decompose residual: %w", err)
	}
	return append(ops, residual...), nil
}

// bgpResidualOperation carries the part of the diff ops leave behind. A
// participant that owns operations gets no section apply on the operation
// path (operationNodes in the transaction package), so a change no operation
// carries would never be applied: a bgp-level leaf such as router-id, a group
// level such as another group's hold-time, a dynamic group (a group-level
// template with no static peer), beside an operation for a peer. It answers one
// reconcile-peers operation when the bgp root without its static peer bodies
// differs on the two sides (bgpPeerlessSkeleton), whether or not every peer
// is covered, or when a peer that no operation in ops names resolves to other
// settings, and nothing otherwise. Its apply is the section apply's reconcile
// (globals, dynamic groups, peers) under the operation's journal. The
// operation carries the candidate root and consumes every peer ops produce,
// so the graph runs it after them and the peers they already moved are
// unchanged by it; it consumes the local address of each residual peer the
// candidate keeps, as a binder does.
func bgpResidualOperation(activeRoot, candidateRoot map[string]any, ops []configtx.ConfigOperation, disturbed []string) ([]configtx.ConfigOperation, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	active, err := collectBGPEffectivePeers(activeRoot)
	if err != nil {
		return nil, fmt.Errorf("active: %w", err)
	}
	candidate, err := collectBGPEffectivePeers(candidateRoot)
	if err != nil {
		return nil, fmt.Errorf("candidate: %w", err)
	}
	covered := make(map[string]bool, len(ops))
	for i := range ops {
		covered[ops[i].Params.Peer] = true
	}
	var residual []string
	for _, name := range slices.Sorted(maps.Keys(active)) {
		after, kept := candidate[name]
		if covered[name] {
			continue
		}
		if kept && after.resolved == active[name].resolved {
			continue
		}
		residual = append(residual, name)
	}
	for _, name := range slices.Sorted(maps.Keys(candidate)) {
		if _, existed := active[name]; existed {
			continue
		}
		if covered[name] {
			continue
		}
		residual = append(residual, name)
	}
	skeletonChanged, err := bgpPeerlessSkeletonChanged(activeRoot, candidateRoot)
	if err != nil {
		return nil, err
	}
	if len(residual) == 0 && !skeletonChanged {
		return nil, nil
	}
	data, err := json.Marshal(candidateRoot)
	if err != nil {
		return nil, fmt.Errorf("marshal candidate root: %w", err)
	}
	var consumes []configtx.ResourceRef
	for _, name := range slices.Sorted(maps.Keys(covered)) {
		consumes = append(consumes, configtx.ResourceRef{Kind: configtx.ResourcePeer, Peer: name})
	}
	for _, name := range residual {
		if peer, kept := candidate[name]; kept {
			consumes = append(consumes, peerAddressConsumes(peer.localAddress, disturbed)...)
		}
	}
	return []configtx.ConfigOperation{{
		ID:       "bgp-reconcile-peers",
		Root:     configRootBGP,
		Owner:    pluginNameBGP,
		Type:     configop.ReconcilePeers,
		Verb:     configtx.VerbModify,
		Target:   configtx.ResourceRef{Kind: configtx.ResourcePeer},
		Consumes: consumes,
		Params:   configtx.ConfigOperationParams{Config: data},
	}}, nil
}

// bgpPeerlessSkeletonChanged reports whether the bgp root differs on the two
// sides once every static peer body is taken out: the bgp-level leaves, each
// group's own leaves (a dynamic group's template and ranges among them), and
// the set and order of the groups. A change there is applied by the reconcile
// (globals, dynamic groups), never by a peer operation.
func bgpPeerlessSkeletonChanged(activeRoot, candidateRoot map[string]any) (bool, error) {
	active, err := bgpPeerlessSkeleton(activeRoot)
	if err != nil {
		return false, fmt.Errorf("active: %w", err)
	}
	candidate, err := bgpPeerlessSkeleton(candidateRoot)
	if err != nil {
		return false, fmt.Errorf("candidate: %w", err)
	}
	return active != candidate, nil
}

// bgpPeerlessSkeleton marshals root with the static peer lists removed, at the
// bgp level and in every group. encoding/json sorts map keys, so two equal
// skeletons marshal to the same string.
func bgpPeerlessSkeleton(root map[string]any) (string, error) {
	skeleton := maps.Clone(root)
	delete(skeleton, "peer")
	if groups, ok := root["group"].(map[string]any); ok {
		levels := make(map[string]any, len(groups))
		for name, body := range groups {
			group, ok := body.(map[string]any)
			if !ok {
				return "", fmt.Errorf("group %s has type %T", name, body)
			}
			level := maps.Clone(group)
			delete(level, "peer")
			levels[name] = level
		}
		skeleton["group"] = levels
	}
	data, err := json.Marshal(skeleton)
	if err != nil {
		return "", fmt.Errorf("marshal bgp skeleton: %w", err)
	}
	return string(data), nil
}

// bgpEffectivePeer is what one peer's settings resolve from.
type bgpEffectivePeer struct {
	resolved     string
	localAddress string
}

// collectBGPEffectivePeers maps every peer, top-level or grouped, to the
// config its settings resolve from: the bgp level's own leaves, its group's
// name and own leaves, and its body (config.PeersFromTree layers them in that
// order). Two sides that map a peer to the same string resolve it alike.
func collectBGPEffectivePeers(root map[string]any) (map[string]bgpEffectivePeer, error) {
	level := maps.Clone(root)
	delete(level, "peer")
	delete(level, "group")
	peers := make(map[string]bgpEffectivePeer)
	add := func(layer map[string]any, group string, groupLevel map[string]any) error {
		peerMap, err := bgpSubtree(layer, "peer")
		if err != nil {
			return err
		}
		for name, body := range peerMap {
			peer, ok := body.(map[string]any)
			if !ok {
				return fmt.Errorf("peer %s has type %T", name, body)
			}
			resolved, err := json.Marshal([]any{level, group, groupLevel, peer})
			if err != nil {
				return fmt.Errorf("marshal peer %s: %w", name, err)
			}
			peers[name] = bgpEffectivePeer{resolved: string(resolved), localAddress: bgpPeerLocalAddress(peer)}
		}
		return nil
	}
	if err := add(root, "", nil); err != nil {
		return nil, err
	}
	groups, err := bgpSubtree(root, "group")
	if err != nil {
		return nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		group, ok := groups[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("group %s has type %T", name, groups[name])
		}
		groupLevel := maps.Clone(group)
		delete(groupLevel, "peer")
		if err := add(group, name, groupLevel); err != nil {
			return nil, err
		}
	}
	return peers, nil
}

// bgpImportLayers is one peer's effective import policy inputs: the bgp, group
// and peer `filter import` layers config.PeersFromTree concatenates, in order.
type bgpImportLayers struct {
	peer   bgpOperationPeer
	layers string
}

// bgpImportLayerChanges names every peer, top-level or grouped, present on
// both sides whose import layers differ, with both sides' peer bodies.
func bgpImportLayerChanges(activeRoot, candidateRoot map[string]any) (map[string]bgpPeerChange, error) {
	active, err := collectBGPImportLayers(activeRoot)
	if err != nil {
		return nil, err
	}
	candidate, err := collectBGPImportLayers(candidateRoot)
	if err != nil {
		return nil, err
	}
	changes := make(map[string]bgpPeerChange)
	for name, before := range active {
		after, exists := candidate[name]
		if !exists {
			continue
		}
		if before.layers == after.layers {
			continue
		}
		changes[name] = bgpPeerChange{name: name, active: before.peer, candidate: after.peer}
	}
	return changes, nil
}

// collectBGPImportLayers reads every peer's import layers from one bgp root.
func collectBGPImportLayers(root map[string]any) (map[string]bgpImportLayers, error) {
	bgpLayer, err := bgpImportLayer(root)
	if err != nil {
		return nil, err
	}
	peers := make(map[string]bgpImportLayers)
	add := func(layer map[string]any, groupLayer any) error {
		peerMap, err := bgpSubtree(layer, "peer")
		if err != nil {
			return err
		}
		for name, rawPeer := range peerMap {
			body, ok := rawPeer.(map[string]any)
			if !ok {
				return fmt.Errorf("peer %s has type %T", name, rawPeer)
			}
			merged, err := cloneStringAnyMap(body)
			if err != nil {
				return fmt.Errorf("clone peer %s: %w", name, err)
			}
			injectBGPGlobalPeerDefaults(root, merged)
			data, err := json.Marshal(merged)
			if err != nil {
				return fmt.Errorf("marshal peer %s: %w", name, err)
			}
			peerLayer, err := bgpImportLayer(body)
			if err != nil {
				return fmt.Errorf("peer %s: %w", name, err)
			}
			layers, err := json.Marshal([]any{bgpLayer, groupLayer, peerLayer})
			if err != nil {
				return fmt.Errorf("marshal peer %s import layers: %w", name, err)
			}
			peers[name] = bgpImportLayers{
				peer:   bgpOperationPeer{localAddress: bgpPeerLocalAddress(merged), raw: data},
				layers: string(layers),
			}
		}
		return nil
	}
	if err := add(root, nil); err != nil {
		return nil, err
	}
	groups, err := bgpSubtree(root, "group")
	if err != nil {
		return nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		group, ok := groups[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("group %s has type %T", name, groups[name])
		}
		groupLayer, err := bgpImportLayer(group)
		if err != nil {
			return nil, fmt.Errorf("group %s: %w", name, err)
		}
		if err := add(group, groupLayer); err != nil {
			return nil, err
		}
	}
	return peers, nil
}

// bgpImportLayer is one layer's `filter import` subtree, nil when the layer
// configures no filter or no import. A filter that is not a container is an
// error: skipping it would read a policy the config does hold as absent.
func bgpImportLayer(layer map[string]any) (any, error) {
	filter, err := bgpSubtree(layer, "filter")
	if err != nil {
		return nil, err
	}
	imported := filter["import"]
	return imported, nil
}

// bgpSubtree answers layer's container key: nil when the layer does not set
// it, which ranges and indexes as empty, and an error when it holds anything
// but a container, so a malformed tree is refused rather than read as empty.
func bgpSubtree(layer map[string]any, key string) (map[string]any, error) {
	var subtree map[string]any
	if section := layer[key]; section != nil {
		typed, ok := section.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s has type %T", key, section)
		}
		subtree = typed
	}
	return subtree, nil
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
		DisturbedAddresses: input.DisturbedAddresses,
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

// bgpPeerOperation builds an add-peer or a remove-peer. A remove-peer that
// carries config is the remove half of a remove and add pair: the peer stays
// configured under that config, so the reactor ends its session with the Cease
// for a configuration change rather than for a removal (RFC 4486 Section 4).
func bgpPeerOperation(opType configtx.ConfigOperationType, name, localAddress string, config, oldConfig json.RawMessage, disturbed []string) configtx.ConfigOperation {
	verb := configtx.VerbCreate
	word := "add"
	if opType == configop.RemovePeer {
		verb = configtx.VerbDestroy
		word = "remove"
	}
	params := configtx.ConfigOperationParams{Peer: name, Address: localAddress}
	if len(config) > 0 {
		params.Config = config
	}
	if len(oldConfig) > 0 {
		params.OldConfig = oldConfig
	}
	return configtx.ConfigOperation{
		ID:     "bgp-" + word + "-peer-" + sanitizeBGPOperationID(name),
		Root:   configRootBGP,
		Owner:  pluginNameBGP,
		Type:   opType,
		Verb:   verb,
		Target: peerResource(name, localAddress),
		// The session is what this operation owns, and the local address it
		// binds is what it needs the `interface` root to have put on a device.
		// The create then runs after that address exists and the destroy runs
		// before it goes, which is the ordering the four deleted rules
		// hand-wrote for this one pair.
		Produces: []configtx.ResourceRef{{Kind: configtx.ResourcePeer, Peer: name}},
		Consumes: peerAddressConsumes(localAddress, disturbed),
		Params:   params,
	}
}

// peerResource is the peer a peer operation targets. The address rides along
// because the settlement rule reads it (SettlementResourceAddress), and it is
// no part of the peer's identity.
func peerResource(name, localAddress string) configtx.ResourceRef {
	return configtx.ResourceRef{
		Kind:    configtx.ResourcePeer,
		Peer:    name,
		Address: localAddress,
	}
}

// peerAddressConsumes declares the addresses a peer operation waits for.
//
// A peer with a configured local address declares that address, and the graph
// puts its create after the address arrives and its destroy before the address
// goes.
//
// A peer whose `connection.local.ip` is absent or `auto` lets the KERNEL pick
// the source address, so Ze cannot say which address the session holds. Where
// this commit disturbs none, there is nothing to wait for and the operation
// declares nothing, as it always has. Where it disturbs some, the peer
// declares every one of them, because its source could be any of them: that is
// the fail-safe default, and it is what orders the stop ahead of the removals
// and the start behind the additions. Guessing that the kernel picked an
// address this commit leaves alone is the reading that leaves a session
// holding an address that is gone (ai/rules/principles.md).
//
// The addresses arrive sorted from the core, and the order is kept, because it
// decides the order of the edges the graph derives from them.
//
// An entry naming no address would be refused by ValidateOperations, and it is
// the operation that must not carry one rather than the check that must
// tolerate it.
func peerAddressConsumes(localAddress string, disturbed []string) []configtx.ResourceRef {
	if localAddress == "" {
		if len(disturbed) == 0 {
			return nil
		}
		refs := make([]configtx.ResourceRef, 0, len(disturbed))
		for _, address := range disturbed {
			refs = append(refs, configtx.ResourceRef{Kind: configtx.ResourceAddress, Address: address})
		}
		return refs
	}
	return []configtx.ResourceRef{{Kind: configtx.ResourceAddress, Address: localAddress}}
}

// peerBindingDisturbed reports whether this commit takes away the address the
// named peer binds, so that the session has to stop before it goes and start
// after it comes back (docs/architecture/config/apply-ordering.md, phase 2).
//
// A peer with a configured local address is disturbed when THAT address is.
// A peer with none lets the kernel choose its source, so Ze cannot establish
// which address it holds, and the requirement's fail-safe default treats it as
// disturbed the moment any address moves: "If it is not easy to establish what
// action will lead to what, be safe and deconf/reconf". The cost of the wrong
// guess is not symmetric. Stopping a session that would have survived costs
// one restart; leaving one up costs a session bound to an address the kernel
// no longer has.
//
// Ze binds specific addresses, never the wildcard: CreateReactorFromTree sets
// no listen address and startListenerForAddressPort opens one listener per
// local address, so the page's wildcard carve-out frees no BGP session.
func peerBindingDisturbed(disturbed []string, localAddress string) bool {
	if len(disturbed) == 0 {
		return false
	}
	if localAddress == "" {
		return true
	}
	// The core names an address by its IP alone (resourceIdentity in
	// internal/component/config/transaction/depgraph.go), so a value carrying
	// a prefix length is cut to the same name before the comparison.
	address, _, _ := strings.Cut(localAddress, "/")
	return slices.Contains(disturbed, address)
}

func bgpModifyPeerOperation(name, localAddress string, config, oldConfig json.RawMessage, disturbed []string) configtx.ConfigOperation {
	return configtx.ConfigOperation{
		ID:     "bgp-modify-peer-" + sanitizeBGPOperationID(name),
		Root:   configRootBGP,
		Owner:  pluginNameBGP,
		Type:   configop.ModifyPeer,
		Verb:   configtx.VerbModify,
		Target: peerResource(name, localAddress),
		// A modify keeps the session it changes, so it produces the peer as
		// the create does. It still binds the local address, which is what
		// puts it after that address where one transaction moves the address
		// and changes the peer.
		Produces: []configtx.ResourceRef{{Kind: configtx.ResourcePeer, Peer: name}},
		Consumes: peerAddressConsumes(localAddress, disturbed),
		Params: configtx.ConfigOperationParams{
			Peer:      name,
			Address:   localAddress,
			Config:    config,
			OldConfig: oldConfig,
		},
	}
}

// bgpDiffTouchesPeer reports whether this diff names a peer key, and answers
// the parse error rather than a verdict when a section will not unmarshal.
//
// "No peer key" is what stops this root from emitting any operation, so a
// section read that way would leave every peer change of the commit unapplied,
// spelled exactly as a commit that changes no peer (ai/rules/principles.md).
// The first-party producer marshals a map and cannot emit one
// (buildDiffSections, internal/component/plugin/server/reload_tx.go), so this
// aborts the transaction rather than guessing on behalf of a plugin that sent
// something else.
func bgpDiffTouchesPeer(diff configtx.DiffSection) (bool, error) {
	for _, raw := range []string{diff.Added, diff.Removed, diff.Changed} {
		if raw == "" {
			continue
		}
		var entries map[string]any
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return false, err
		}
		for key := range entries {
			if key == "bgp/peer" || strings.HasPrefix(key, "bgp/peer/") || key == "peer" || strings.HasPrefix(key, "peer/") {
				return true, nil
			}
		}
	}
	return false, nil
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
	slices.Sort(names)
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

func verifyBGPOperation(txID string, op *sdk.ConfigOperation, handle registry.BGPReactorHandle) error {
	adapter, err := bgpOperationAdapter(handle)
	if err != nil {
		return err
	}
	return adapter.VerifyConfigOperation(txID, op)
}

func applyBGPOperation(op *sdk.ConfigOperation, handle registry.BGPReactorHandle, journal registry.ConfigJournal) (*sdk.ConfigOperationApplyOutput, error) {
	adapter, err := bgpOperationAdapter(handle)
	if err != nil {
		return nil, err
	}
	return adapter.ApplyConfigOperation(op, journal)
}

// bgpCaptureReactor is the narrow view of the reactor a config event needs. It
// is a separate interface from bgpOperationReactor so a reactor implementation
// without capture support still satisfies the operation callbacks.
type bgpCaptureReactor interface {
	CapturesOpen() bool
	CaptureConfigEvent(op, txID string, payload []byte)
}

// captureHandleWarnOnce keeps the unreachable-branch warning below to one line
// per process: it fires per config operation, and a repeating warning would bury
// the transaction log it sits in.
var captureHandleWarnOnce sync.Once

// captureBGPConfigEvent records one config-transaction phase into every open
// protocol event capture, so a replayed session carries the config the reactor
// was applying at the time (spec improve-3 AC-6).
//
// The transaction id lives only here, at the plugin callback boundary: the
// reactor's own ApplyConfigOperation is handed the operation without it.
//
// It is best-effort by design. A capture is a diagnostic aid, so a handle that
// does not support capture, or a payload that will not marshal, must never fail
// a config transaction.
func captureBGPConfigEvent(handle registry.BGPReactorHandle, phase, txID string, detail any) {
	rec, ok := handle.(bgpCaptureReactor)
	if !ok {
		// Say it once. This branch is not supposed to be reachable: the factory
		// returns *reactor.Reactor and bgpconfig asserts the method set at
		// compile time (bgp/config/register.go, bgpCaptureHandle). If it ever
		// IS reached, config event capture is dead and silence would be the
		// whole defect, so the log line is the only signal an operator gets.
		captureHandleWarnOnce.Do(func() {
			slogutil.LazyLogger("bgp.capture")().Warn("the BGP reactor handle records no config events; captures will show no config operations")
		})
		return
	}
	if !rec.CapturesOpen() {
		return
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		// Say it, then record the operation without its detail. A config event
		// with no payload also means "this phase carries no detail", so silence
		// here would leave a lost payload and an empty one spelled the same way
		// in the capture, and nothing in the file could tell them apart.
		slogutil.LazyLogger("bgp.capture")().Warn("config detail could not be marshaled; the capture records the operation without it",
			"op", phase, "tx-id", txID, "error", err)
		payload = nil
	}
	rec.CaptureConfigEvent(phase, txID, payload)
}

// captureOperationPhase maps a BGP config operation type onto the capture
// format's operation name, so a capture spells the same operations the reactor
// dispatched.
func captureOperationPhase(opType sdk.ConfigOperationType) string {
	switch opType {
	case configop.AddPeer:
		return capture.OpAddPeer
	case configop.ModifyPeer:
		return capture.OpModifyPeer
	case configop.RemovePeer:
		return capture.OpRemovePeer
	default:
		return string(opType)
	}
}
