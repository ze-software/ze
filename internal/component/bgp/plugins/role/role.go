// Design: docs/architecture/core-design.md — BGP role plugin
// RFC: rfc/short/rfc9234.md
// Detail: otc.go — OTC attribute processing (ingress/egress)
// Detail: config.go — per-peer role config parsing (import/export)
//
// Package role implements RFC 9234 BGP Role as a plugin for ze.
// It receives per-peer role config during Stage 2 and registers
// Role capabilities (code 9) per-peer during Stage 3.
//
// RFC 9234: Route Leak Prevention and Detection Using Roles.
package role

import (
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/policy"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/plugins/role/yang"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// RFC 9234 Section 4.1: BGP Role Capability (Code 9, Length 1).
const roleCapCode = 9

// roleCapabilities computes role's declaration from the bgp root, and
// roleCapabilityLeaves names the leaf that sets each code. RunRolePlugin
// registers both; the tests drive the same pair through the SDK's comparison.
var (
	roleCapabilities = configjson.BGPCapabilityExtractor(func(data string) ([]sdk.CapabilityDecl, error) {
		return extractRoleCapabilities(data), nil
	})
	roleCapabilityLeaves = map[uint8]string{roleCapCode: roleKeyword}
)

// roleKeyword is the word that names the RFC 9234 role everywhere it is spelled:
// the YANG container a peer or group declares it in, the capability leaf, and
// the capability's name in decode output.
const roleKeyword = "role"

const configRootBGP = "bgp"

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// ConfigureLogger sets the package-level logger.
func ConfigureLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// Package-level filter state. Populated by the config transaction (config_tx.go) and OnValidateOpen.
// Read by filter closures registered in register.go. Protected by filterMu.
//
// A configured peer is keyed by its IP address (from remote.ip in config). A
// dynamic group's template is keyed by configjson.CapabilityGroupKey, which is
// the group's name behind a "group:" prefix. The peers built from that template
// carry the group's name and no address the config document holds. No address
// can carry that prefix, so the two namespaces cannot collide.
var (
	filterMu          sync.RWMutex
	filterPeerConfigs map[string]*peerRoleConfig // IP or group selector -> role config (from the config transaction)
	filterRemoteRoles map[string]remoteRoleState // filter key -> what the peer's OPEN declared (from OnValidateOpen)
	filterNameToIP    map[string]string          // peer name -> IP (for OnValidateOpen name resolution)
	filterPeerKeys    map[string]struct{}        // every configured peer's name and address, every group selector (installed)
	prunedPeerKeys    map[string]struct{}        // filterPeerKeys as the last pruneLearnedRoles saw it
)

// remoteRoleState is what one peer's OPEN declared about its role, plus the
// group that peer belongs to.
//
// group is empty for a standalone peer and carries the enclosing group's name
// for a grouped one: a static peer the group lists, or a peer created from a
// dynamic group's template. pruneLearnedRoles reads it only for an entry whose
// own key no config names, which is how a dynamic member is stored: its learned
// role then lives as long as its group stays dynamic.
//
// opened is the generation the engine stamped on the OPEN that stored the
// entry (rpc.ValidateOpenInput.Generation). A removal frees the entry only
// when the generation its down names is above it (forgetRemovedPeer).
type remoteRoleState struct {
	role   string
	group  string
	opened uint64
}

// pruneLearnedRoles drops every learned role whose peer the installed config
// no longer names (learnedRolePeerConfiguredLocked). A reload runs it at
// config-committed only (roleConfigTx): the learned roles are runtime state no
// rollback could restore.
//
// It reads the configured-peer set (configuredPeerKeys), never the role
// configs. A dynamic group's member is stored under its own name, which no
// role config keys: only its group's selector says whether it is still
// configured (learnedRolePeerConfiguredLocked).
//
// The other path is forgetRemovedPeer, which the state down with reason
// removed reaches (removal.go); this commit-time prune covers a commit's
// removal even when that down is still queued.
func pruneLearnedRoles() {
	filterMu.Lock()
	defer filterMu.Unlock()
	for key, state := range filterRemoteRoles {
		if !learnedRolePeerConfiguredLocked(key, state) {
			delete(filterRemoteRoles, key)
		}
	}
	prunedPeerKeys = filterPeerKeys
}

// learnedRolePeerConfiguredLocked reports whether the installed config still
// names the peer a learned role was stored for under key. Caller MUST hold
// filterMu.
//
// A key the installed config names is its peer. A key the last prune saw
// configured and the installed config does not is a peer the commit removed,
// whatever its group: a static peer a group lists carries that group too, and
// a commit that removes the group's listen range removes the group's template
// selector while the static peer stays.
//
// Any other key was never a configured peer's. With a group, it is a dynamic
// member's, so the answer is whether its group is still a configured dynamic
// group. Without one, it is kept, since no commit is evidence that its peer
// is gone; applyValidateOpen stores no role for a peer without role config
// (a peer made at runtime with `create bgp peer <addr>` has none), so such a
// key is one an OPEN validated after its peer's removal already ran
// (docs/architecture/meta/role.md, "Learned role lifecycle").
func learnedRolePeerConfiguredLocked(key string, state remoteRoleState) bool {
	if _, configured := filterPeerKeys[key]; configured {
		return true
	}
	if _, wasConfigured := prunedPeerKeys[key]; wasConfigured {
		return false
	}
	if state.group == "" {
		return true
	}
	_, groupConfigured := filterPeerKeys[configjson.CapabilityGroupKey(state.group)]
	return groupConfigured
}

// forgetRemovedPeer drops the learned role of a peer the reactor removed. The
// state down with reason removed calls it (removal.go), on either transport,
// for every peer whose filter lists carry a bgp-role member, which are the
// only peers a role is stored for (applyValidateOpen).
//
// It clears the key the peer's name maps to (filterKeyLocked, the key every
// OPEN of that peer wrote) and the peer's address, which keys a role an OPEN
// stored under the address itself.
//
// removed is the session generation the down names. The reactor issues it
// when it deletes the peer (doRemovePeer), so it is above the generation of
// every OPEN the removed peer validated, and at or below the generation of
// every OPEN a peer created again under the same name validates
// (rpc.ValidateOpenInput.Generation). The down reaches the plugin on the event
// loop and validate-open on the callback loop, so a down still queued when
// the new peer's OPEN is stored would otherwise delete the new peer's role. A
// down that names no generation (zero) predates that contract: it frees the
// entry as the removal it reports, with a warning. Safe for concurrent use.
func forgetRemovedPeer(peer filterapi.PeerFilterInfo, removed uint64) {
	if removed == 0 {
		logger().Warn("bgp-role: a peer removal names no session generation, its learned role is freed even if a newer session stored it",
			"peer", peer.Name, "address", peer.Address)
	}
	filterMu.Lock()
	defer filterMu.Unlock()
	if peer.Name != "" {
		forgetLearnedRoleLocked(filterKeyLocked(peer.Name), peer.Name, removed)
	}
	if peer.Address.IsValid() {
		forgetLearnedRoleLocked(peer.Address.String(), peer.Name, removed)
	}
}

// forgetLearnedRoleLocked deletes the learned role under key, unless an OPEN
// validated after the removal stored it (forgetRemovedPeer), and logs what the
// removal dropped. Caller MUST hold filterMu.
func forgetLearnedRoleLocked(key, peerName string, removed uint64) {
	state, recorded := filterRemoteRoles[key]
	if !recorded {
		return
	}
	if removed != 0 && state.opened >= removed {
		logger().Info("learned role kept: a session newer than the removal stored it",
			"peer", peerName, "key", key, "role", state.role,
			"opened-generation", state.opened, "removed-generation", removed)
		return
	}
	delete(filterRemoteRoles, key)
	logger().Info("learned role dropped: peer removed",
		"peer", peerName, "key", key, "previous-role", state.role)
}

// filterKeyLocked maps an OnValidateOpen peer id (the peer NAME) to the key the
// filter maps use (the peer ADDRESS). If peerID is already an address, or no
// mapping exists, it is used as-is.
//
// Both the setter and the clearer go through here so they can never disagree
// about which key they touch: a clear that skipped the name -> IP translation
// would delete an unreachable key and leave the live one in place.
//
// Caller must hold filterMu.
func filterKeyLocked(peerID string) string {
	if ip, ok := filterNameToIP[peerID]; ok {
		return ip
	}
	return peerID
}

// recordLearnedRole stores what a peer's OPEN declared about its role for the
// filter closures, and returns the role it replaced, or "" if there was none,
// so the caller can report a real transition without a second lookup.
//
// peerID is the peer name from OnValidateOpen; it is resolved to IP via
// filterNameToIP. group is the peer's enclosing group, empty for a peer that
// stands alone. It lets pruneLearnedRoles tell a live dynamic member from a
// peer the config dropped (remoteRoleState). opened is the OPEN's
// rpc.ValidateOpenInput.Generation: forgetRemovedPeer frees the entry only
// for a removal issued after it.
//
// An OPEN with no usable role WRITES the empty string rather than deleting the
// key, and that is the whole point. Deleting made "this peer's OPEN declared
// no role" share a representation with "no OPEN was ever recorded for this
// peer". The two states behave alike at the export set -- Thomas ruled on
// 2026-08-03 that the operator's `unknown` token covers both (otc.go) -- but
// they have different causes, so an operator needs them told apart in the drop
// reason (remoteRoleRecorded). Both resolve to the configured complement for
// the RFC gates (resolvePeerRole), so the Section 5 procedures are unchanged
// either way. Safe for concurrent use.
func recordLearnedRole(peerID, group, remoteRole string, opened uint64) string {
	filterMu.Lock()
	defer filterMu.Unlock()
	if filterRemoteRoles == nil {
		filterRemoteRoles = make(map[string]remoteRoleState)
	}
	key := filterKeyLocked(peerID)
	previous := filterRemoteRoles[key].role
	filterRemoteRoles[key] = remoteRoleState{role: remoteRole, group: group, opened: opened}
	return previous
}

// getFilterConfig returns the role config and the learned remote role for one
// peer, identified by the three names every filter decision already carries
// (filterapi.PeerFilterInfo: Address, Name, GroupName).
//
// The config is resolved by ADDRESS first and by the peer's GROUP second. That
// is the precedence configjson.LookupPeerConfig defines: what a peer states
// beats what its group states. A configured peer already carries its group's
// statement, merged into its own entry at parse time, so the fallback answers
// only for a peer the config document does not hold. That peer is a dynamic
// group's member, created from the template when its connection arrives, so no
// address and no name of its own can key it. Before the fallback existed, cfg
// was nil for every such peer, and each RFC 9234 Section 5 gate below took its
// permissive branch.
//
// This is one lookup mechanism, not a second one beside configjson's. It is
// spelled out here because role's map is keyed by string rather than by
// configjson.PeerConfigKey. The same map IS the capability selector index
// extractRoleCapabilities publishes (config.go). The group's key therefore
// carries configjson.CapabilityGroupKey's prefix, which no address can carry.
func getFilterConfig(addr, name, group string) (cfg *peerRoleConfig, remoteRole string) {
	filterMu.RLock()
	defer filterMu.RUnlock()
	cfg = filterPeerConfigs[addr]
	if cfg == nil && group != "" {
		cfg = filterPeerConfigs[configjson.CapabilityGroupKey(group)]
	}
	state, _ := remoteRoleLocked(addr, name)
	return cfg, state.role
}

// remoteRoleLocked resolves one peer's learned-role entry from the two keys its
// writer can have used, and reports whether an entry was found.
//
// filterKeyLocked keys the entry by the peer's ADDRESS when the config names
// that peer, and by the peer NAME itself when it does not. A peer built from a
// dynamic group's template is always the second case. Reactor's
// buildDynamicPeerSettings names it "dyn-<addr>", and no config document holds
// that name, so filterNameToIP can never translate it. A reader of the address
// alone therefore cannot see what such a peer declared in its OPEN.
//
// The address is tried first, so a configured peer resolves exactly as it did
// before the name was consulted at all.
//
// The bare name is tried last, for a peer a reload added. Its session can
// validate an OPEN before the role plugin installs the config that names it,
// and filterKeyLocked then has no translation, so the OPEN is stored under the
// name. Once the config is installed the name translates to the address, which
// holds no entry, and the peer's role read as unknown until its session
// bounced. A later OPEN writes the address key, which is read first.
//
// Caller must hold filterMu.
func remoteRoleLocked(addr, name string) (remoteRoleState, bool) {
	if state, ok := filterRemoteRoles[addr]; ok {
		return state, true
	}
	if name == "" {
		return remoteRoleState{}, false
	}
	if name == addr {
		return remoteRoleState{}, false
	}
	if state, ok := filterRemoteRoles[filterKeyLocked(name)]; ok {
		return state, true
	}
	if state, ok := filterRemoteRoles[name]; ok {
		return state, true
	}
	return remoteRoleState{}, false
}

// remoteRoleRecorded reports whether this peer's OPEN was ever recorded, which
// is a different question from what it recorded.
//
// getFilterConfig above returns the entry's role, so it answers "" for two
// states that are not the same fact (ai/rules/evidence.md):
//
//   - the key is present and empty: this peer's OPEN was validated and declared
//     no usable role.
//   - the key is absent: no OPEN was ever recorded, and it is
//     reachable without any peer misbehaving. broadcastValidateOpen
//     (internal/component/bgp/server/validate.go) skips a plugin when the process
//     manager is nil, when the plugin conn is nil, and when the validate-open RPC
//     returns an error -- and lets the session establish in all three.
//
// Only the export-set branch of OTCEgressFilter needs them apart, and only on
// the suppression path, so this stays a separate question rather than widening
// the reader every caller uses. Since Thomas's 2026-08-03 ruling the two states
// take the SAME export-set decision (`unknown` covers both); what this reader
// changes is the drop REASON the operator sees, never the verdict. Every RFC
// 9234 Section 5 gate resolves through
// resolvePeerRole, which takes the configured complement for either state and
// so cannot tell them apart by design.
// It takes the peer's address and name for the reason remoteRoleLocked gives:
// a dynamic group's member is recorded under its name, so an address-only reader
// would report every such peer as never validated.
func remoteRoleRecorded(addr, name string) bool {
	filterMu.RLock()
	defer filterMu.RUnlock()
	_, recorded := remoteRoleLocked(addr, name)
	return recorded
}

// Role name constants (RFC 9234 Section 4.1, Table 1).
// enumeration: gated by TestRoleNamesMatchTheYANGModel
const (
	roleProvider = "provider"
	roleRS       = "rs"
	roleRSClient = "rs-client"
	roleCustomer = "customer"
	rolePeer     = "peer"
	roleUnknown  = "unknown" // pseudo-role: peers with no role configured
)

// RFC 9234 Section 4.1, Table 1: Role values.
var roleNames = map[uint8]string{
	0: roleProvider,
	1: roleRS,
	2: roleRSClient,
	3: roleCustomer,
	4: rolePeer,
}

// roleValues is the reverse mapping: role name → wire value.
var roleValues = map[string]uint8{
	roleProvider: 0,
	roleRS:       1,
	roleRSClient: 2,
	roleCustomer: 3,
	rolePeer:     4,
}

// roleNameToValue maps a role name to its RFC 9234 wire value.
func roleNameToValue(name string) (uint8, bool) {
	v, ok := roleValues[name]
	return v, ok
}

// roleValueToName maps an RFC 9234 wire value to a role name.
func roleValueToName(value uint8) (string, bool) {
	name, ok := roleNames[value]
	return name, ok
}

// RunRolePlugin runs the Role plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRolePlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-role", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	// The role config: installed at Stage 2 and at every reload that changes it,
	// restored at rollback (config_tx.go). The local AS for OTC egress stamping
	// is supplied per destination peer by the reactor
	// (filterapi.PeerFilterInfo.LocalAS), not parsed from the config.
	newRoleConfigTx().register(p)

	// The role capability declaration is fixed for the process life: a reload
	// that changes it is refused at config-verify
	// (sdk.Plugin.DeclareCapabilitiesFrom).
	p.DeclareCapabilitiesFrom(roleCapabilities, roleCapabilityLeaves)

	// RFC 9234 Section 4.2: Validate OPEN pairs for role compatibility.
	// WantsValidateOpen is auto-set by SDK when this callback is registered.
	// Also stores the remote peer's role for ingress/egress filter closures.
	p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
		installed := installedRoleConfig()
		return applyValidateOpen(installed.peers, installed.nameToIP, input)
	})

	// The state down with reason removed frees a removed peer's learned role
	// (removal.go). The implicit state grant delivers it for every peer whose
	// filter lists carry a bgp-role member, on either transport.
	p.SetStartupSubscriptions([]string{"state"}, nil, "full")
	p.OnStructuredEvent(handleRoleStructuredEvents)
	p.OnEvent(handleRoleEvent)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	// RFC 9234 Section 5 (otc_filter.go) and RFC 7454 Section 9
	// (upstream.go): the filters the loader inserts on role peers, one
	// evaluator on both transports.
	p.OnFilter(evaluateRoleFilter)

	err := p.Run(ctx, &sdk.Registration{
		WantsConfig:      []string{configRootBGP, policy.RootName},
		Filters:          roleFilterDecls(),
		FilterInsertions: roleInsertions(),
	})
	if err != nil {
		logger().Error("role plugin failed", "error", err)
		return 1
	}

	return 0
}

