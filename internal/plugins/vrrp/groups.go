// RFC: rfc/short/rfc9568.md -- VRRPv3 config semantics (Sections 5.2.4, 5.2.7, 5.2.9, 6.1)
// RFC: rfc/short/rfc3768.md -- VRRPv2 config semantics (Section 5.3.7)
//
// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- VRRP config extraction and verification
//
// Extraction walks ONLY the vrrp-bearing path of the shared `interface` config
// section (type list -> name -> unit -> ipv4|ipv6 -> vrrp -> group) and ignores
// every other key at each level, so hosting the plugin on the same config root
// as the iface component stays cheap on reload (spec-vrrp-5 R-1).
//
// Cross-leaf rules live here rather than in per-leaf `ze:validate` handlers
// because each needs sibling context: the interval range depends on the version
// leaf, owner detection compares a group's VIPs against the unit's real
// addresses, and duplicate-VIP detection spans groups (spec-vrrp-5 D-4).
package vrrp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// configRoot is the config section this plugin consumes. It is the iface
// component's root: vrrp config lives under interface units, so both plugins
// receive the same section (umbrella A-2).
const configRoot = "interface"

// Address families. VRID namespaces are independent per family: RFC 9568
// Section 1.2 models IPv4 and IPv6 virtual routers as separate virtual routers.
const (
	familyIPv4 = "ipv4"
	familyIPv6 = "ipv6"
)

// Interface backends. VRRP needs macvlan devices and raw sockets, so it is
// netlink-only until native VPP VRRP lands (spec-vrrp-7).
const (
	backendNetlink = "netlink"
	backendVPP     = "vpp"
)

// Protocol versions.
const (
	versionV2 = 2
	versionV3 = 3
)

// Leaf defaults. These mirror the YANG defaults in yang/ze-vrrp-conf.yang; the
// engine never sees an unset field, so a group extracted from a sparse tree
// behaves exactly like one whose defaults were written out.
const (
	defaultPriority         = 100
	defaultAdvertIntervalMs = 1000
	ownerPriority           = 255 // RFC 9568 Section 5.2.4: reserved for the address owner.
	maxVIPs                 = 16
)

// Interval bounds per version, in milliseconds.
const (
	v3IntervalMinMs  = 10    // 1 centisecond (RFC 9568 erratum 8301).
	v3IntervalMaxMs  = 40950 // 4095 centiseconds (12-bit Max Advertise Interval).
	v3IntervalStepMs = 10    // Wire unit is centiseconds.
	v2IntervalMinMs  = 1000  // 1 second (RFC 3768 Section 5.3.7, 8-bit seconds).
	v2IntervalMaxMs  = 255000
	v2IntervalStepMs = 1000
)

// ifaceTypes are the interface list names whose units the vrrp YANG augments
// attach to. Kept in sync with the augment paths in yang/ze-vrrp-conf.yang.
var ifaceTypes = []string{"ethernet", "veth", "bridge", "dummy"}

var (
	// errMissingVRID fires when a group carries no vrid. The schema marks it
	// mandatory, so this only reaches an operator through a producer that
	// bypassed schema validation -- still a hard error, never a default.
	errMissingVRID = errors.New("vrid is required: it is the virtual router's identity on the wire (RFC 9568 Section 5.2.3), and the group name is a local label only")
	// errZeroVRID fires on vrid 0, which the range constraint already rejects.
	errZeroVRID = errors.New("vrid 0 is not valid: configure 1..255")
)

// configSection mirrors the SDK/RPC config section shape without importing
// either into the pure config layer (ospf model, config.go:530).
type configSection struct {
	Root string
	Data string
}

