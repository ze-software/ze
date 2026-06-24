// Design: plan/learned/958-ospf-4-component-config.md -- OSPFv2 config resolution
// Related: yang/ze-ospf-conf.yang -- schema this resolver consumes
// RFC: rfc/short/rfc3101.md -- NSSA translate-role / stability-interval config
//
// Config flows file -> YANG schema -> validated tree -> SDK ConfigSection as
// root-wrapped JSON ({"ospf": {...}}). Tree.ToMap renders scalar leaves as
// strings, keyed lists as key -> entry maps, and nested containers as maps. This
// file mirrors the IS-IS config resolver shape while keeping OSPFv2 types local.
package ospf

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

const (
	DefaultReferenceBandwidth = uint32(100000)
	DefaultMaximumPaths       = uint8(8)
	DefaultSPFDelayMS         = uint32(50)
	DefaultSPFHoldMS          = uint32(200)
	DefaultSPFMaxHoldMS       = uint32(5000)
	DefaultMinLSIntervalMS    = uint32(5000)
	DefaultMinLSArrivalMS     = uint32(1000)
	DefaultExternalMetric     = uint32(20)
	DefaultDefaultMetric      = uint32(1)
	DefaultHelloInterval      = uint16(10)
	DefaultDeadInterval       = uint16(40)
	DefaultRetransmitInterval = uint16(5)
	DefaultTransmitDelay      = uint16(1)
	DefaultPriority           = uint8(1)
	DefaultAreaCost           = uint32(1)
	// DefaultNSSAStabilityInterval is the RFC 3101 section 3.5 translator-stability
	// hysteresis (seconds) a newly demoted translator keeps translating.
	DefaultNSSAStabilityInterval = uint16(40)
)

const (
	areaTypeNormal = "normal"
	areaTypeStub   = "stub"
	areaTypeNSSA   = "nssa"

	translateRoleCandidate = "candidate"
	translateRoleAlways    = "always"
	translateRoleNever     = "never"

	networkBroadcast    = "broadcast"
	networkPointToPoint = "point-to-point"
	networkLoopback     = "loopback"

	metricType1 = "type-1"
	metricType2 = "type-2"

	authModeInherit    = "inherit"
	authAlgorithmMD5   = "md5"
	rangeAdvertise     = "advertise"
	rangeNotAdvertise  = "not-advertise"
	redistributeStatic = "static"
)

var (
	ErrRouterIDRequired  = errors.New("ospf: router-id is required or must be derivable from an IPv4 interface address")
	ErrUndeclaredArea    = errors.New("ospf: interface references undeclared area")
	ErrDuplicateArea     = errors.New("ospf: duplicate canonical area")
	ErrNonIPv4Range      = errors.New("ospf: area range prefix must be IPv4")
	ErrInvalidNSSARole   = errors.New("ospf: nssa translate-role must be candidate, always, or never")
	ErrESNRequiresHMAC   = errors.New("ospf: key-chain extended-sequence (AuType 3) requires an hmac-sha algorithm")
	ErrKeyIDTooWide      = errors.New("ospf: AuType 2 key-id must be 0..255 (the on-wire Key ID is one octet); use extended-sequence for 32-bit key-ids")
	ErrKeyLifetimeFormat = errors.New("ospf: key send/accept lifetime start/end must be an RFC3339 timestamp")
	ErrKeyRolloverGap    = errors.New("ospf: key-chain send-lifetime rollover gap (a key's send start must be at or before the previous key's send end so signing coverage never lapses; RFC 5709 §X / RFC 7210)")
	ErrInterfaceCostZero = errors.New("ospf: interface cost must be greater than 0 (RFC 2328 App C.3)")
	ErrTransmitDelayZero = errors.New("ospf: interface transmit-delay must be greater than 0 (RFC 2328 App C.3 InfTransDelay)")
	ErrSimplePasswordLen = errors.New("ospf: simple-password (AuType 1) secret must be at most 8 octets (RFC 2328 App D); use md5/hmac-sha for longer keys")
)

