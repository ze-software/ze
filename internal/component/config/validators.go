// Design: docs/architecture/config/yang-config-design.md — custom validators
// Detail: validators_register.go — init registration of validators into registry
// Related: schema.go — schema types and validation

package config

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errPort0IsNotValidIn = errors.New("port 0 is not valid in port spec")

// baseSendTypes are the built-in send types (handled by dedicated bool fields
// in peer config, not plugin-registered). Used by SendMessageValidator for completion.
var baseSendTypes = []string{"update", "refresh"}

// ReceiveEventValidator returns a validator that checks if a value is a valid event type
// for the receive leaf-list. Queries the BGP event namespace at call time so it reflects
// plugin-registered event types (e.g., "update-rpki").
func ReceiveEventValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if !events.IsValidEvent(bgpevents.Namespace, str) {
				return fmt.Errorf("%q is not a valid receive event type (valid: %s)",
					str, events.ValidEventNames(bgpevents.Namespace))
			}
			return nil
		},
		CompleteFn: allBGPEventNames,
	}
}

// SendMessageValidator returns a validator that checks if a value is a valid send type.
// Base types (update, refresh) plus any plugin-registered send types.
func SendMessageValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if slices.Contains(baseSendTypes, str) {
				return nil
			}
			if events.IsValidSendType(str) {
				return nil
			}
			return fmt.Errorf("%q is not a valid send type (valid: %s)",
				str, allSendTypeNames())
		},
		CompleteFn: func() []string {
			names := append([]string{}, baseSendTypes...)
			extra := events.ValidSendTypeNames()
			if extra != "" {
				for part := range strings.SplitSeq(extra, ", ") {
					names = append(names, part)
				}
			}
			sort.Strings(names)
			return names
		},
	}
}

// allBGPEventNames returns sorted BGP event type names from the event registry.
func allBGPEventNames() []string {
	raw := events.ValidEventNames(bgpevents.Namespace)
	if raw == "" {
		return nil
	}
	var names []string
	for part := range strings.SplitSeq(raw, ", ") {
		names = append(names, part)
	}
	sort.Strings(names)
	return names
}

// allSendTypeNames returns a comma-separated string of all valid send types
// (base + plugin-registered) for error messages.
func allSendTypeNames() string {
	names := append([]string{}, baseSendTypes...)
	extra := events.ValidSendTypeNames()
	if extra != "" {
		for part := range strings.SplitSeq(extra, ", ") {
			names = append(names, part)
		}
	}
	sort.Strings(names)
	return textbuf.Join(names, ", ")
}

// AddressFamilyValidator returns a validator that checks if a value is a registered address family.
// Queries the plugin registry at validation time (not creation time) so it reflects
// whatever families are currently registered.
func AddressFamilyValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			families := registry.FamilyMap()
			if _, registered := families[str]; !registered {
				return fmt.Errorf("%q is not a registered address family", str)
			}
			return nil
		},
		CompleteFn: func() []string {
			families := registry.FamilyMap()
			names := make([]string, 0, len(families))
			for name := range families {
				names = append(names, name)
			}
			sort.Strings(names)
			return names
		},
	}
}

// NonzeroIPv4Validator returns a validator that accepts valid IPv4 addresses
// except 0.0.0.0. Combine with LiteralSelfValidator via "|" for next-hop leaves.
func NonzeroIPv4Validator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			addr, err := netip.ParseAddr(str)
			if err != nil || !addr.Is4() {
				return fmt.Errorf("%q is not a valid IPv4 address for %s", str, path)
			}
			if addr == netip.IPv4Unspecified() {
				return fmt.Errorf("0.0.0.0 is not valid for %s", path)
			}
			return nil
		},
	}
}

// LiteralSelfValidator returns a validator that accepts only the literal string "self".
func LiteralSelfValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(_ string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if str == "self" {
				return nil
			}
			return fmt.Errorf("%q is not \"self\"", str)
		},
	}
}

// macPattern validates MAC address format (xx:xx:xx:xx:xx:xx).
var macPattern = regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)

// MACAddressValidator returns a validator for MAC address fields.
// CompleteFn is registered separately by the iface package via
// yang.RegisterCompleteFn to avoid config importing iface.
func MACAddressValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(_ string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if !macPattern.MatchString(str) {
				return fmt.Errorf("%q is not a valid MAC address (expected xx:xx:xx:xx:xx:xx)", str)
			}
			return nil
		},
	}
}

