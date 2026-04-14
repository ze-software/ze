// Design: docs/architecture/config/yang-config-design.md — custom validators
// Detail: validators_register.go — init registration of validators into registry
// Related: schema.go — schema types and validation

package config

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
)

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
	return strings.Join(names, ", ")
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
			ip := net.ParseIP(str)
			if ip == nil {
				return fmt.Errorf("%q is not a valid IPv4 address for %s", str, path)
			}
			if ip.To4() == nil {
				return fmt.Errorf("%q is not a valid IPv4 address for %s", str, path)
			}
			if ip.Equal(net.IPv4zero) {
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

// CommunityRangeValidator returns a validator that checks BGP community ASN:value ranges.
// Both parts must be uint16 (0-65535).

func CommunityRangeValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(path string, value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", value)
			}
			parts := strings.SplitN(str, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("community %q must be in ASN:value format", str)
			}
			if _, err := strconv.ParseUint(parts[0], 10, 16); err != nil {
				return fmt.Errorf("community ASN part %q exceeds uint16 range (0-65535)", parts[0])
			}
			if _, err := strconv.ParseUint(parts[1], 10, 16); err != nil {
				return fmt.Errorf("community value part %q exceeds uint16 range (0-65535)", parts[1])
			}
			return nil
		},
	}
}