type areaType string

type networkType string

type authConfig struct {
	Mode     string
	KeyChain string
}

type timerConfig struct {
	SPFDelayMS      uint32
	SPFHoldMS       uint32
	SPFMaxHoldMS    uint32
	MinLSIntervalMS uint32
	MinLSArrivalMS  uint32
}

type defaultInformationConfig struct {
	Originate  bool
	Always     bool
	Metric     uint32
	MetricType string
}

type maxMetricConfig struct {
	RouterLSAAlways bool
	OnStartupSec    uint32 // RFC 6987 stub-router seconds after startup (0 = disabled)
	OnShutdownSec   uint32 // RFC 6987 stub-router seconds during graceful shutdown (0 = disabled)
}

type redistributeConfig struct {
	Source     string
	Metric     uint32
	MetricType string
	Tag        uint32
}

type rangeConfig struct {
	Prefix    netip.Prefix
	Advertise bool
	Cost      uint32
	HasCost   bool
}

type areaConfig struct {
	AreaID       types.AreaID
	AreaType     areaType
	NoSummary    bool
	DefaultCost  uint32
	AuthKeyChain string
	Ranges       []rangeConfig
	// NSSA-only (applied when AreaType is nssa).
	NSSATranslateRole     string // candidate | always | never
	NSSAStabilityInterval uint16 // seconds (RFC 3101 section 3.5 hysteresis)
	NSSADefaultOriginate  bool
}

type interfaceConfig struct {
	Name               string
	AreaID             types.AreaID
	Enabled            bool
	NetworkType        networkType
	Cost               uint16
	HasCost            bool
	HelloInterval      uint16
	DeadInterval       uint16
	Priority           uint8
	Passive            bool
	MTUIgnore          bool
	RetransmitInterval uint16
	TransmitDelay      uint16
	HasTransmitDelay   bool
	Authentication     authConfig
}

type lifetimeConfig struct {
	Start string
	End   string
}

type keyConfig struct {
	KeyID          uint32
	Algorithm      string
	Secret         string //nolint:gosec // G117: config field name, not a literal; masked via ze:sensitive in YANG and never logged
	SendLifetime   lifetimeConfig
	AcceptLifetime lifetimeConfig
}

type keyChainConfig struct {
	Name             string
	ExtendedSequence bool // RFC 7474 AuType 3 (extended 64-bit sequence) instead of AuType 2
	Keys             []keyConfig
}

type ospfConfig struct {
	present            bool
	RouterID           types.RouterID
	routerIDFromConfig bool
	ReferenceBandwidth uint32
	MaximumPaths       uint8
	DefaultInformation defaultInformationConfig
	Timers             timerConfig
	MaxMetric          maxMetricConfig
	Redistribute       []redistributeConfig
	Areas              []areaConfig
	Interfaces         []interfaceConfig
	KeyChains          []keyChainConfig
	// InstanceID is the OSPFv3 Instance ID (RFC 5340 sec 2.5); 0 for the IPv4 family.
	InstanceID uint8
	// V6 is the IPv6 (OSPFv3) address-family sub-config parsed from
	// `ospf { address-family ipv6 { ... } }`; nil when no v6 family is configured. It carries
	// its own areas and interfaces and inherits the parent Router ID. A second engine instance
	// (v6 codec + ospfv3 transport) consumes it.
	V6 *ospfConfig
}

type configSection struct {
	Root string
	Data string
}

type routerIDSource interface {
	Interfaces() ([]iface.InterfaceInfo, error)
}

type systemRouterIDSource struct{}

func (systemRouterIDSource) Interfaces() ([]iface.InterfaceInfo, error) {
	return iface.ListInterfaces()
}

