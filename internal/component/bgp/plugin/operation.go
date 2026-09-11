// Design: docs/architecture/config/apply-ordering.md -- BGP-owned operation decomposition
// Related: register.go -- SDK operation callback wiring

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
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
	VerifyConfigOperation(*rpc.ConfigOperation) error
	ApplyConfigOperation(*rpc.ConfigOperation, registry.ConfigJournal) (*rpc.ConfigOperationApplyOutput, error)
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
	// A commit that disturbs an address reaches this root with no diff at all,
	// and the sessions bound to that address still have to stop and start
	// (docs/architecture/config/apply-ordering.md, phase 2).
	if !bgpDiffTouchesPeer(req.Diff) && len(req.DisturbedAddresses) == 0 {
		return nil, nil
	}
	disturbed := req.DisturbedAddresses
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
				bgpPeerOperation(configop.RemovePeer, name, activePeer.localAddress, nil, activePeer.raw, disturbed),
				bgpPeerOperation(configop.AddPeer, name, candidatePeer.localAddress, candidatePeer.raw, nil, disturbed),
			)
			continue
		}
		if string(activePeer.raw) == string(candidatePeer.raw) {
			continue
		}
		if activePeer.localAddress != candidatePeer.localAddress {
			ops = append(ops,
				bgpPeerOperation(configop.RemovePeer, name, activePeer.localAddress, nil, activePeer.raw, disturbed),
				bgpPeerOperation(configop.AddPeer, name, candidatePeer.localAddress, candidatePeer.raw, nil, disturbed),
			)
			continue
		}
		sameAddressChanges = append(sameAddressChanges, bgpPeerChange{name: name, active: activePeer, candidate: candidatePeer})
	}
	if peerRouterIDRotation(sameAddressChanges) {
		for _, change := range sameAddressChanges {
			ops = append(ops, bgpPeerOperation(configop.RemovePeer, change.name, change.active.localAddress, nil, change.active.raw, disturbed))
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
