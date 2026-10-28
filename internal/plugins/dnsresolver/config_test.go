package dnsresolver

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/plugins/dnsresolver/yang"
)

// parseDelivered runs text through the pipeline the daemon delivers a
// plugin's config with (File, Tree, Tree.ToPluginMap, JSON) and parses the
// result the way OnConfigure does.
func parseDelivered(t *testing.T, text string) (resolverConfig, error) {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, map[string]string{
		"ze-dns-resolver-conf.yang": yang.ZeDNSResolverConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}
	data, err := json.Marshal(tree.ToPluginMap())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return parseConfig(string(data))
}

// VALIDATES: forward servers come out in the order the operator wrote them,
// which is the failover order, for both orders of the same two addresses.
// METHOD: parse two configs through the daemon's delivery pipeline, one
// writing 192.0.2.20 then 192.0.2.10 (against key order), one the reverse.
// PREVENTS: a parse that sorts by address, making the operator's primary
// upstream whichever address sorts first, with no message anywhere.
func TestParseForwardKeepsConfiguredOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first string
		then  string
	}{
		{"against key order", "192.0.2.20", "192.0.2.10"},
		{"in key order", "192.0.2.10", "192.0.2.20"},
	} {
		text := "service {\n dns-resolver {\n  forward {\n" +
			"   server " + tc.first + " {\n    port 5300\n   }\n" +
			"   server " + tc.then + " {\n    port 5301\n   }\n" +
			"  }\n }\n}\n"
		cfg, err := parseDelivered(t, text)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		want := []upstream{
			{IP: netip.MustParseAddr(tc.first), Port: 5300},
			{IP: netip.MustParseAddr(tc.then), Port: 5301},
		}
		if len(cfg.Forward) != len(want) {
			t.Fatalf("%s: %d forward servers, want %d", tc.name, len(cfg.Forward), len(want))
		}
		for i := range want {
			if cfg.Forward[i] != want[i] {
				t.Errorf("%s: forward[%d] = %v, want %v", tc.name, i, cfg.Forward[i], want[i])
			}
		}
	}
}

// VALIDATES: a multi-entry forward list that arrives with no order is
// refused, naming the list, rather than read in an order nobody chose.
// METHOD: hand the parser a two-server list with no reserved order key, the
// shape Tree.ToMap (not ToPluginMap) would produce.
// PREVENTS: a lowering defect silently swapping primary and backup.
func TestParseForwardRefusesUnorderedList(t *testing.T) {
	data := `{"service":{"dns-resolver":{"forward":{"server":{"192.0.2.10":{},"192.0.2.20":{}}}}}}`
	_, err := parseConfig(data)
	if err == nil {
		t.Fatal("two forward servers with no order parsed, want a refusal")
	}
	if !strings.Contains(err.Error(), "forward") || !strings.Contains(err.Error(), "no order") {
		t.Errorf("refusal %q names neither the forward list nor the missing order", err)
	}
}

// VALIDATES: AC-3 for every spelling of a wildcard listener address,
// the IPv4-mapped any address included.
// METHOD: parse one listener entry per spelling and require the wildcard
// refusal; a loopback address in each family is accepted.
// PREVENTS: `ip ::ffff:0:0`, which Go binds as 0.0.0.0, slipping past a
// check that asks netip whether the mapped form is unspecified.
func TestParseListenerRefusesEveryWildcard(t *testing.T) {
	for _, ip := range []string{"0.0.0.0", "::", "::ffff:0:0", "::ffff:0.0.0.0"} {
		_, err := parseListener("lan", map[string]any{"ip": ip})
		if err == nil {
			t.Fatalf("listener ip %s accepted, want the wildcard refusal", ip)
		}
		if !strings.Contains(err.Error(), "wildcard") {
			t.Fatalf("listener ip %s: %v, want the wildcard refusal", ip, err)
		}
	}
	for _, ip := range []string{"127.0.0.1", "::1"} {
		if _, err := parseListener("lan", map[string]any{"ip": ip}); err != nil {
			t.Fatalf("listener ip %s: %v, want accepted", ip, err)
		}
	}
}