func defaultOSPFConfig() ospfConfig {
	return ospfConfig{
		ReferenceBandwidth: DefaultReferenceBandwidth,
		MaximumPaths:       DefaultMaximumPaths,
		DefaultInformation: defaultInformationConfig{Metric: DefaultDefaultMetric, MetricType: metricType2},
		Timers: timerConfig{
			SPFDelayMS:      DefaultSPFDelayMS,
			SPFHoldMS:       DefaultSPFHoldMS,
			SPFMaxHoldMS:    DefaultSPFMaxHoldMS,
			MinLSIntervalMS: DefaultMinLSIntervalMS,
			MinLSArrivalMS:  DefaultMinLSArrivalMS,
		},
	}
}

func (c ospfConfig) Present() bool { return c.present }

func (c ospfConfig) areaSet() map[types.AreaID]struct{} {
	areas := make(map[types.AreaID]struct{}, len(c.Areas))
	for _, a := range c.Areas {
		areas[a.AreaID] = struct{}{}
	}
	return areas
}

// enrolledInterfaces returns enabled interfaces that bind a declared area.
// Passive interfaces are included in area state, but activeInterfaces excludes
// them from raw-socket Hello processing.
func (c ospfConfig) enrolledInterfaces() []interfaceConfig {
	areas := c.areaSet()
	out := make([]interfaceConfig, 0, len(c.Interfaces))
	for _, ic := range c.Interfaces {
		if !ic.Enabled {
			continue
		}
		if _, ok := areas[ic.AreaID]; ok {
			out = append(out, ic)
		}
	}
	return out
}

func (c ospfConfig) activeInterfaces() []interfaceConfig {
	enrolled := c.enrolledInterfaces()
	out := enrolled[:0]
	for _, ic := range enrolled {
		if !ic.Passive && ic.NetworkType != networkLoopback {
			out = append(out, ic)
		}
	}
	return out
}

func parseOSPFConfig(sections []configSection, source routerIDSource) (ospfConfig, error) {
	cfg := defaultOSPFConfig()
	for _, s := range sections {
		if s.Root != "ospf" || s.Data == "" {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(s.Data), &wrapper); err != nil {
			return cfg, fmt.Errorf("ospf: invalid config JSON: %w", err)
		}
		tree, _ := wrapper["ospf"].(map[string]any)
		if tree == nil {
			continue
		}
		cfg.present = true
		if err := applyTree(&cfg, tree); err != nil {
			return cfg, err
		}
	}
	if cfg.present && !cfg.routerIDFromConfig && source != nil {
		if rid, ok := deriveRouterID(source); ok {
			cfg.RouterID = rid
		}
	}
	// The IPv6 family inherits the (possibly derived) Router ID unless it set its own
	// (OSPFv3 still uses a 32-bit Router ID, RFC 5340 sec 2.11).
	if cfg.V6 != nil && !cfg.V6.routerIDFromConfig {
		cfg.V6.RouterID = cfg.RouterID
	}
	return cfg, nil
}