// GroupSpec is one configured virtual router: the (interface, unit, family,
// vrid) key plus its resolved leaves. It is the single config value type shared
// by the verifier, the engine, and the show surface.
type GroupSpec struct {
	IfType    string
	Interface string
	Unit      string
	Family    string
	// Name is the operator's label for this group and the config list key. It
	// never reaches the wire: VRID is the protocol identity. Naming the group
	// lets an operator rename or renumber independently (and matches the
	// wireguard peer precedent, ze-iface-conf.yang list peer { key "name" }).
	Name string
	// ParentDevice is the OS device that hosts this virtual router: the
	// interface itself, or its VLAN sub-interface when the unit carries a
	// vlan-id (the iface naming rule, config_apply.go:35-39). Sockets, the
	// macvlan parent, and the tie-break source address all belong to THIS
	// device, not to the logical interface name.
	ParentDevice        string
	VRID                uint8
	VIPs                []netip.Addr // Wire order == configuration order (RFC 9568 Section 5.2.9).
	Priority            uint8
	Preempt             bool
	PreemptDelaySeconds uint16
	AdvertIntervalMs    uint32
	Version             uint8
	AcceptMode          bool
	IsOwner             bool // VIP equals a real address on the same unit+family.

	// realAddresses are the unit's configured addresses for this family, kept
	// for owner detection. Not part of the key.
	realAddresses []netip.Addr

	// realPrefixes are the unit's configured addresses WITH their prefix length,
	// for this family. A virtual address is installed on the macvlan with the
	// prefix of the parent subnet that contains it (vipCIDR), so the macvlan
	// carries the subnet's connected route -- the kernel needs that route to
	// answer ARP/ND for the VIP from the virtual MAC (spec-vrrp-6 dataplane
	// recipe). Not part of the key.
	realPrefixes []netip.Prefix
}

// EffectivePriority is the priority the FSM runs with.
//
// RFC 9568 Section 5.2.4 requires the priority of the router that owns the
// virtual router's IPvX addresses to be 255, so ownership overrides whatever
// priority the operator configured.
func (g GroupSpec) EffectivePriority() uint8 {
	if g.IsOwner {
		return ownerPriority
	}
	return g.Priority
}

// EffectiveAcceptMode is the accept-mode the FSM runs with.
//
// RFC 9568 Section 6.1: the owner always accepts packets addressed to the
// virtual addresses regardless of the configured Accept_Mode.
func (g GroupSpec) EffectiveAcceptMode() bool {
	return g.IsOwner || g.AcceptMode
}

// Key identifies the instance this spec configures.
//
// Keyed on the config NAME, not the VRID: renumbering a group's vrid must
// reconfigure that same instance, not silently create a second one alongside it.
func (g GroupSpec) Key() string {
	var tb textbuf.Buffer
	return tb.Str(g.Interface).Byte('/').Str(g.Unit).Byte('/').Str(g.Family).Byte('/').Str(g.Name).String()
}

// describe names the group for diagnostics: every error message must locate the
// offending config node for the operator (ai/rules/error-messages.md). The name
// is what the operator typed, so it leads; the vrid follows because it is what
// the peers see, and is omitted while it is still unknown (the error that
// reports a MISSING vrid must not claim the group has vrid 0).
//
// No "vrrp:" prefix: the config pipeline already labels a plugin's errors with
// its name, and doubling it reads as a bug to the operator.
func (g GroupSpec) describe() string {
	var tb textbuf.Buffer
	tb.Str("interface ").Str(g.Interface).
		Str(" unit ").Str(g.Unit).
		Byte(' ').Str(g.Family).
		Str(" group ").Quoted(g.Name)
	if g.VRID != 0 {
		tb.Str(" (vrid ").Uint8(g.VRID).Byte(')')
	}
	return tb.String()
}

// vipKey identifies a virtual address within one unit+family scope.
func (g GroupSpec) vipKey(vip netip.Addr) string {
	var tb textbuf.Buffer
	return tb.Str(g.Interface).Byte('|').Str(g.Unit).Byte('|').Str(g.Family).Byte('|').Addr(vip).String()
}

// vridKey identifies a VRID within one unit+family scope: the scope in which
// RFC 9568 Section 1.2 requires it to be unique.
func (g GroupSpec) vridKey() string {
	var tb textbuf.Buffer
	return tb.Str(g.Interface).Byte('|').Str(g.Unit).Byte('|').Str(g.Family).Byte('|').Uint8(g.VRID).String()
}

// ifaceBackend reports the configured interface backend, defaulting to netlink
// when the leaf is absent. The vrrp verifier reads it itself rather than
// depending on the iface plugin having verified first (spec-vrrp-5 R-6).
func ifaceBackend(sections []configSection) string {
	for _, s := range sections {
		if s.Root != configRoot {
			continue
		}
		tree, err := sectionTree(s)
		if err != nil {
			continue
		}
		if b, ok := tree["backend"].(string); ok && b != "" {
			return b
		}
	}
	return backendNetlink
}