// applyValidateOpen runs the RFC 9234 Section 4.2 OPEN validation for one peer
// and re-establishes what that peer IS to us for the Section 5 filters.
//
// Every OPEN re-establishes the learned role, INCLUDING declaring its absence.
// The previous form only wrote when the peer sent a usable Role capability, so
// a peer that once advertised a role and later reconnected without one kept the
// stale value indefinitely -- and resolvePeerRole (otc.go) PREFERS the learned
// capability over the configured complement, so the peer went on being gated
// against a relationship it had stopped claiming. The only clear was then a
// wholesale wipe of every learned role on reconfigure.
//
// The OPEN is the ordering-safe place to do this, and a session-down handler is
// not. A down clear races this write from a different goroutine: for an
// in-process plugin the structured state event is delivered on the process
// deliveryLoop while validate-open arrives on the bridge callback loop. Only
// the peer's removal clears from a down, and it orders itself against this
// write by generation: the entry carries input.Generation, and the removal
// frees it only when the down names a newer generation (forgetRemovedPeer).
// Every other change is driven by the very event that sets it, for the same
// session, so the last OPEN always decides.
//
// The clear is deliberately symmetric with the set on the reject path too: the
// role is recorded even when validateOpenRolePair refuses the session, so it
// must be dropped on refusal as well, rather than leaving a stale relationship
// exactly when the relationship is most in doubt.
func applyValidateOpen(
	peerConfigs map[string]*peerRoleConfig,
	nameToIP map[string]string,
	input *sdk.ValidateOpenInput,
) *sdk.ValidateOpenOutput {
	// Resolve peer name to IP for config lookup (peerConfigs keyed by IP).
	configKey := input.Peer
	if ip, ok := nameToIP[input.Peer]; ok {
		configKey = ip
	}
	cfg := peerConfigs[configKey]
	if cfg == nil && input.Group != "" {
		// A peer created from a dynamic group's template carries neither a name
		// nor an address the config document holds. The group is the only key
		// that answers for it. Without this, cfg was nil for every such peer and
		// validateOpenRolePair accepted its OPEN unconditionally.
		//
		// RFC 9234 Section 4.2 requires the Roles to correspond to Table 2, and
		// requires the connection to be rejected with the Role Mismatch
		// NOTIFICATION when they do not. Ze advertises the group's Role
		// capability to this peer, so the check binds it as it binds a
		// configured peer.
		//
		// The peer's own key is tried first, so a peer that states its own role
		// still beats its group's.
		cfg = peerConfigs[configjson.CapabilityGroupKey(input.Group)]
	}
	result := validateOpenRolePair(cfg, input)

	// A learned role is stored only for a peer whose config sets role import:
	// that is the peer whose filter lists carry the bgp-role members, the only
	// readers of a learned role, and the peer whose state the implicit state
	// grant delivers, so its removal frees the role on either transport
	// (removal.go). RFC 9234 Section 4.2 makes the configured role the input of
	// the Section 5 procedures ("The locally configured BGP Role is used for
	// the procedures described in Section 5."), so a peer with no role config
	// has no procedure that reads what its OPEN declared.
	if cfg == nil {
		return result
	}

	// What role, if any, this OPEN declares. An unassigned value (RFC 9234
	// Table 1 leaves 5-255 unassigned) is not a role we can act on, so it
	// clears rather than preserving the previous session's value.
	learned := ""
	if roles := extractRolesFromCaps(input.Remote.Capabilities); len(roles) > 0 {
		learned, _ = roleValueToName(roles[0])
	}

	previous := recordLearnedRole(input.Peer, input.Group, learned, input.Generation)
	if learned != "" {
		return result
	}

	// Report only a real transition: a peer that has never advertised a role
	// must not log on every reconnect.
	if previous != "" {
		logger().Info("role capability withdrawn: peer reconnected without the role it previously advertised",
			"peer", input.Peer, "previous-role", previous,
			"effect", "RFC 9234 Section 5 gates now use the configured role complement for this peer")
	}
	return result
}

// GetYANG returns the embedded YANG for the Role plugin.
func GetYANG() string {
	return yang.ZeRoleYANG
}