func applyTree(cfg *ospfConfig, tree map[string]any) error {
	if s := configString(tree["router-id"]); s != "" {
		rid, err := types.ParseRouterID(s)
		if err != nil {
			return fmt.Errorf("ospf: invalid router-id %q: %w", s, err)
		}
		cfg.RouterID = rid
		cfg.routerIDFromConfig = true
	}
	if v, ok := configNumber(tree["reference-bandwidth"]); ok && v > 0 {
		cfg.ReferenceBandwidth = uint32(v)
	}
	if v, ok := configNumber(tree["maximum-paths"]); ok && v > 0 {
		cfg.MaximumPaths = uint8(v)
	}
	if m, ok := tree["default-information"].(map[string]any); ok {
		cfg.DefaultInformation = parseDefaultInformation(m)
	}
	if m, ok := tree["max-metric"].(map[string]any); ok {
		cfg.MaxMetric = parseMaxMetric(m)
	}
	if m, ok := tree["timers"].(map[string]any); ok {
		cfg.Timers = parseTimers(m)
	}
	for _, entry := range keyedList(tree["redistribute"], false) {
		cfg.Redistribute = append(cfg.Redistribute, parseRedistribute(entry))
	}
	if areas, ok := tree["areas"].(map[string]any); ok {
		for _, entry := range keyedList(areas["area"], false) {
			area, err := parseArea(entry)
			if err != nil {
				return err
			}
			cfg.Areas = append(cfg.Areas, area)
		}
	}
	if interfaces, ok := tree["interfaces"].(map[string]any); ok {
		for _, entry := range keyedList(interfaces["interface"], false) {
			ic, err := parseInterface(entry)
			if err != nil {
				return err
			}
			cfg.Interfaces = append(cfg.Interfaces, ic)
		}
	}
	for _, entry := range keyedList(tree["key-chains"], false) {
		cfg.KeyChains = append(cfg.KeyChains, parseKeyChain(entry))
	}
	if v, ok := configNumber(tree["instance-id"]); ok {
		cfg.InstanceID = uint8(v)
	}
	// RFC 5340: the IPv6 (OSPFv3) address family carries its own areas/interfaces under
	// `address-family ipv6`. Parse it into a sub-config the v6 engine instance consumes; it
	// reuses the same area/interface shape and inherits the parent Router ID (set in
	// parseOSPFConfig after derivation). The v6 subtree has no nested address-family, so the
	// recursive applyTree does not re-enter this branch.
	if af, ok := tree["address-family"].(map[string]any); ok {
		if v6, ok := af["ipv6"].(map[string]any); ok {
			sub := defaultOSPFConfig()
			sub.present = true
			if err := applyTree(&sub, v6); err != nil {
				return err
			}
			cfg.V6 = &sub
		}
	}
	return nil
}

// validNSSATranslateRole reports whether s is one of the RFC 3101 translator roles.
func validNSSATranslateRole(s string) bool {
	switch s {
	case translateRoleCandidate, translateRoleAlways, translateRoleNever:
		return true
	default:
		return false
	}
}

func validateConfig(cfg ospfConfig) error {
	if !cfg.present {
		return nil
	}
	if cfg.RouterID == (types.RouterID{}) {
		return ErrRouterIDRequired
	}
	areas := make(map[types.AreaID]struct{}, len(cfg.Areas))
	for _, a := range cfg.Areas {
		if _, dup := areas[a.AreaID]; dup {
			return fmt.Errorf("%w: %s", ErrDuplicateArea, a.AreaID.String())
		}
		if !validNSSATranslateRole(a.NSSATranslateRole) {
			return fmt.Errorf("%w: %q (area %s)", ErrInvalidNSSARole, a.NSSATranslateRole, a.AreaID.String())
		}
		areas[a.AreaID] = struct{}{}
	}
	for _, ic := range cfg.Interfaces {
		if _, ok := areas[ic.AreaID]; !ok {
			return fmt.Errorf("%w: interface %q area %s", ErrUndeclaredArea, ic.Name, ic.AreaID.String())
		}
		// RFC 2328 Appendix C.3: the interface output cost MUST be greater than 0.
		if ic.HasCost && ic.Cost == 0 {
			return fmt.Errorf("%w: interface %q", ErrInterfaceCostZero, ic.Name)
		}
		// RFC 2328 Appendix C.3: InfTransDelay MUST be greater than 0.
		if ic.HasTransmitDelay && ic.TransmitDelay == 0 {
			return fmt.Errorf("%w: interface %q", ErrTransmitDelayZero, ic.Name)
		}
	}
	for _, kc := range cfg.KeyChains {
		for _, k := range kc.Keys {
			if kc.ExtendedSequence {
				// RFC 7474 AuType 3 only defines the HMAC-SHA algorithms.
				if !isHMACSHA(k.Algorithm) {
					return fmt.Errorf("%w: key-chain %q key %d uses %q", ErrESNRequiresHMAC, kc.Name, k.KeyID, k.Algorithm)
				}
				continue
			}
			// RFC 2328 App D / RFC 5709 AuType 2: the on-wire Key ID is a single octet, so a
			// crypto key-id above 255 cannot be represented and would truncate silently.
			// AuType 1 (simple) carries no Key ID on the wire, so it is unconstrained.
			if k.Algorithm != "simple" && k.KeyID > 255 {
				return fmt.Errorf("%w: key-chain %q key %d", ErrKeyIDTooWide, kc.Name, k.KeyID)
			}
			// RFC 2328 App D: the AuType 1 (Simple Password) authentication field is exactly 8
			// octets, so a longer simple-password secret cannot be carried on the wire and would
			// be silently truncated. Reject it at config time instead of truncating.
			if k.Algorithm == "simple" && len(decodeSecret(k.Secret)) > 8 {
				return fmt.Errorf("%w: key-chain %q key %d", ErrSimplePasswordLen, kc.Name, k.KeyID)
			}
		}
		if err := validateKeyRollover(kc); err != nil {
			return err
		}
	}
	if cfg.V6 != nil {
		if err := validateConfig(*cfg.V6); err != nil {
			return fmt.Errorf("address-family ipv6: %w", err)
		}
	}
	return nil
}

