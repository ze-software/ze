// Design: docs/architecture/core-design.md — BGP role plugin
// Design: rfc/short/rfc9234.md
//
// Package role implements RFC 9234 BGP Role as a plugin for ze.
// It receives per-peer role config during Stage 2 and registers
// Role capabilities (code 9) per-peer during Stage 3.
//
// RFC 9234: Route Leak Prevention and Detection Using Roles.
package bgp_role

import (
	"context"
	"log/slog"
	"net"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp-role/schema"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
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

// RFC 9234 Section 4.1, Table 1: Role values.
var roleNames = map[uint8]string{
	0: "provider",
	1: "rs",
	2: "rs-client",
	3: "customer",
	4: "peer",
}

// roleValues is the reverse mapping: role name → wire value.
var roleValues = map[string]uint8{
	"provider":  0,
	"rs":        1,
	"rs-client": 2,
	"customer":  3,
	"peer":      4,
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
func RunRolePlugin(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("bgp-role", engineConn, callbackConn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	// Store peer role configs from OnConfigure for validate-open.
	var peerConfigs map[string]*peerRoleConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			peerConfigs = extractPeerRoleConfigs(section.Data)
			caps = append(caps, extractRoleCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)
		return nil
	})

	// RFC 9234 Section 4.2: Validate OPEN pairs for role compatibility.
	// WantsValidateOpen is auto-set by SDK when this callback is registered.
	p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
		cfg := peerConfigs[input.Peer]
		return validateOpenRolePair(cfg, input)
	})

	ctx := context.Background()
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