// isisSystemIDPattern validates the IS-IS System ID dotted-hex form
// xxxx.xxxx.xxxx (6 octets = three 4-hex-digit groups). RFC 1195 / ISO/IEC
// 10589 section 1.4: the System ID is a fixed 6-octet field.
var isisSystemIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{4}\.[0-9a-fA-F]{4}\.[0-9a-fA-F]{4}$`)

// ISISNETValidator validates an IS-IS Network Entity Title in dotted-hex text
// form (e.g. 49.0001.0000.0000.0001.00). Inline (no isis import, to avoid a
// cycle since the isis component imports config), mirroring the mac-address
// precedent; the isis component registers the CompleteFn via
// yang.RegisterCompleteFn.
//
// ISO/IEC 10589 section 6.2: a NET is an Area Address (1..13 octets) followed by
// the 6-octet System ID and a 1-octet NSEL (0x00 for an IS). Total 8..20 octets.
func ISISNETValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			n, err := isisDecodeNETLen(str)
			if err != nil {
				return fmt.Errorf("%q is not a valid IS-IS NET for %s: %w", str, path, err)
			}
			// ISO/IEC 10589 section 6.2: 1..13 area + 6 system-id + 1 NSEL = 8..20.
			if n < 8 || n > 20 {
				return fmt.Errorf("%q is not a valid IS-IS NET for %s: length %d octets, want 8..20", str, path, n)
			}
			return nil
		},
	}
}

// isisDecodeNETLen decodes the dotted-hex NET groups and returns the total octet
// count, rejecting odd nibble counts and non-hex digits. It does not allocate a
// byte buffer; it counts octets while validating each group is whole octets.
func isisDecodeNETLen(s string) (int, error) {
	if s == "" {
		return 0, errEmptyNET
	}
	total := 0
	for group := range strings.SplitSeq(s, ".") {
		if group == "" {
			return 0, errEmptyNETGroup
		}
		if len(group)%2 != 0 {
			return 0, errOddNETGroup
		}
		for i := range len(group) {
			c := group[i]
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return 0, errBadNETHex
			}
		}
		total += len(group) / 2
	}
	return total, nil
}

var (
	errEmptyNET      = errors.New("empty NET")
	errEmptyNETGroup = errors.New("empty group in NET")
	errOddNETGroup   = errors.New("NET group has an odd number of hex digits")
	errBadNETHex     = errors.New("NET contains a non-hex digit")
)

// ISISSystemIDValidator validates the IS-IS System ID dotted-hex form
// xxxx.xxxx.xxxx. The YANG leaf also carries the same pattern, so this is the
// custom-validator hook for completion (the isis component registers the
// CompleteFn via yang.RegisterCompleteFn).
func ISISSystemIDValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if !isisSystemIDPattern.MatchString(str) {
				return fmt.Errorf("%q is not a valid IS-IS system-id for %s (expected xxxx.xxxx.xxxx)", str, path)
			}
			return nil
		},
	}
}

// ospfRouterIDValidator validates the OSPFv2 Router ID dotted-quad form. It
// lives in config, not the ospf component, to avoid an import cycle; the ospf
// package registers the CompleteFn through yang.RegisterCompleteFn.
func ospfRouterIDValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			addr, err := netip.ParseAddr(str)
			if err != nil || !addr.Is4() || addr.IsUnspecified() {
				return fmt.Errorf("%q is not a valid OSPF router-id for %s (expected non-zero dotted-quad IPv4)", str, path)
			}
			return nil
		},
	}
}

// ospfAreaIDValidator validates an OSPFv2 Area ID as either dotted quad or a
// decimal uint32, matching the operator forms accepted by OSPF CLIs.
func ospfAreaIDValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if str == "" {
				return fmt.Errorf("empty OSPF area-id for %s", path)
			}
			if strings.Contains(str, ".") {
				addr, err := netip.ParseAddr(str)
				if err != nil || !addr.Is4() {
					return fmt.Errorf("%q is not a valid OSPF area-id for %s (expected dotted-quad IPv4 or uint32)", str, path)
				}
				return nil
			}
			if _, err := strconv.ParseUint(str, 10, 32); err != nil {
				return fmt.Errorf("%q is not a valid OSPF area-id for %s (expected dotted-quad IPv4 or uint32)", str, path)
			}
			return nil
		},
	}
}

// IPv4AddressValidator returns a validator that accepts valid IPv4 addresses.
func IPv4AddressValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			addr, err := netip.ParseAddr(str)
			if err != nil || !addr.Is4() {
				return fmt.Errorf("%q is not a valid IPv4 address for %s", str, path)
			}
			return nil
		},
	}
}

// IPv6AddressValidator returns a validator that accepts valid IPv6 addresses.
func IPv6AddressValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if _, err := netip.ParseAddr(str); err != nil {
				return fmt.Errorf("%q is not a valid IPv6 address for %s", str, path)
			}
			return nil
		},
	}
}

// IPv4PrefixValidator returns a validator that accepts valid IPv4 CIDR prefixes.
func IPv4PrefixValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			pfx, err := netip.ParsePrefix(str)
			if err != nil {
				return fmt.Errorf("%q is not a valid IPv4 prefix for %s: %w", str, path, err)
			}
			if !pfx.Addr().Is4() {
				return fmt.Errorf("%q is not an IPv4 prefix for %s", str, path)
			}
			return nil
		},
	}
}

// IPv6PrefixValidator returns a validator that accepts valid IPv6 CIDR prefixes.
func IPv6PrefixValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if _, err := netip.ParsePrefix(str); err != nil {
				return fmt.Errorf("%q is not a valid IPv6 prefix for %s: %w", str, path, err)
			}
			return nil
		},
	}
}

// SetRefValidator returns a validator that accepts @set-name references to firewall sets.
func SetRefValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if !strings.HasPrefix(str, "@") || len(str) < 2 {
				return fmt.Errorf("%q is not a valid set reference (must start with @)", str)
			}
			return nil
		},
	}
}

var portSpecSetRefPattern = regexp.MustCompile(`^@[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// PortSpecValidator returns a validator for firewall/policy-route port specs.
// Accepts a whole named-set reference (@set-name) or comma-separated ports and
// ranges. Numeric ports are real TCP/UDP ports, so 0 is rejected here.
func PortSpecValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if str == "" {
				return fmt.Errorf("empty port spec for %s", path)
			}
			if portSpecSetRefPattern.MatchString(str) {
				return nil
			}
			if strings.HasPrefix(str, "@") {
				return fmt.Errorf("%q is not a valid port set reference", str)
			}

			count := 0
			for entry := range strings.SplitSeq(str, ",") {
				count++
				if count > 128 {
					return fmt.Errorf("port spec %q has more than 128 entries", str)
				}
				if entry == "" || strings.TrimSpace(entry) != entry {
					return fmt.Errorf("invalid empty or spaced port entry %q in %q", entry, str)
				}
				if loStr, hiStr, ok := strings.Cut(entry, "-"); ok {
					lo, err := parsePortSpecNumber(loStr)
					if err != nil {
						return err
					}
					hi, err := parsePortSpecNumber(hiStr)
					if err != nil {
						return err
					}
					if hi < lo {
						return fmt.Errorf("inverted port range %q", entry)
					}
					continue
				}
				if _, err := parsePortSpecNumber(entry); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func parsePortSpecNumber(s string) (uint16, error) {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	if n == 0 {
		return 0, errPort0IsNotValidIn
	}
	return uint16(n), nil
}

// RedistributeSourceValidator returns a validator for redistribute source names.
// Validates against the central redistribute source registry. Each protocol
// component registers its own sources (e.g., BGP registers ibgp/ebgp).
func RedistributeSourceValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if _, found := redistribute.LookupSource(str); !found {
				return fmt.Errorf("%q is not a registered redistribute source", str)
			}
			return nil
		},
		CompleteFn: redistribute.SourceNames,
	}
}

func sortedInternalPluginNames() []string {
	// registry.Names already returns a freshly-allocated, alphabetically
	// sorted slice, so no further sorting is needed here.
	return registry.Names()
}

// InternalPluginNameValidator returns a validator for the internal plugin `use` leaf.
// Validates against registered built-in plugin names from the plugin registry.
func InternalPluginNameValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			if !registry.Has(str) {
				return fmt.Errorf("%q is not a registered internal plugin (available: %s)",
					str, textbuf.Join(sortedInternalPluginNames(), ", "))
			}
			return nil
		},
		CompleteFn: sortedInternalPluginNames,
	}
}

// CommunityRangeValidator returns a validator that checks BGP community values.
// Accepts well-known names (no-export, blackhole, graceful-shutdown, etc.),
// ASN:value format (both parts uint16 0-65535), hex (0xNNNNNNNN), and bare uint32.
func CommunityRangeValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			_, err := attribute.ParseCommunity(str)
			if err != nil {
				return fmt.Errorf("invalid community %q: %w", str, err)
			}
			return nil
		},
		CompleteFn: attribute.WellKnownCommunityNames,
	}
}