// validateKeyRollover checks the send-lifetime windows of a key chain in start order:
// a malformed timestamp is rejected, and a gap where one key's send window ends
// strictly before the next key's send window begins is rejected (RFC 5709 §X / RFC 7210
// require overlapping send lifetimes so signing coverage never lapses). Keys without a
// send-lifetime (unbounded) never create a gap.
func validateKeyRollover(kc keyChainConfig) error {
	type window struct {
		keyID      uint32
		start, end time.Time
	}
	windows := make([]window, 0, len(kc.Keys))
	for _, k := range kc.Keys {
		start, end, ok := lifetimeBounds(k.SendLifetime)
		if !ok {
			return fmt.Errorf("%w: key-chain %q key %d send-lifetime", ErrKeyLifetimeFormat, kc.Name, k.KeyID)
		}
		if _, _, ok := lifetimeBounds(k.AcceptLifetime); !ok {
			return fmt.Errorf("%w: key-chain %q key %d accept-lifetime", ErrKeyLifetimeFormat, kc.Name, k.KeyID)
		}
		windows = append(windows, window{keyID: k.KeyID, start: start, end: end})
	}
	// Order by send-start (zero start sorts first, i.e. earliest); equal starts keep config order.
	sort.SliceStable(windows, func(i, j int) bool { return windows[i].start.Before(windows[j].start) })
	for i := 1; i < len(windows); i++ {
		prev, cur := windows[i-1], windows[i]
		// An unbounded previous end (zero) never lapses; an unbounded current start (zero)
		// begins before any finite end, so it cannot open a gap.
		if prev.end.IsZero() || cur.start.IsZero() {
			continue
		}
		if cur.start.After(prev.end) {
			return fmt.Errorf("%w: key-chain %q key %d send starts %s after key %d send ends %s",
				ErrKeyRolloverGap, kc.Name, cur.keyID, cur.start.Format(time.RFC3339), prev.keyID, prev.end.Format(time.RFC3339))
		}
	}
	return nil
}

// isHMACSHA reports whether algo is one of the RFC 5709 HMAC-SHA algorithms (the only
// ones valid under RFC 7474 extended sequence numbers).
func isHMACSHA(algo string) bool {
	switch algo {
	case "hmac-sha-1", "hmac-sha-256", "hmac-sha-384", "hmac-sha-512":
		return true
	default:
		return false
	}
}

func parseDefaultInformation(m map[string]any) defaultInformationConfig {
	cfg := defaultInformationConfig{Metric: DefaultDefaultMetric, MetricType: metricType2}
	cfg.Originate = configBool(m["originate"], false)
	cfg.Always = configBool(m["always"], false)
	if v, ok := configNumber(m["metric"]); ok {
		cfg.Metric = uint32(v)
	}
	if s := configString(m["metric-type"]); s != "" {
		cfg.MetricType = s
	}
	return cfg
}

func parseMaxMetric(m map[string]any) maxMetricConfig {
	cfg := maxMetricConfig{}
	if routerLSA, ok := m["router-lsa"].(map[string]any); ok {
		cfg.RouterLSAAlways = configBool(routerLSA["always"], false)
		if v, ok := configNumber(routerLSA["on-startup"]); ok {
			cfg.OnStartupSec = uint32(v)
		}
		if v, ok := configNumber(routerLSA["on-shutdown"]); ok {
			cfg.OnShutdownSec = uint32(v)
		}
	}
	return cfg
}

