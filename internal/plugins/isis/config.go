// Design: plan/learned/930-isis-4-component-config.md -- IS-IS config resolution
// Related: yang/ze-isis-conf.yang -- the schema this resolves from
//
// Config flows file -> YANG schema -> validated tree -> the SDK delivers the
// `isis` subtree as a root-wrapped JSON ConfigSection ({"isis": {...}}). Every
// leaf is rendered as a JSON string by Tree.ToMap (so "10", not 10), keyed lists
// (the `interface` list nested under the `interfaces` container, and key-chains)
// render as a key->entry map, and a single-element leaf-list (net) renders as a
// bare scalar while a multi-element one renders as a []any. The interface list is
// wrapped in an `interfaces` container, mirroring the OSPF config shape
// (ze-ospf-conf.yang). This file parses that shape into typed Go structs, applies
// the YANG defaults, and validates the required fields (at least one NET; a
// derivable System ID).

package isis

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// Level is the configured routing level of the IS or a circuit.
type Level uint8

// Routing level values (ISO/IEC 10589). LevelL1L2 is the default.
const (
	LevelL1 Level = iota
	LevelL2
	LevelL1L2
)

// String renders the level as the YANG enum token.
func (l Level) String() string {
	switch l {
	case LevelL1:
		return "l1"
	case LevelL2:
		return "l2"
	default:
		return "l1-l2"
	}
}

// HasL1 reports whether the level includes Level-1.
func (l Level) HasL1() bool { return l == LevelL1 || l == LevelL1L2 }

// HasL2 reports whether the level includes Level-2.
func (l Level) HasL2() bool { return l == LevelL2 || l == LevelL1L2 }

// TransportLevel maps the configured level to the transport's enable level.
// A dual-level circuit (l1-l2) enables both; the transport selects per-level
// multicast groups on send.
func (l Level) TransportLevel() transport.Level {
	switch l {
	case LevelL1:
		return transport.Level1
	case LevelL2:
		return transport.Level2
	default:
		// l1-l2: open at Level1; the engine reaches the L2 group with
		// SendPDUBothLevels (transport.SendPDUBothLevels).
		return transport.Level1
	}
}

// CircuitType is the per-interface circuit medium.
type CircuitType uint8

// Circuit type values.
const (
	CircuitBroadcast CircuitType = iota
	CircuitPointToPoint
)

// YANG enum tokens for CircuitType: the strings used in ze-isis-conf.yang,
// rendered by String() and matched by the config resolver.
const (
	circuitTypeBroadcast = "broadcast"
	circuitTypeP2P       = "point-to-point"
)

// String renders the circuit type as the YANG enum token.
func (c CircuitType) String() string {
	if c == CircuitPointToPoint {
		return circuitTypeP2P
	}
	return circuitTypeBroadcast
}

// YANG defaults (mirrored from ze-isis-conf.yang; the single source of the
// numbers is the YANG, these constants keep the Go resolver self-contained and
// are asserted equal to the schema defaults by TestISISConfigDefaults).
const (
	DefaultLevel              = LevelL1L2
	DefaultLSPLifetime        = 1200
	DefaultLSPRefreshInterval = 900
	DefaultMetric             = 10
	DefaultHelloInterval      = 10
	DefaultHoldMultiplier     = 3
	DefaultPriority           = 64
	DefaultCircuitType        = CircuitBroadcast
)

// MaxWideMetric is the wide-metric bound (RFC 5305 section 3.0: the metric is a
// 24-bit unsigned value 0..16777215; the per-interface circuit metric leaf is
// bounded 1..MaxWideMetric).
const MaxWideMetric = 16777215

// LevelInterfaceConfig holds the per-level metric/hello/hold/priority overrides
// for a circuit. A zero field means "no override; use the circuit-wide value".
type LevelInterfaceConfig struct {
	Metric        uint32
	HelloInterval uint16
	HoldMult      uint8
	Priority      uint8
	AuthKeyChain  string
}

// InterfaceConfig is one resolved per-interface IS-IS circuit configuration.
type InterfaceConfig struct {
	Name          string
	Enabled       bool
	Passive       bool
	CircuitType   CircuitType
	Level         Level
	Metric        uint32
	HelloInterval uint16
	HoldMult      uint8
	Priority      uint8
	Level1        LevelInterfaceConfig
	Level2        LevelInterfaceConfig
	AddressFamily []string // af enum tokens (ipv4-unicast / ipv6-unicast)
}

