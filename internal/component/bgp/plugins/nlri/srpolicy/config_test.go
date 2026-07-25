package srpolicy

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// cr builds a ConfigRouteRequest for the SR-Policy parser, which only uses the
// NLRI content tokens plus next-hop.
func cr(content []string, nextHop string, isIPv6 bool) registry.ConfigRouteRequest {
	return registry.ConfigRouteRequest{Content: content, NextHop: nextHop, IsIPv6: isIPv6}
}

func TestParseConfigRoute_MPLS(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 binding-sid mpls 24000 segment-list weight 1 segment type-a mpls 16001")
	pr, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.IsIPv6 {
		t.Error("expected IPv4")
	}
	if len(pr.NLRI) == 0 {
		t.Fatal("empty NLRI")
	}
	if pr.NextHop != "192.0.2.1" {
		t.Errorf("next-hop = %q, want 192.0.2.1", pr.NextHop)
	}
	if len(pr.Attrs) != 1 {
		t.Fatalf("attrs count = %d, want 1", len(pr.Attrs))
	}
	if pr.Attrs[0].Code != 23 {
		t.Errorf("attr code = %d, want 23 (TunnelEncap)", pr.Attrs[0].Code)
	}
}

func TestParseConfigRoute_SRv6(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 1 color 200 endpoint 10.0.0.2 preference 200 srv6-binding-sid fc00::1 segment-list weight 2 segment type-b srv6 fc00::1 endpoint-behavior 65 32 0 16 0 policy-name my-policy candidate-path-name primary")
	pr, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.Attrs) != 1 {
		t.Fatalf("attrs count = %d, want 1", len(pr.Attrs))
	}
	if pr.Attrs[0].Code != 23 {
		t.Errorf("attr code = %d, want 23", pr.Attrs[0].Code)
	}
}

func TestParseConfigRoute_MultiSegList(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 300 endpoint 10.0.0.3 preference 300 segment-list weight 1 segment type-a mpls 16001 segment-list weight 2 segment type-a mpls 16002")
	pr, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.NLRI) == 0 {
		t.Fatal("empty NLRI")
	}
}

func TestParseConfigRoute_IPv6(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint 2001:db8::1 preference 100 segment-list weight 1 segment type-b srv6 2001:db8:1::1")
	pr, err := parseConfigRoute(cr(content, "2001:db8::2", true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pr.IsIPv6 {
		t.Error("expected IPv6")
	}
}

func TestParseConfigRoute_MissingEndpoint(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100")
	_, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
}

func TestParseConfigRoute_EmptyContent(t *testing.T) {
	t.Parallel()
	_, err := parseConfigRoute(cr(nil, "192.0.2.1", false))
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestParseConfigRoute_InvalidDistinguisher(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher notanumber color 100 endpoint 10.0.0.1")
	_, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err == nil {
		t.Fatal("expected error for invalid distinguisher")
	}
}

func TestParseConfigRoute_UnknownKeyword(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 bogus-keyword value")
	_, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err == nil {
		t.Fatal("expected error for unknown keyword")
	}
}

func TestParseConfigRoute_InvalidEndpoint(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint not-an-ip")
	_, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err == nil {
		t.Fatal("expected error for invalid endpoint")
	}
}

func TestParseConfigRoute_TruncatedBindingSID(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid")
	_, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err == nil {
		t.Fatal("expected error for truncated binding-sid")
	}
}

func TestParseConfigRoute_Priority(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 priority 10 segment-list weight 1 segment type-a mpls 16001")
	pr, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.Attrs) != 1 || pr.Attrs[0].Code != 23 {
		t.Fatalf("attrs = %v, want one TunnelEncap", pr.Attrs)
	}
}

func TestParseConfigRoute_BindingSIDNull(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 binding-sid null")
	pr, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.Attrs) != 1 || pr.Attrs[0].Code != 23 {
		t.Fatalf("attrs = %v, want one TunnelEncap", pr.Attrs)
	}
}

func TestParseConfigRoute_TruncatedEndpointBehavior(t *testing.T) {
	t.Parallel()
	content := strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 segment-list weight 1 segment type-b srv6 fc00::1 endpoint-behavior 65 32")
	_, err := parseConfigRoute(cr(content, "192.0.2.1", false))
	if err == nil {
		t.Fatal("expected error for truncated endpoint-behavior")
	}
}