func parseTimers(m map[string]any) timerConfig {
	t := defaultOSPFConfig().Timers
	if v, ok := configNumber(m["spf-delay-ms"]); ok {
		t.SPFDelayMS = uint32(v)
	}
	if v, ok := configNumber(m["spf-hold-ms"]); ok {
		t.SPFHoldMS = uint32(v)
	}
	if v, ok := configNumber(m["spf-max-hold-ms"]); ok {
		t.SPFMaxHoldMS = uint32(v)
	}
	if v, ok := configNumber(m["min-ls-interval-ms"]); ok {
		t.MinLSIntervalMS = uint32(v)
	}
	if v, ok := configNumber(m["min-ls-arrival-ms"]); ok {
		t.MinLSArrivalMS = uint32(v)
	}
	return t
}

func parseRedistribute(entry listEntry) redistributeConfig {
	r := redistributeConfig{Source: entry.key, Metric: DefaultExternalMetric, MetricType: metricType2}
	if s := configString(entry.data["source"]); s != "" {
		r.Source = s
	}
	if v, ok := configNumber(entry.data["metric"]); ok {
		r.Metric = uint32(v)
	}
	if s := configString(entry.data["metric-type"]); s != "" {
		r.MetricType = s
	}
	if v, ok := configNumber(entry.data["tag"]); ok {
		r.Tag = uint32(v)
	}
	return r
}

func parseArea(entry listEntry) (areaConfig, error) {
	idText := entry.key
	if s := configString(entry.data["area-id"]); s != "" {
		idText = s
	}
	id, err := types.ParseAreaID(idText)
	if err != nil {
		return areaConfig{}, fmt.Errorf("ospf: invalid area-id %q: %w", idText, err)
	}
	a := areaConfig{
		AreaID:                id,
		AreaType:              areaTypeNormal,
		DefaultCost:           DefaultAreaCost,
		NSSATranslateRole:     translateRoleCandidate,
		NSSAStabilityInterval: DefaultNSSAStabilityInterval,
	}
	if s := configString(entry.data["area-type"]); s != "" {
		// Validate against the YANG enum instead of silently coercing an unrecognized value
		// (which fell through to normal). Defends the non-YANG doctor/verifier parse paths.
		switch s {
		case areaTypeNormal, areaTypeStub, areaTypeNSSA:
			a.AreaType = areaType(s)
		default:
			return areaConfig{}, fmt.Errorf("ospf: area %s invalid area-type %q (want normal|stub|nssa)", id, s)
		}
	}
	a.NoSummary = configBool(entry.data["no-summary"], false)
	if v, ok := configNumber(entry.data["default-cost"]); ok {
		a.DefaultCost = uint32(v)
	}
	if nssa, ok := entry.data["nssa"].(map[string]any); ok {
		if s := configString(nssa["translate-role"]); s != "" {
			a.NSSATranslateRole = s
		}
		if v, ok := configNumber(nssa["stability-interval"]); ok {
			a.NSSAStabilityInterval = uint16(v)
		}
		a.NSSADefaultOriginate = configBool(nssa["default-originate"], false)
	}
	if auth, ok := entry.data["authentication"].(map[string]any); ok {
		a.AuthKeyChain = configString(auth["key-chain"])
	}
	if ranges, ok := entry.data["ranges"].(map[string]any); ok {
		for _, rangeEntry := range keyedList(ranges["range"], false) {
			r, err := parseRange(rangeEntry)
			if err != nil {
				return areaConfig{}, err
			}
			a.Ranges = append(a.Ranges, r)
		}
	}
	return a, nil
}