// sectionTree decodes a section's JSON payload and returns the `interface`
// subtree. Callers filter on Root before calling.
func sectionTree(s configSection) (map[string]any, error) {
	var outer map[string]any
	if err := json.Unmarshal([]byte(s.Data), &outer); err != nil {
		return nil, fmt.Errorf("vrrp: decode interface config section: %w", err)
	}
	tree, _ := outer[configRoot].(map[string]any)
	if tree == nil {
		// Some producers deliver the section already rooted at the container.
		tree = outer
	}
	return tree, nil
}

// extractGroupSpecs walks the interface sections and returns every configured
// group, sorted by key for deterministic diffing. Keys other than the vrrp path
// are skipped without inspection (extract-only walk, spec-vrrp-5 R-1).
func extractGroupSpecs(sections []configSection) ([]GroupSpec, error) {
	var specs []GroupSpec
	for _, s := range sections {
		if s.Root != configRoot {
			continue
		}
		tree, err := sectionTree(s)
		if err != nil {
			return nil, err
		}
		for _, ifType := range ifaceTypes {
			byName, ok := tree[ifType].(map[string]any)
			if !ok {
				continue
			}
			for name, raw := range byName {
				ifCfg, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				units, ok := ifCfg["unit"].(map[string]any)
				if !ok {
					continue
				}
				for unit, rawUnit := range units {
					unitCfg, ok := rawUnit.(map[string]any)
					if !ok {
						continue
					}
					device, err := unitDevice(name, unitCfg)
					if err != nil {
						return nil, err
					}
					for _, family := range []string{familyIPv4, familyIPv6} {
						famCfg, ok := unitCfg[family].(map[string]any)
						if !ok {
							continue
						}
						got, gerr := groupsForFamily(ifType, name, unit, device, family, famCfg)
						if gerr != nil {
							return nil, gerr
						}
						specs = append(specs, got...)
					}
				}
			}
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Key() < specs[j].Key() })
	return specs, nil
}

// unitDevice returns the OS device that hosts a unit.
//
// The iface component names a unit's device after its parent, suffixed with the
// VLAN id when the unit carries one (config_apply.go:35-39). VRRP must bind its
// sockets and hang its macvlan on THAT device: a group on a VLAN unit that
// bound the bare parent would advertise on the wrong broadcast domain and
// silently never see its peers.
func unitDevice(iface string, unitCfg map[string]any) (string, error) {
	v, ok := unitCfg["vlan-id"]
	if !ok {
		return iface, nil
	}
	vlan, err := asUint(v, 4094)
	if err != nil {
		return "", fmt.Errorf("vrrp: interface %s: vlan-id: %w", iface, err)
	}
	if vlan == 0 {
		return iface, nil
	}
	var tb textbuf.Buffer
	return tb.Str(iface).Byte('.').Uint(vlan).String(), nil
}

// groupsForFamily extracts the vrrp groups configured under one unit's family
// container, resolving owner status against that family's real addresses.
func groupsForFamily(ifType, name, unit, device, family string, famCfg map[string]any) ([]GroupSpec, error) {
	vrrpCfg, ok := famCfg["vrrp"].(map[string]any)
	if !ok {
		return nil, nil
	}
	groups, ok := vrrpCfg["group"].(map[string]any)
	if !ok {
		return nil, nil
	}
	real := parseAddressList(famCfg["address"])
	realPfx := parsePrefixList(famCfg["address"])

	var out []GroupSpec
	for groupName, raw := range groups {
		groupCfg, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		spec := GroupSpec{
			IfType:           ifType,
			Interface:        name,
			Unit:             unit,
			ParentDevice:     device,
			Family:           family,
			Name:             groupName,
			Priority:         defaultPriority,
			Preempt:          true,
			AdvertIntervalMs: defaultAdvertIntervalMs,
			Version:          versionV3,
			realAddresses:    real,
			realPrefixes:     realPfx,
		}
		if err := applyGroupLeaves(&spec, groupCfg); err != nil {
			return nil, fmt.Errorf("%s: %w", spec.describe(), err)
		}
		spec.IsOwner = ownsAny(spec.VIPs, real)
		out = append(out, spec)
	}
	return out, nil
}

