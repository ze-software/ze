// Package role implements RFC 9234 BGP Role as a plugin for ze.
// It receives per-peer role config during Stage 2 and registers
// Role capabilities (code 9) per-peer during Stage 3.
//
// RFC 9234: Route Leak Prevention and Detection Using Roles.
package role

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/plugin/role/schema"
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

// peerRoleConfig holds per-peer role configuration.
type peerRoleConfig struct {
	role   string // role name: provider, rs, rs-client, customer, peer
	strict bool   // require peer to send Role capability
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

// RFC 9234 Section 4.2, Table 2: Valid local→peer role pairs.
var validPairs = map[[2]string]bool{
	{"provider", "customer"}: true,
	{"customer", "provider"}: true,
	{"rs", "rs-client"}:      true,
	{"rs-client", "rs"}:      true,
	{"peer", "peer"}:         true,
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

// validRolePair checks if a local/peer role pair is valid per RFC 9234 Table 2.
func validRolePair(local, peer string) bool {
	return validPairs[[2]string{local, peer}]
}

// extractPeerRoleConfigs parses BGP config JSON and returns per-peer role configs.
func extractPeerRoleConfigs(jsonStr string) map[string]*peerRoleConfig {
	var bgpConfig map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &bgpConfig); err != nil {
		logger().Warn("invalid JSON in bgp config", "err", err)
		return nil
	}

	// The config tree is wrapped: {"bgp": {"peer": {...}}}
	bgpSubtree, ok := bgpConfig["bgp"].(map[string]any)
	if !ok {
		bgpSubtree = bgpConfig
	}

	peersMap, ok := bgpSubtree["peer"].(map[string]any)
	if !ok {
		logger().Debug("no peer config in bgp tree")
		return nil
	}

	configs := make(map[string]*peerRoleConfig)

	for peerAddr, peerData := range peersMap {
		peerMap, ok := peerData.(map[string]any)
		if !ok {
			continue
		}

		capMap, ok := peerMap["capability"].(map[string]any)
		if !ok {
			continue
		}

		roleData, ok := capMap["role"].(map[string]any)
		if !ok {
			continue
		}

		roleName, ok := roleData["role"].(string)
		if !ok || roleName == "" {
			continue
		}

		// Validate role name
		if _, valid := roleValues[roleName]; !valid {
			logger().Warn("unknown role name", "peer", peerAddr, "role", roleName)
			continue
		}

		cfg := &peerRoleConfig{role: roleName}

		// Extract strict mode (default false)
		if strict, ok := roleData["role-strict"].(bool); ok {
			cfg.strict = strict
		}

		configs[peerAddr] = cfg
		logger().Debug("role config", "peer", peerAddr, "role", roleName, "strict", cfg.strict)
	}

	return configs
}

// extractRoleCapabilities parses BGP config JSON and returns per-peer Role capabilities.
// RFC 9234 Section 4.1: Role capability code is 9.
func extractRoleCapabilities(jsonStr string) []sdk.CapabilityDecl {
	configs := extractPeerRoleConfigs(jsonStr)
	if len(configs) == 0 {
		return nil
	}

	var caps []sdk.CapabilityDecl
	for peerAddr, cfg := range configs {
		value, ok := roleNameToValue(cfg.role)
		if !ok {
			continue
		}

		// RFC 9234 Section 4.1: capability value is single byte
		caps = append(caps, sdk.CapabilityDecl{
			Code:     roleCapCode,
			Encoding: "hex",
			Payload:  fmt.Sprintf("%02x", value),
			Peers:    []string{peerAddr},
		})
		logger().Debug("role capability", "peer", peerAddr, "role", cfg.role, "value", value)
	}

	return caps
}

// decodeRole decodes a Role capability wire value.
// RFC 9234 Section 4.1: Capability length MUST be 1.
func decodeRole(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty Role capability")
	}
	if len(data) != 1 {
		return "", fmt.Errorf("invalid Role capability length: want 1, got %d", len(data))
	}

	name, ok := roleValueToName(data[0])
	if !ok {
		return fmt.Sprintf("unknown(%d)", data[0]), nil
	}
	return name, nil
}

// writeStr writes a string to a writer, ignoring pipe errors (CLI output).
func writeStr(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...) //nolint:errcheck // CLI output - pipe failure is unrecoverable
}

// RunRolePlugin runs the Role plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunRolePlugin(engineConn, callbackConn net.Conn) int {
	p := sdk.NewWithConn("role", engineConn, callbackConn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			caps = append(caps, extractRoleCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)
		return nil
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

// RunCLIDecode decodes hex capability data directly from CLI arguments.
// This is for human use: `ze bgp plugin role --capa <hex>` or with `--text`.
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	if hexData == "" {
		writeStr(stderr, "error: empty input\n")
		return 1
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeStr(stderr, "error: invalid hex: %v\n", err)
		return 1
	}

	roleName, err := decodeRole(data)
	if err != nil {
		writeStr(stderr, "error: %v\n", err)
		return 1
	}

	if textOutput {
		writeStr(stdout, "%-20s %s\n", "role", roleName)
	} else {
		result := map[string]any{
			"name":  "role",
			"value": roleName,
		}
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			writeStr(stderr, "error: JSON encoding: %v\n", err)
			return 1
		}
		writeStr(stdout, "%s\n", string(jsonBytes))
	}
	return 0
}

// GetYANG returns the embedded YANG schema for the Role plugin.
func GetYANG() string {
	return schema.ZeRoleYANG
}
