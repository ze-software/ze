package flowspec

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// A partly readable expression cannot be installed as the readable subset:
// removing a malformed AND operand broadens the configured filter.
func TestConfigFlowSpecRejectsPartialCriteria(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		values    []string
	}{
		{"port-and", kwDestPort, []string{">80&<oops"}},
		{"port-list", kwPort, []string{"80", "oops"}},
		{"protocol-list", kwProtocol, []string{"tcp", "unknown"}},
		{"dscp-list", kwDSCP, []string{"46", "unknown"}},
		{"icmp-type-list", kwICMPType, []string{"echo-request", "unknown"}},
		{"icmp-code-list", kwICMPCode, []string{"port-unreachable", "unknown"}},
		{"tcp-flags-and", kwTCPFlags, []string{"syn&unknown"}},
		{"fragment-list", kwFragment, []string{"first-fragment", "unknown"}},
		{"multiple-prefixes", kwDestinationIPv4, []string{"10.0.0.0/8", "192.0.2.0/24"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := []string{kwSourceIPv4, "192.0.2.0/24", tc.key, "["}
			content = append(content, tc.values...)
			content = append(content, "]")
			if _, err := parseConfigRoute(registry.ConfigRouteRequest{Content: content}); err == nil {
				t.Fatal("partly invalid criterion was accepted")
			}
		})
	}
	_, err := parseConfigRoute(registry.ConfigRouteRequest{
		IsIPv6:  true,
		Content: []string{kwSourceIPv6, "2001:db8::/32", kwFlowLabel, "[", "100", "bad", "]"},
	})
	if err == nil {
		t.Fatal("partial IPv6 flow label accepted")
	}
}

func TestConfigFlowSpecRejectsMissingCriteriaValues(t *testing.T) {
	for _, suffix := range [][]string{
		{kwDestPort},
		{kwDestPort, "[", "]"},
	} {
		content := append([]string{kwDestinationIPv4, "192.0.2.0/24"}, suffix...)
		if _, err := parseConfigRoute(registry.ConfigRouteRequest{Content: content}); err == nil {
			t.Fatalf("missing criterion values widened the route: %v", content)
		}
	}
}

// The enclosing family determines prefix layout. A mismatched address must be
// rejected before encoding, never decoded later as unrelated component bytes.
func TestFlowSpecRejectsPrefixFamilyMismatch(t *testing.T) {
	for _, tc := range []struct {
		family Family
		prefix netip.Prefix
	}{
		{IPv4FlowSpec, netip.MustParsePrefix("2001:db8::/32")},
		{IPv6FlowSpec, netip.MustParsePrefix("192.0.2.0/24")},
		{IPv4FlowSpec, netip.Prefix{}},
	} {
		fs := NewFlowSpec(tc.family)
		if err := fs.AddComponent(NewFlowDestPrefixComponent(tc.prefix)); err == nil {
			t.Fatalf("accepted prefix %s in %s", tc.prefix, tc.family)
		}
		if len(fs.Components()) != 0 {
			t.Fatal("rejected component was retained")
		}
	}
}

func TestIPv4FlowSpecRejectsIPv6FlowLabel(t *testing.T) {
	fs := NewFlowSpec(IPv4FlowSpec)
	if err := fs.AddComponent(NewFlowFlowLabelComponent(1)); err == nil {
		t.Fatal("IPv6-only flow label accepted in IPv4 builder")
	}
	if _, err := ParseFlowSpec(IPv4FlowSpec, []byte{3, 13, 0x81, 1}); err == nil {
		t.Fatal("IPv6-only flow label accepted from IPv4 wire")
	}
	if _, err := ParseFlowSpec(IPv6FlowSpec, []byte{3, 13, 0x81, 1}); err != nil {
		t.Fatalf("valid IPv6 flow label refused: %v", err)
	}
}