func parseRange(entry listEntry) (rangeConfig, error) {
	prefixText := entry.key
	if s := configString(entry.data["prefix"]); s != "" {
		prefixText = s
	}
	pfx, err := netip.ParsePrefix(prefixText)
	if err != nil {
		return rangeConfig{}, fmt.Errorf("ospf: invalid area range prefix %q: %w", prefixText, err)
	}
	if !pfx.Addr().Is4() {
		return rangeConfig{}, fmt.Errorf("%w: %s", ErrNonIPv4Range, prefixText)
	}
	r := rangeConfig{Prefix: pfx, Advertise: true}
	if s := configString(entry.data["advertise"]); s == rangeNotAdvertise {
		r.Advertise = false
	}
	if v, ok := configNumber(entry.data["cost"]); ok {
		r.Cost = uint32(v)
		r.HasCost = true
	}
	return r, nil
}

func parseInterface(entry listEntry) (interfaceConfig, error) {
	m := entry.data
	ic := interfaceConfig{
		Name:               entry.key,
		Enabled:            configBool(m["enabled"], true),
		NetworkType:        networkBroadcast,
		HelloInterval:      DefaultHelloInterval,
		DeadInterval:       DefaultDeadInterval,
		Priority:           DefaultPriority,
		RetransmitInterval: DefaultRetransmitInterval,
		TransmitDelay:      DefaultTransmitDelay,
		Authentication:     authConfig{Mode: authModeInherit},
	}
	if s := configString(m["name"]); s != "" {
		ic.Name = s
	}
	areaText := configString(m["area"])
	if areaText == "" {
		return ic, fmt.Errorf("ospf: interface %q missing area", ic.Name)
	}
	areaID, err := types.ParseAreaID(areaText)
	if err != nil {
		return ic, fmt.Errorf("ospf: interface %q invalid area %q: %w", ic.Name, areaText, err)
	}
	ic.AreaID = areaID
	if s := configString(m["network-type"]); s != "" {
		// Validate against the YANG enum instead of silently coercing an unrecognized value
		// (which fell through to broadcast). Defends the non-YANG doctor/verifier parse paths.
		switch s {
		case networkBroadcast, networkPointToPoint, networkLoopback:
			ic.NetworkType = networkType(s)
		default:
			return ic, fmt.Errorf("ospf: interface %q invalid network-type %q (want broadcast|point-to-point|loopback)", ic.Name, s)
		}
	}
	if v, ok := configNumber(m["cost"]); ok {
		// The interface cost is a 16-bit field (YANG range 1..65535); reject an out-of-range
		// value rather than silently truncating it via uint16 (e.g. 65536 -> 0).
		if v > 65535 {
			return ic, fmt.Errorf("ospf: interface %q cost %d out of range (1-65535)", ic.Name, v)
		}
		ic.Cost = uint16(v)
		ic.HasCost = true
	}
	if v, ok := configNumber(m["hello-interval"]); ok && v > 0 {
		ic.HelloInterval = uint16(v)
	}
	if v, ok := configNumber(m["dead-interval"]); ok && v > 0 {
		ic.DeadInterval = uint16(v)
	}
	if v, ok := configNumber(m["priority"]); ok {
		ic.Priority = uint8(v)
	}
	ic.Passive = configBool(m["passive"], false)
	ic.MTUIgnore = configBool(m["mtu-ignore"], false)
	if v, ok := configNumber(m["retransmit-interval"]); ok && v > 0 {
		ic.RetransmitInterval = uint16(v)
	}
	if v, ok := configNumber(m["transmit-delay"]); ok {
		ic.TransmitDelay = uint16(v)
		ic.HasTransmitDelay = true
	}
	if auth, ok := m["authentication"].(map[string]any); ok {
		ic.Authentication = parseAuth(auth, authModeInherit)
	}
	return ic, nil
}

func parseAuth(m map[string]any, defMode string) authConfig {
	a := authConfig{Mode: defMode}
	if s := configString(m["mode"]); s != "" {
		a.Mode = s
	}
	a.KeyChain = configString(m["key-chain"])
	return a
}