// KeyConfig is one key in a key chain (parsed and stored; verify/sign is isis-10).
// SendStart/SendEnd and AcceptStart/AcceptEnd are the optional RFC3339 hitless-
// rotation lifetimes from the YANG send-lifetime / accept-lifetime containers
// (empty when unset); isis-10 interprets them as the signing and accept-on-
// receive windows.
type KeyConfig struct {
	KeyID       uint16
	Algorithm   string
	Secret      string //nolint:gosec // G117: config field name, not a literal; masked via ze:sensitive in YANG and never logged
	SendStart   string
	SendEnd     string
	AcceptStart string
	AcceptEnd   string
}

// KeyChainConfig is a named authentication key chain.
type KeyChainConfig struct {
	Name string
	Keys []KeyConfig
}

// Config is the fully-resolved typed IS-IS configuration.
type Config struct {
	NETs               []types.NET
	SystemID           types.SystemID
	systemIDFromConfig bool
	Level              Level
	LSPLifetime        uint16
	LSPRefreshInterval uint16
	Overload           bool
	Hostname           string
	Interfaces         []InterfaceConfig
	KeyChains          []KeyChainConfig
	Level1AuthKeyChain string
	Level2AuthKeyChain string
}

// Present reports whether a meaningful IS-IS config was delivered (at least one
// NET). A config with no NET leaves the engine idle (like LDP with no lsr-id).
func (c Config) Present() bool { return len(c.NETs) > 0 }

// EnabledCircuits returns the interfaces that should have a circuit opened: those
// enabled and non-passive. Passive interfaces are advertised but form no
// adjacency (so isis-4 opens no circuit for them).
func (c Config) EnabledCircuits() []InterfaceConfig {
	out := make([]InterfaceConfig, 0, len(c.Interfaces))
	for _, ic := range c.Interfaces {
		if ic.Enabled && !ic.Passive {
			out = append(out, ic)
		}
	}
	return out
}

// Errors surfaced by config resolution.
var (
	// ErrNoNET reports a config with no `net` leaf (AC-3).
	ErrNoNET = errors.New("isis: at least one net is required")
	// ErrSystemIDMismatch reports an explicit system-id that does not match the
	// System ID derivable from the first NET.
	ErrSystemIDMismatch = errors.New("isis: system-id does not match the system id derived from net")
)

// configUint8, configUint16 and configUint32 read a config-tree scalar and
// narrow it, returning false when the value does not fit the target type.
//
// The bound belongs here, next to the narrowing, even though the config file
// parser already rejects anything outside the leaf's declared YANG type range
// (ValidateLeafValue, internal/component/config/schema.go:787-805, reached from
// parser.go:266). Relying on a guard three layers up means this code fails OPEN
// for any future entry point that delivers a tree without that validation, and a
// bare uintN(v) would then store a silently truncated value rather than reject
// it (ai/rules/evidence.md, ai/rules/protocol.md).
//
// The bound is the target type's own maximum, not a per-leaf maximum: every YANG
// leaf here declares the same width as the Go field it feeds, so no config the
// parser accepts can be refused by these helpers.
func configUint8(v any) (uint8, bool) {
	n, ok := configNumber(v)
	if !ok || n > math.MaxUint8 {
		return 0, false
	}
	return uint8(n), true
}

func configUint16(v any) (uint16, bool) {
	n, ok := configNumber(v)
	if !ok || n > math.MaxUint16 {
		return 0, false
	}
	return uint16(n), true
}

func configUint32(v any) (uint32, bool) {
	n, ok := configNumber(v)
	if !ok || n > math.MaxUint32 {
		return 0, false
	}
	return uint32(n), true
}

// configNumber coerces a config-tree scalar to a uint64. Tree.ToMap renders
// every numeric YANG leaf as a JSON string (e.g. "10"); accept a JSON number too
// for robustness.
func configNumber(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case string:
		u, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return u, true
	default:
		return 0, false
	}
}

// configBool coerces a config-tree scalar to a bool. Tree.ToMap renders a
// boolean leaf as the string "true"/"false".
func configBool(v any, def bool) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch b {
		case "true":
			return true
		case "false":
			return false
		}
	}
	return def
}