// applyGroupLeaves overlays the configured leaves onto a spec pre-loaded with
// YANG defaults.
func applyGroupLeaves(spec *GroupSpec, groupCfg map[string]any) error {
	// vrid is mandatory in the schema, but a config delivered by a producer that
	// skipped schema validation must still fail loudly rather than run as VRID 0
	// (a wire-invalid value that would make every peer discard our adverts).
	v, ok := groupCfg["vrid"]
	if !ok {
		return errMissingVRID
	}
	vrid, err := asUint(v, 255)
	if err != nil {
		return fmt.Errorf("vrid: %w", err)
	}
	if vrid == 0 {
		return errZeroVRID
	}
	spec.VRID = uint8(vrid)

	for _, v := range asSlice(groupCfg["virtual-address"]) {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("virtual-address %v is not a string", v)
		}
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return fmt.Errorf("virtual-address %q is not an IP address: %w", s, err)
		}
		spec.VIPs = append(spec.VIPs, addr)
	}
	if v, ok := groupCfg["priority"]; ok {
		n, err := asUint(v, 255)
		if err != nil {
			return fmt.Errorf("priority: %w", err)
		}
		spec.Priority = uint8(n)
	}
	if v, ok := groupCfg["preempt"]; ok {
		b, err := asBool(v)
		if err != nil {
			return fmt.Errorf("preempt: %w", err)
		}
		spec.Preempt = b
	}
	if v, ok := groupCfg["preempt-delay-seconds"]; ok {
		n, err := asUint(v, 3600)
		if err != nil {
			return fmt.Errorf("preempt-delay-seconds: %w", err)
		}
		spec.PreemptDelaySeconds = uint16(n)
	}
	if v, ok := groupCfg["advertise-interval-milliseconds"]; ok {
		n, err := asUint(v, 1<<32-1)
		if err != nil {
			return fmt.Errorf("advertise-interval-milliseconds: %w", err)
		}
		spec.AdvertIntervalMs = uint32(n)
	}
	if v, ok := groupCfg["accept-mode"]; ok {
		b, err := asBool(v)
		if err != nil {
			return fmt.Errorf("accept-mode: %w", err)
		}
		spec.AcceptMode = b
	}
	if v, ok := groupCfg["version"]; ok {
		n, err := parseVersion(v)
		if err != nil {
			return err
		}
		spec.Version = n
	}
	return nil
}

// parseVersion accepts the enum in the shapes a config tree can carry.
func parseVersion(v any) (uint8, error) {
	switch t := v.(type) {
	case string:
		n, err := strconv.ParseUint(t, 10, 8)
		if err != nil || (n != versionV2 && n != versionV3) {
			return 0, fmt.Errorf("version %q: configure 2 (RFC 3768) or 3 (RFC 9568)", t)
		}
		return uint8(n), nil
	case float64:
		if t != versionV2 && t != versionV3 {
			return 0, fmt.Errorf("version %v: configure 2 (RFC 3768) or 3 (RFC 9568)", t)
		}
		return uint8(t), nil
	default:
		return 0, fmt.Errorf("version %v: configure 2 (RFC 3768) or 3 (RFC 9568)", v)
	}
}

// validateGroups applies every cross-leaf rule the YANG grammar cannot express.
// It is pure: no side effects, safe to run at verify time.
func validateGroups(specs []GroupSpec, backend string) error {
	// A VPP-backed tree cannot host VRRP: the implementation needs macvlan
	// devices and raw sockets that only the netlink backend provides. Fail
	// closed rather than silently approximating (exact-or-reject).
	if backend == backendVPP && len(specs) > 0 {
		g := specs[0]
		return fmt.Errorf("%s: VRRP requires the netlink interface backend, but /interface/backend is %q; remove the vrrp configuration or set the netlink backend (native VPP VRRP is not implemented)",
			g.describe(), backend)
	}

	seenVIP := make(map[string]string)  // unit+family+VIP -> owning group name
	seenVRID := make(map[string]string) // unit+family+vrid -> owning group name
	for i := range specs {
		g := &specs[i]
		if err := validateGroup(*g); err != nil {
			return err
		}
		// Naming the group made the VRID a leaf, so two groups can now claim the
		// same VRID on one unit+family. That has to be rejected: they would be
		// the same virtual router to every peer on the link, and ze would run
		// two state machines fighting over one identity, one virtual MAC, and
		// one macvlan name (RFC 9568 Section 1.2 -- a VRID is unique per
		// interface per family).
		vk := g.vridKey()
		if other, dup := seenVRID[vk]; dup {
			return fmt.Errorf("%s: vrid %d is also configured by group %q on the same unit and family; a VRID identifies one virtual router, so give one group a different vrid",
				g.describe(), g.VRID, other)
		}
		seenVRID[vk] = g.Name

		for _, vip := range g.VIPs {
			k := g.vipKey(vip)
			if other, dup := seenVIP[k]; dup {
				return fmt.Errorf("%s: virtual-address %s is also configured by group %q on the same unit; each virtual address belongs to exactly one group",
					g.describe(), vip, other)
			}
			seenVIP[k] = g.Name
		}
	}
	return nil
}