func parseKeyChain(entry listEntry) keyChainConfig {
	kc := keyChainConfig{Name: entry.key}
	if s := configString(entry.data["name"]); s != "" {
		kc.Name = s
	}
	kc.ExtendedSequence = configBool(entry.data["extended-sequence"], false)
	for _, keyEntry := range keyedList(entry.data["key"], true) {
		k := keyConfig{Algorithm: authAlgorithmMD5}
		if v, ok := configNumber(keyEntry.data["key-id"]); ok {
			k.KeyID = uint32(v)
		} else if id, err := strconv.ParseUint(keyEntry.key, 10, 32); err == nil {
			k.KeyID = uint32(id)
		}
		if s := configString(keyEntry.data["algorithm"]); s != "" {
			k.Algorithm = s
		}
		k.Secret = configString(keyEntry.data["secret"])
		if sl, ok := keyEntry.data["send-lifetime"].(map[string]any); ok {
			k.SendLifetime = parseLifetime(sl)
		}
		if al, ok := keyEntry.data["accept-lifetime"].(map[string]any); ok {
			k.AcceptLifetime = parseLifetime(al)
		}
		kc.Keys = append(kc.Keys, k)
	}
	return kc
}

func parseLifetime(m map[string]any) lifetimeConfig {
	return lifetimeConfig{Start: configString(m["start"]), End: configString(m["end"])}
}

// lifetimeBounds parses a lifetimeConfig's RFC3339 timestamps into a half-open
// [start, end) window. An empty Start or End yields the zero time.Time for that
// bound, which the keystore treats as unbounded (always valid), so a key with no
// configured lifetime is active at all times. ok is false only when a present
// string fails to parse.
func lifetimeBounds(l lifetimeConfig) (start, end time.Time, ok bool) {
	if l.Start != "" {
		t, err := time.Parse(time.RFC3339, l.Start)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		start = t
	}
	if l.End != "" {
		t, err := time.Parse(time.RFC3339, l.End)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		end = t
	}
	return start, end, true
}

func deriveRouterID(source routerIDSource) (types.RouterID, bool) {
	infos, err := source.Interfaces()
	if err != nil {
		return types.RouterID{}, false
	}
	return deriveRouterIDFromInterfaces(infos)
}

// RFC 2328 Section C.1: when no Router ID is configured, routers commonly pick
// the highest loopback address, else the highest interface address. This helper
// keeps that policy pure and testable; the source owns OS discovery.
func deriveRouterIDFromInterfaces(infos []iface.InterfaceInfo) (types.RouterID, bool) {
	var loop, any netip.Addr
	var haveLoop, haveAny bool
	for i := range infos {
		isLoop := isLoopback(infos[i])
		for _, a := range infos[i].Addresses {
			addr, err := netip.ParseAddr(a.Address)
			if err != nil || !addr.Is4() || addr.IsUnspecified() {
				continue
			}
			if !haveAny || addr4Less(any, addr) {
				any = addr
				haveAny = true
			}
			if isLoop && (!haveLoop || addr4Less(loop, addr)) {
				loop = addr
				haveLoop = true
			}
		}
	}
	if haveLoop {
		return routerIDFromAddr(loop), true
	}
	if haveAny {
		return routerIDFromAddr(any), true
	}
	return types.RouterID{}, false
}

func routerIDFromAddr(addr netip.Addr) types.RouterID { return types.RouterID(addr.As4()) }

func isLoopback(info iface.InterfaceInfo) bool {
	return info.Type == "loopback" || info.Name == "lo" || info.Name == "lo0" || strings.HasPrefix(info.Name, "lo:")
}

func addr4Less(a, b netip.Addr) bool { return addr4Value(a) < addr4Value(b) }

func addr4Value(addr netip.Addr) uint32 {
	b := addr.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

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

func configString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type listEntry struct {
	key  string
	data map[string]any
}

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
			ai, _ := strconv.ParseUint(entries[i].key, 10, 64)
			bj, _ := strconv.ParseUint(entries[j].key, 10, 64)
			return ai < bj
		})
	} else {
		sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	}
	return entries
}