// configString reads a string leaf, returning "" when absent or not a string.
func configString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// configLeafList coerces a YANG leaf-list value into a string slice. A single
// element renders as a bare scalar, several as a []any.
func configLeafList(v any) []string {
	switch list := v.(type) {
	case string:
		if list == "" {
			return nil
		}
		return []string{list}
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// listEntry is one entry of a keyed YANG list (key + child map), used for the
// interface and key-chains lists.
type listEntry struct {
	key  string
	data map[string]any
}

// keyedList coerces a YANG list value (rendered by Tree.ToMap as a key->entry
// map) into its entries, ordered by the key. When numericKey is set the keys are
// compared numerically (key-id) rather than lexically.
func keyedList(v any, numericKey bool) []listEntry {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	entries := make([]listEntry, 0, len(m))
	for key, raw := range m {
		em, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, listEntry{key: key, data: em})
	}
	if numericKey {
		sort.Slice(entries, func(i, j int) bool {
			ai, _ := strconv.Atoi(entries[i].key)
			bj, _ := strconv.Atoi(entries[j].key)
			return ai < bj
		})
	} else {
		sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	}
	return entries
}

// parseLevel maps the YANG enum token to a Level, defaulting to l1-l2.
func parseLevel(v any) Level {
	switch configString(v) {
	case "l1":
		return LevelL1
	case "l2":
		return LevelL2
	default:
		return LevelL1L2
	}
}

// parseISISConfig parses the delivered `isis` ConfigSection JSON into a typed
// Config, applying YANG defaults. It does NOT enforce required-field policy
// (ErrNoNET); that is validateConfig's job, so OnConfigure can stage a partial
// config the same way OnConfigVerify rejects it.
func parseISISConfig(sections []configSection) (Config, error) {
	cfg := Config{
		Level:              DefaultLevel,
		LSPLifetime:        DefaultLSPLifetime,
		LSPRefreshInterval: DefaultLSPRefreshInterval,
	}
	for _, s := range sections {
		if s.Root != "isis" || s.Data == "" {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(s.Data), &wrapper); err != nil {
			return cfg, fmt.Errorf("isis: invalid config JSON: %w", err)
		}
		tree, _ := wrapper["isis"].(map[string]any)
		if tree == nil {
			continue
		}
		if err := applyTree(&cfg, tree); err != nil {
			return cfg, err
		}
	}
	// Derive the System ID from the first NET when no explicit system-id leaf was
	// given (AC-9). ISO/IEC 10589 section 6.2: the System ID is the 6 octets
	// before the 1-octet NSEL.
	if !cfg.systemIDFromConfig && len(cfg.NETs) > 0 {
		cfg.SystemID = cfg.NETs[0].SystemID()
	}
	return cfg, nil
}

// applyTree reads the unwrapped `isis` tree into cfg.
func applyTree(cfg *Config, tree map[string]any) error {
	for _, s := range configLeafList(tree["net"]) {
		net, err := types.ParseNET(s)
		if err != nil {
			return fmt.Errorf("isis: invalid net %q: %w", s, err)
		}
		cfg.NETs = append(cfg.NETs, net)
	}
	if s := configString(tree["system-id"]); s != "" {
		sid, err := types.ParseSystemID(s)
		if err != nil {
			return fmt.Errorf("isis: invalid system-id %q: %w", s, err)
		}
		cfg.SystemID = sid
		cfg.systemIDFromConfig = true
	}
	cfg.Level = parseLevel(tree["level"])
	if v, ok := configUint16(tree["lsp-lifetime"]); ok && v > 0 {
		cfg.LSPLifetime = v
	}
	if v, ok := configUint16(tree["lsp-refresh-interval"]); ok && v > 0 {
		cfg.LSPRefreshInterval = v
	}
	cfg.Overload = configBool(tree["overload"], false)
	cfg.Hostname = configString(tree["hostname"])

	if interfaces, ok := tree["interfaces"].(map[string]any); ok {
		for _, entry := range keyedList(interfaces["interface"], false) {
			cfg.Interfaces = append(cfg.Interfaces, parseInterface(entry))
		}
	}
	for _, entry := range keyedList(tree["key-chains"], false) {
		cfg.KeyChains = append(cfg.KeyChains, parseKeyChain(entry))
	}
	if l1, ok := tree["level-1"].(map[string]any); ok {
		cfg.Level1AuthKeyChain = configString(l1["auth-key-chain"])
	}
	if l2, ok := tree["level-2"].(map[string]any); ok {
		cfg.Level2AuthKeyChain = configString(l2["auth-key-chain"])
	}
	return nil
}

// parseInterface resolves one interface{} entry with YANG defaults.
func parseInterface(entry listEntry) InterfaceConfig {
	m := entry.data
	ic := InterfaceConfig{
		Name:          entry.key,
		Enabled:       configBool(m["enabled"], true),
		Passive:       configBool(m["passive"], false),
		CircuitType:   DefaultCircuitType,
		Level:         parseLevel(m["level"]),
		Metric:        DefaultMetric,
		HelloInterval: DefaultHelloInterval,
		HoldMult:      DefaultHoldMultiplier,
		Priority:      DefaultPriority,
	}
	if v := configString(m["name"]); v != "" {
		ic.Name = v
	}
	if configString(m["circuit-type"]) == circuitTypeP2P {
		ic.CircuitType = CircuitPointToPoint
	}
	if v, ok := configUint32(m["metric"]); ok && v > 0 {
		ic.Metric = v
	}
	if v, ok := configUint16(m["hello-interval"]); ok && v > 0 {
		ic.HelloInterval = v
	}
	if v, ok := configUint8(m["hold-multiplier"]); ok && v > 0 {
		ic.HoldMult = v
	}
	if v, ok := configUint8(m["priority"]); ok {
		ic.Priority = v
	}
	if l1, ok := m["level-1"].(map[string]any); ok {
		ic.Level1 = parseLevelInterface(l1)
	}
	if l2, ok := m["level-2"].(map[string]any); ok {
		ic.Level2 = parseLevelInterface(l2)
	}
	for _, afEntry := range keyedList(m["address-family"], false) {
		af := afEntry.key
		if v := configString(afEntry.data["af"]); v != "" {
			af = v
		}
		ic.AddressFamily = append(ic.AddressFamily, af)
	}
	return ic
}

// parseLevelInterface resolves a per-level interface override container (no
// defaults: a zero field means "inherit the circuit-wide value").
func parseLevelInterface(m map[string]any) LevelInterfaceConfig {
	var lc LevelInterfaceConfig
	if v, ok := configUint32(m["metric"]); ok {
		lc.Metric = v
	}
	if v, ok := configUint16(m["hello-interval"]); ok {
		lc.HelloInterval = v
	}
	if v, ok := configUint8(m["hold-multiplier"]); ok {
		lc.HoldMult = v
	}
	if v, ok := configUint8(m["priority"]); ok {
		lc.Priority = v
	}
	lc.AuthKeyChain = configString(m["auth-key-chain"])
	return lc
}

// parseKeyChain resolves one key-chains{} entry.
func parseKeyChain(entry listEntry) KeyChainConfig {
	kc := KeyChainConfig{Name: entry.key}
	if v := configString(entry.data["name"]); v != "" {
		kc.Name = v
	}
	for _, keyEntry := range keyedList(entry.data["key"], true) {
		k := KeyConfig{Algorithm: "hmac-md5"}
		if v, ok := configUint16(keyEntry.data["key-id"]); ok {
			k.KeyID = v
		} else if id, err := strconv.ParseUint(keyEntry.key, 10, 16); err == nil {
			k.KeyID = uint16(id)
		}
		if v := configString(keyEntry.data["algorithm"]); v != "" {
			k.Algorithm = v
		}
		k.Secret = configString(keyEntry.data["secret"])
		// Optional hitless-rotation lifetimes (RFC3339 start/end) from the YANG
		// send-lifetime / accept-lifetime containers; isis-10 interprets them.
		if sl, ok := keyEntry.data["send-lifetime"].(map[string]any); ok {
			k.SendStart = configString(sl["start"])
			k.SendEnd = configString(sl["end"])
		}
		if al, ok := keyEntry.data["accept-lifetime"].(map[string]any); ok {
			k.AcceptStart = configString(al["start"])
			k.AcceptEnd = configString(al["end"])
		}
		kc.Keys = append(kc.Keys, k)
	}
	return kc
}

// validateConfig enforces the required-field policy that the SDK OnConfigVerify
// callback applies: at least one NET (AC-3), and a consistent explicit
// system-id when one is given (AC-4/AC-9). The per-leaf shape (NET hex/length,
// system-id pattern, numeric ranges, enums) is enforced by YANG native
// validation before this runs, and structurally re-checked by parseISISConfig's
// types.ParseNET / ParseSystemID.
func validateConfig(cfg Config) error {
	if len(cfg.NETs) == 0 {
		return ErrNoNET
	}
	if cfg.systemIDFromConfig {
		if cfg.SystemID != cfg.NETs[0].SystemID() {
			return ErrSystemIDMismatch
		}
	}
	return nil
}

// configSection mirrors sdk.ConfigSection without importing the SDK into the
// config-parse path (so config_test.go does not need the SDK). register.go
// converts the delivered sdk.ConfigSection into this shape.
type configSection struct {
	Root string
	Data string
}