// validateGroup applies the per-group cross-leaf rules.
func validateGroup(g GroupSpec) error {
	where := g.describe()

	if len(g.VIPs) == 0 {
		return fmt.Errorf("%s: at least one virtual-address is required", where)
	}
	if len(g.VIPs) > maxVIPs {
		return fmt.Errorf("%s: %d virtual-address entries exceed the maximum of %d", where, len(g.VIPs), maxVIPs)
	}
	// RFC 9568 Section 5.2.4: 255 is the owner's priority and is assigned by
	// ze, never configured; 0 is reserved for the resignation advertisement.
	if g.Priority == 0 || g.Priority == ownerPriority {
		return fmt.Errorf("%s: priority %d is out of range; configure 1..254 (255 is assigned automatically to the address owner)", where, g.Priority)
	}

	// IPv6 is VRRPv3 only (RFC 9568); the version leaf does not exist there.
	if g.Family == familyIPv6 && g.Version != versionV3 {
		return fmt.Errorf("%s: IPv6 groups are always VRRPv3; version %d cannot be configured for an IPv6 group", where, g.Version)
	}

	for _, vip := range g.VIPs {
		if g.Family == familyIPv4 && !vip.Is4() {
			return fmt.Errorf("%s: virtual-address %s is not an IPv4 address", where, vip)
		}
		if g.Family == familyIPv6 && vip.Is4() {
			return fmt.Errorf("%s: virtual-address %s is not an IPv6 address", where, vip)
		}
	}

	// RFC 9568 Section 5.2.9 (with erratum 8300): for IPv6 the first address in
	// the list is the link-local address, and it is the advertisement source
	// identity, so a global-first list would emit non-conformant advertisements.
	if g.Family == familyIPv6 && !g.VIPs[0].IsLinkLocalUnicast() {
		return fmt.Errorf("%s: the first virtual-address must be an IPv6 link-local (fe80::/10) address, but it is %s; the first address is the advertisement source identity (RFC 9568 Section 5.2.9)",
			where, g.VIPs[0])
	}

	// Accept_Mode exists only in VRRPv3 (RFC 9568 Section 6.1); RFC 3768 has no
	// such concept, so accepting the leaf under version 2 would silently lie.
	if g.Version == versionV2 && g.AcceptMode {
		return fmt.Errorf("%s: accept-mode is a VRRPv3 feature and cannot be combined with version 2; remove accept-mode or configure version 3", where)
	}

	return validateInterval(g, where)
}

