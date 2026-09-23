// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- local reservation authorization.
// Related: admission.go -- separate bandwidth admission decision.
package rsvpte

import (
	"fmt"
	"math"
	"net/netip"
	"strconv"
)

// reservationPolicy authorizes network addresses for local RSVP reservations.
// It does not authenticate users or interpret POLICY_DATA credentials. The zero
// value permits reservations, as configurations without a policy did before.
// Parsed policies are immutable and safe for concurrent use.
type reservationPolicy struct {
	rules       []reservationPolicyRule
	defaultDeny bool
}

type reservationPolicyRule struct {
	sender   netip.Prefix
	endpoint netip.Prefix
	deny     bool
}

// allows applies the first matching rule, in ascending numeric index order.
// Callers MUST also pass bandwidth admission before installing a reservation.
func (p reservationPolicy) allows(sender, endpoint netip.Addr) bool {
	for _, rule := range p.rules {
		if rule.sender.IsValid() {
			if !rule.sender.Contains(sender) {
				continue
			}
		}
		if rule.endpoint.IsValid() {
			if !rule.endpoint.Contains(endpoint) {
				continue
			}
		}
		return !rule.deny
	}
	return !p.defaultDeny
}

// parseReservationPolicy consumes the unwrapped rsvp-te root. It validates the
// keyed list before rsvpteList, which otherwise skips malformed entries.
func parseReservationPolicy(tree map[string]any) (reservationPolicy, error) {
	var policy reservationPolicy
	raw, exists := tree["reservation-policy"]
	if !exists {
		return policy, nil
	}
	settings, ok := raw.(map[string]any)
	if !ok {
		return reservationPolicy{}, fmt.Errorf("reservation-policy: expected a container")
	}
	for key := range settings {
		switch key {
		case "default-action", "rule":
		default:
			return reservationPolicy{}, fmt.Errorf("reservation-policy: unknown setting %q", key)
		}
	}
	if action, exists := settings["default-action"]; exists {
		deny, err := parseReservationPolicyAction(action)
		if err != nil {
			return reservationPolicy{}, fmt.Errorf("reservation-policy default-action: %w", err)
		}
		policy.defaultDeny = deny
	}
	raw, exists = settings["rule"]
	if !exists {
		return policy, nil
	}
	rules, ok := raw.(map[string]any)
	if !ok {
		return reservationPolicy{}, fmt.Errorf("reservation-policy rule: expected a keyed list")
	}
	for key, raw := range rules {
		index, err := parseReservationPolicyIndex(key)
		if err != nil {
			return reservationPolicy{}, fmt.Errorf("reservation-policy rule %q: %w", key, err)
		}
		rule, ok := raw.(map[string]any)
		if !ok {
			return reservationPolicy{}, fmt.Errorf("reservation-policy rule %q: expected a container", key)
		}
		if value, exists := rule["index"]; exists {
			embedded, err := parseReservationPolicyIndex(value)
			if err != nil {
				return reservationPolicy{}, fmt.Errorf("reservation-policy rule %q index: %w", key, err)
			}
			if embedded != index {
				return reservationPolicy{}, fmt.Errorf("reservation-policy rule %q: index disagrees with list key", key)
			}
		}
	}
	entries := rsvpteList(rules, true)
	policy.rules = make([]reservationPolicyRule, 0, len(entries))
	for _, entry := range entries {
		rule, err := parseReservationPolicyRule(entry.data)
		if err != nil {
			return reservationPolicy{}, fmt.Errorf("reservation-policy rule %q: %w", entry.key, err)
		}
		policy.rules = append(policy.rules, rule)
	}
	return policy, nil
}

func parseReservationPolicyRule(settings map[string]any) (reservationPolicyRule, error) {
	var rule reservationPolicyRule
	for key := range settings {
		switch key {
		case "index", "sender-prefix", "endpoint-prefix", "action":
		default:
			return reservationPolicyRule{}, fmt.Errorf("unknown setting %q", key)
		}
	}
	deny, err := parseReservationPolicyAction(settings["action"])
	if err != nil {
		return reservationPolicyRule{}, fmt.Errorf("action: %w", err)
	}
	rule.deny = deny
	if value, exists := settings["sender-prefix"]; exists {
		prefix, err := parseReservationPolicyPrefix(value)
		if err != nil {
			return reservationPolicyRule{}, fmt.Errorf("sender-prefix: %w", err)
		}
		rule.sender = prefix
	}
	if value, exists := settings["endpoint-prefix"]; exists {
		prefix, err := parseReservationPolicyPrefix(value)
		if err != nil {
			return reservationPolicyRule{}, fmt.Errorf("endpoint-prefix: %w", err)
		}
		rule.endpoint = prefix
	}
	return rule, nil
}

func parseReservationPolicyAction(value any) (bool, error) {
	switch value {
	case "permit":
		return false, nil
	case "deny":
		return true, nil
	default:
		return false, fmt.Errorf("expected permit or deny, got %v", value)
	}
}

// parseReservationPolicyIndex rejects alternate spellings so two map keys can
// never name the same position. uint16 also bounds the number of policy rules.
func parseReservationPolicyIndex(value any) (uint16, error) {
	index, ok := rsvpteNumber(value)
	if !ok || math.IsNaN(index) || index < 0 || index > math.MaxUint16 || index != math.Trunc(index) {
		return 0, fmt.Errorf("expected an integer index from 0 to 65535, got %v", value)
	}
	if text, ok := value.(string); ok {
		if text != strconv.FormatUint(uint64(index), 10) {
			return 0, fmt.Errorf("expected a canonical decimal index, got %q", text)
		}
	}
	return uint16(index), nil
}

func parseReservationPolicyPrefix(value any) (netip.Prefix, error) {
	text, ok := value.(string)
	if !ok {
		return netip.Prefix{}, fmt.Errorf("expected an IPv4 prefix, got %v", value)
	}
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid IPv4 prefix %q: %w", text, err)
	}
	if !prefix.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("expected an IPv4 prefix, got %q", text)
	}
	return prefix.Masked(), nil
}
