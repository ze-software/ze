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

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/role/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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
	filterLocalASN    uint32                     // local AS number for OTC egress stamping
)

// setFilterState stores peer role configs, name-to-IP mapping, and local ASN for filter closures.
func setFilterState(configs map[string]*peerRoleConfig, n2ip map[string]string, localASN uint32) {
	filterMu.Lock()
	filterPeerConfigs = configs
	filterNameToIP = n2ip
	filterLocalASN = localASN
	filterRemoteRoles = nil // Clear stale remote roles from previous config.
	filterMu.Unlock()
}

// getLocalASN returns the local AS number captured from config.
func getLocalASN() uint32 {
	filterMu.RLock()
	defer filterMu.RUnlock()
	return filterLocalASN
}

// setFilterRemoteRole stores a peer's negotiated remote role for filter closures.
// peerID is the peer name from OnValidateOpen; it is resolved to IP via filterNameToIP.
func setFilterRemoteRole(peerID, remoteRole string) {
	filterMu.Lock()
	if filterRemoteRoles == nil {
		filterRemoteRoles = make(map[string]string)
	}
	// Resolve peer name to IP. If peerID is already an IP (no name mapping), use as-is.
	key := peerID
	if ip, ok := filterNameToIP[peerID]; ok {
		key = ip
	}
	filterRemoteRoles[key] = remoteRole
	filterMu.Unlock()
}

// getFilterConfig returns the role config and remote role for a peer by IP address.
func getFilterConfig(peerIP string) (cfg *peerRoleConfig, remoteRole string) {
	filterMu.RLock()
	defer filterMu.RUnlock()
	return filterPeerConfigs[peerIP], filterRemoteRoles[peerIP]
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
		var localASN uint32
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			peerConfigs, nameToIP = extractPeerRoleConfigs(section.Data)
			caps = append(caps, extractRoleCapabilities(section.Data)...)
			localASN = extractLocalASN(section.Data)
		}
		// Store configs in package-level state for filter closures.
		setFilterState(peerConfigs, nameToIP, localASN)
		p.SetCapabilities(caps)
		return nil
	})

	// RFC 9234 Section 4.2: Validate OPEN pairs for role compatibility.
	// WantsValidateOpen is auto-set by SDK when this callback is registered.
	// Also stores the remote peer's role for ingress/egress filter closures.
	p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
		// Resolve peer name to IP for config lookup (peerConfigs keyed by IP).
		configKey := input.Peer
		if ip, ok := nameToIP[input.Peer]; ok {
			configKey = ip
		}
		cfg := peerConfigs[configKey]
		result := validateOpenRolePair(cfg, input)

		// Store remote role for filter closures (even if validation rejects).
		remoteRoles := extractRolesFromCaps(input.Remote.Capabilities)
		if len(remoteRoles) > 0 {
			if name, ok := roleValueToName(remoteRoles[0]); ok {
				setFilterRemoteRole(input.Peer, name)
			}
		}

		return result
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

// GetYANG returns the embedded YANG schema for the Role plugin.
func GetYANG() string {
	return schema.ZeRoleYANG
}