// validateInterval narrows the YANG-native interval range to what the group's
// version can actually encode on the wire.
func validateInterval(g GroupSpec, where string) error {
	switch g.Version {
	case versionV3:
		// RFC 9568 Section 5.2.7: the Max Advertise Interval field is 12 bits of
		// centiseconds, so ze rejects sub-centisecond granularity at verify time
		// rather than rounding behind the operator's back.
		if g.AdvertIntervalMs < v3IntervalMinMs || g.AdvertIntervalMs > v3IntervalMaxMs {
			return fmt.Errorf("%s: advertise-interval-milliseconds %d is out of range for VRRPv3; configure %d..%d",
				where, g.AdvertIntervalMs, v3IntervalMinMs, v3IntervalMaxMs)
		}
		if g.AdvertIntervalMs%v3IntervalStepMs != 0 {
			return fmt.Errorf("%s: advertise-interval-milliseconds %d is not a multiple of %d; VRRPv3 encodes centiseconds on the wire (RFC 9568 Section 5.2.7)",
				where, g.AdvertIntervalMs, v3IntervalStepMs)
		}
	case versionV2:
		// RFC 3768 Section 5.3.7: the Adver Int field is 8 bits of whole seconds.
		if g.AdvertIntervalMs < v2IntervalMinMs || g.AdvertIntervalMs > v2IntervalMaxMs {
			return fmt.Errorf("%s: advertise-interval-milliseconds %d is out of range for VRRPv2; configure %d..%d",
				where, g.AdvertIntervalMs, v2IntervalMinMs, v2IntervalMaxMs)
		}
		if g.AdvertIntervalMs%v2IntervalStepMs != 0 {
			return fmt.Errorf("%s: advertise-interval-milliseconds %d is not a whole number of seconds; VRRPv2 encodes seconds on the wire (RFC 3768 Section 5.3.7)",
				where, g.AdvertIntervalMs)
		}
	default:
		return fmt.Errorf("%s: unsupported version %d; configure 2 or 3", where, g.Version)
	}
	return nil
}

// ownsAny reports whether any VIP equals one of the unit's real addresses,
// which makes this router the address owner for the group.
func ownsAny(vips, real []netip.Addr) bool {
	for _, vip := range vips {
		if slices.Contains(real, vip) {
			return true
		}
	}
	return false
}

// parseAddressList extracts the bare addresses from a unit's address leaf-list,
// dropping the prefix length (owner detection compares addresses, not prefixes).
func parseAddressList(v any) []netip.Addr {
	var out []netip.Addr
	for _, e := range asSlice(v) {
		s, ok := e.(string)
		if !ok {
			continue
		}
		if pfx, err := netip.ParsePrefix(s); err == nil {
			out = append(out, pfx.Addr())
			continue
		}
		if addr, err := netip.ParseAddr(s); err == nil {
			out = append(out, addr)
		}
	}
	return out
}

// parsePrefixList extracts the addresses WITH their prefix length from a unit's
// address leaf-list, keeping only entries that carry a prefix (a bare address
// has no subnet to contribute). Used to size the VIP mask (register.go vipCIDR).
func parsePrefixList(v any) []netip.Prefix {
	var out []netip.Prefix
	for _, e := range asSlice(v) {
		s, ok := e.(string)
		if !ok {
			continue
		}
		if pfx, err := netip.ParsePrefix(s); err == nil {
			out = append(out, pfx)
		}
	}
	return out
}

// asSlice normalizes a leaf-list value, which arrives as a JSON array or, when
// a single element was configured, as a bare scalar.
func asSlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case nil:
		return nil
	default:
		return []any{t}
	}
}

// asBool accepts the shapes a boolean leaf can arrive in.
//
// The text config parser stores scalars as strings ("true"), while a JSON
// producer sends a real bool. Accepting only one shape drops the other
// silently, which is how `accept-mode true` once parsed as false and slipped
// past its own validator. An uninterpretable value is an error, never a
// default: config an operator wrote must take effect or be refused.
func asBool(v any) (bool, error) {
	switch t := v.(type) {
	case bool:
		return t, nil
	case string:
		b, err := strconv.ParseBool(t)
		if err != nil {
			return false, fmt.Errorf("%q is not a boolean (want true or false)", t)
		}
		return b, nil
	default:
		return false, fmt.Errorf("%v is not a boolean (want true or false)", v)
	}
}

// asUint accepts the numeric shapes a config tree can carry (JSON numbers, or
// strings from text-form producers) and range-checks the result.
func asUint(v any, max uint64) (uint64, error) {
	var n uint64
	switch t := v.(type) {
	case float64:
		if t < 0 || t != float64(uint64(t)) {
			return 0, fmt.Errorf("%v is not a whole number", t)
		}
		n = uint64(t)
	case string:
		parsed, err := strconv.ParseUint(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%q is not a number", t)
		}
		n = parsed
	default:
		return 0, fmt.Errorf("%v is not a number", v)
	}
	if n > max {
		return 0, fmt.Errorf("%d exceeds the maximum of %d", n, max)
	}
	return n, nil
}
