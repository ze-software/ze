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

	"github.com/ze-software/ze/internal/component/bgp/plugins/role/yang"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// RFC 9234 Section 4.1: BGP Role Capability (Code 9, Length 1).
const roleCapCode = 9

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

// Package-level filter state. Populated by RunRolePlugin (OnConfigure, OnValidateOpen).
// Read by filter closures registered in register.go. Protected by filterMu.
// All maps are keyed by peer IP address (from remote.ip in config).
var (
	filterMu          sync.RWMutex
	filterPeerConfigs map[string]*peerRoleConfig // IP -> role config (from OnConfigure)
	filterRemoteRoles map[string]string          // IP -> remote role name (from OnValidateOpen)
	filterNameToIP    map[string]string          // peer name -> IP (for OnValidateOpen name resolution)
)

// setFilterState stores peer role configs and name-to-IP mapping for filter closures.
// The local AS used for OTC egress stamping is NOT captured here: the reactor hands
// it per destination peer via filterapi.PeerFilterInfo.LocalAS on the forward path.
//
// Learned remote roles are RETAINED for every peer the new config still names,
// and dropped only for peers it no longer names. A learned role is a property of
// the SESSION, not of our config: it is what the peer put in its OPEN, and a peer
// whose session survives a reconfigure has not withdrawn it. The map is keyed by
// IP, so a peer still present keeps the same key.
//
// It used to be wiped wholesale, and that was not a clean slate but a lie: an
// established peer sends no second OPEN, so nothing rewrote the entry, and
// getFilterConfig then read the absent key as "this peer announced no role". Every
// such peer became roleUnknown for the OTCEgressFilter export-set check, so a
// source configured `role { export customer }` stopped advertising to its
// customers on the next config reload, silently and until each session bounced.
func setFilterState(configs map[string]*peerRoleConfig, n2ip map[string]string) {
	filterMu.Lock()
	filterPeerConfigs = configs
	filterNameToIP = n2ip
	// Drop learned roles for peers the new config no longer names; a role we
	// cannot map back to a configured peer can never be read again anyway.
	for key := range filterRemoteRoles {
		if _, stillConfigured := configs[key]; !stillConfigured {
			delete(filterRemoteRoles, key)
		}
	}
	filterMu.Unlock()
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

// setFilterRemoteRole stores a peer's negotiated remote role for filter closures.
// peerID is the peer name from OnValidateOpen; it is resolved to IP via filterNameToIP.
func setFilterRemoteRole(peerID, remoteRole string) {
	filterMu.Lock()
	if filterRemoteRoles == nil {
		filterRemoteRoles = make(map[string]string)
	}
	filterRemoteRoles[filterKeyLocked(peerID)] = remoteRole
	filterMu.Unlock()
}

// recordNoRemoteRole records that a peer's OPEN declared no usable role, so the
// RFC 9234 Section 5 gates fall back to the configured complement
// (resolvePeerRole) instead of a value the peer is no longer advertising.
// Returns the role that was replaced, or "" if there was none, so the caller can
// report a real transition without a second lookup.
//
// It WRITES the empty string rather than deleting the key, and that is the whole
// point. Deleting made "this peer's OPEN declared no role" share a
// representation with "no OPEN was ever recorded for this peer". The two states
// behave alike at the export set -- Thomas ruled on 2026-08-03 that the
// operator's `unknown` token covers both (otc.go) -- but they have different
// causes, so an operator needs them told apart in the drop reason
// (remoteRoleRecorded). Both resolve to the configured complement for the RFC
// gates, so the Section 5 procedures are unchanged either way.
func recordNoRemoteRole(peerID string) string {
	filterMu.Lock()
	defer filterMu.Unlock()
	if filterRemoteRoles == nil {
		filterRemoteRoles = make(map[string]string)
	}
	key := filterKeyLocked(peerID)
	previous := filterRemoteRoles[key]
	filterRemoteRoles[key] = ""
	return previous
}

// getFilterConfig returns the role config and remote role for a peer by IP address.
func getFilterConfig(peerIP string) (cfg *peerRoleConfig, remoteRole string) {
	filterMu.RLock()
	defer filterMu.RUnlock()
	return filterPeerConfigs[peerIP], filterRemoteRoles[peerIP]
}

// remoteRoleRecorded reports whether this peer's OPEN was ever recorded, which
// is a different question from what it recorded.
//
// getFilterConfig above reads the map bare, so it answers "" for two states that
// are not the same fact (ai/rules/evidence.md):
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
func remoteRoleRecorded(peerIP string) bool {
	filterMu.RLock()
	defer filterMu.RUnlock()
	_, recorded := filterRemoteRoles[peerIP]
	return recorded
}

// Role name constants (RFC 9234 Section 4.1, Table 1).
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

	// Store peer role configs from OnConfigure for validate-open.
	// Both maps shared between OnConfigure and OnValidateOpen closures.
	var peerConfigs map[string]*peerRoleConfig
	var nameToIP map[string]string

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			peerConfigs, nameToIP = extractPeerRoleConfigs(section.Data)
			caps = append(caps, extractRoleCapabilities(section.Data)...)
		}
		// Store configs in package-level state for filter closures. The local AS
		// for OTC egress stamping is supplied per destination peer by the reactor
		// (filterapi.PeerFilterInfo.LocalAS), not parsed from the config here.
		setFilterState(peerConfigs, nameToIP)
		p.SetCapabilities(caps)
		return nil
	})

	// RFC 9234 Section 4.2: Validate OPEN pairs for role compatibility.
	// WantsValidateOpen is auto-set by SDK when this callback is registered.
	// Also stores the remote peer's role for ingress/egress filter closures.
	p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
		return applyValidateOpen(peerConfigs, nameToIP, input)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
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
// against a relationship it had stopped claiming. The only clear was the
// wholesale wipe in setFilterState on reconfigure.
//
// The OPEN is the ordering-safe place to do this, and a session-down handler is
// not. A down clear would have to race this write from a different goroutine:
// for an in-process plugin the structured state event is delivered on the
// process deliveryLoop while validate-open arrives on the bridge callback loop,
// and the state event carries no session identity or sequence number to order
// the two by (internal/component/bgp/server/events.go:101-114 sets PeerAddress,
// PeerName, State and Reason, and never a MessageID). A late down could
// therefore delete a role that a newer session had already written. Clearing
// here is driven by the very event that sets it, for the same session, so the
// last OPEN always decides.
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
	result := validateOpenRolePair(cfg, input)

	// What role, if any, this OPEN declares. An unassigned value (RFC 9234
	// Table 1 leaves 5-255 unassigned) is not a role we can act on, so it
	// clears rather than preserving the previous session's value.
	learned := ""
	if roles := extractRolesFromCaps(input.Remote.Capabilities); len(roles) > 0 {
		learned, _ = roleValueToName(roles[0])
	}

	if learned != "" {
		setFilterRemoteRole(input.Peer, learned)
		return result
	}

	// Report only a real transition: a peer that has never advertised a role
	// must not log on every reconnect.
	if previous := recordNoRemoteRole(input.Peer); previous != "" {
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
