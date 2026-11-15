// VALIDATES: spec-ospf-ext-14 AC-1/AC-2/AC-26, A-4, R-9 -- the new IPv4 (ze-ospf:show-*) and
// IPv6 (ze-ospf:show-ospfv3-*) deep-introspection proxies and the two inject proxies are
// registered and reachable; the v6 wire methods and command nouns do not collide with the
// v4 ones.
// PREVENTS: a command that registers no proxy (unreachable), or a v6 method that clobbers a
// v4 one in the shared internal/plugins/ospf package.
package ospf

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestShowOSPFv3RouteWired: spec-ospfv3-views-every-address-family wiring.
// VALIDATES: `show ospf ipv6 route` has a daemon proxy and a plugin command
// declaration of the same name, so the daemon forwards it to the plugin.
// METHOD: read the builtin RPC registry and commandDecls.
func TestShowOSPFv3RouteWired(t *testing.T) {
	m := registeredOSPFProxies(t)
	if got := m["ze-ospf:show-ospfv3-route"]; got != cmdShowIPv6Route {
		t.Fatalf("ze-ospf:show-ospfv3-route proxies %q, want %q", got, cmdShowIPv6Route)
	}
	if !slices.ContainsFunc(commandDecls(), func(decl sdk.CommandDecl) bool { return decl.Name == cmdShowIPv6Route }) {
		t.Fatalf("commandDecls does not declare %q, so the plugin never registers it", cmdShowIPv6Route)
	}
}

// TestAddressFamilyArgsForwardOneShape: spec-ospfv3-views-every-address-family,
// the contract between the daemon proxy and OnExecuteCommand.
// VALIDATES: an OSPFv3 view hands the plugin nothing, or `address-family <af>`,
// whether the operator typed the keyword or the bare value, and refuses a token
// no leaf took instead of forwarding or dropping it.
// METHOD: judge token lists with ValidateArgs against the leaf the model declares,
// then read what addressFamilyArgs answers.
func TestAddressFamilyArgsForwardOneShape(t *testing.T) {
	def, err := command.NewEnumArg(keywordAddressFamily,
		[]string{afNameIPv6Unicast, afNameIPv6Multicast, afNameIPv4Unicast, afNameIPv4Multicast}, command.ArgOptions{})
	if err != nil {
		t.Fatalf("build the address-family definition: %v", err)
	}
	defs := []command.ArgDef{def}
	cases := []struct {
		name    string
		typed   []string
		want    []string
		refused string
	}{
		{name: "absent", typed: nil, want: nil},
		{name: "keyword", typed: []string{keywordAddressFamily, afNameIPv4Unicast}, want: []string{keywordAddressFamily, afNameIPv4Unicast}},
		{name: "bare value", typed: []string{afNameIPv4Unicast}, want: []string{keywordAddressFamily, afNameIPv4Unicast}},
		{name: "keyword then stray", typed: []string{keywordAddressFamily, afNameIPv4Unicast, "detail"}, refused: `"detail"`},
		{name: "bare value then stray", typed: []string{afNameIPv4Unicast, "detail"}, refused: `"ipv4-unicast detail"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			validated, err := command.ValidateArgs(c.typed, defs, nil)
			if err != nil {
				t.Fatalf("ValidateArgs(%q): %v", c.typed, err)
			}
			got, refusal := addressFamilyArgs(cmdShowIPv6Neighbor, validated)
			if c.refused != "" {
				if refusal == nil {
					t.Fatalf("%q was forwarded as %q, want a refusal", c.typed, got)
				}
				if !strings.Contains(refusal.Error(), c.refused) {
					t.Fatalf("refusal %q does not name %s", refusal.Error(), c.refused)
				}
				if !strings.Contains(refusal.Error(), "show ospf ipv6 instance") {
					t.Fatalf("refusal %q does not name show ospf ipv6 instance", refusal.Error())
				}
				return
			}
			if refusal != nil {
				t.Fatalf("%q refused: %v", c.typed, refusal)
			}
			if !slices.Equal(got, c.want) {
				t.Fatalf("%q forwards %q, want %q", c.typed, got, c.want)
			}
		})
	}
}

func registeredOSPFProxies(t *testing.T) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, r := range pluginserver.AllBuiltinRPCs() {
		m[r.WireMethod] = r.PluginCommand
	}
	return m
}

func TestShowOSPFDatabaseDetailWired(t *testing.T) {
	m := registeredOSPFProxies(t)
	if got := m["ze-ospf:show-database-opaque-area-detail"]; got != "show ospf database opaque-area detail" {
		t.Fatalf("IPv4 detail proxy command = %q", got)
	}
	if got := m["ze-ospf:show-spf-detail"]; got != "show ospf spf detail" {
		t.Fatalf("IPv4 spf-detail proxy command = %q", got)
	}
}

func TestShowOSPFv3DatabaseDetailWired(t *testing.T) {
	m := registeredOSPFProxies(t)
	if got := m["ze-ospf:show-ospfv3-database-detail"]; got != "show ospf ipv6 database detail" {
		t.Fatalf("IPv6 detail proxy command = %q", got)
	}
	if got := m["ze-ospf:show-ospfv3-instance"]; got != "show ospf ipv6 instance" {
		t.Fatalf("IPv6 instance proxy command = %q", got)
	}
}

func TestDebugInjectWired(t *testing.T) {
	m := registeredOSPFProxies(t)
	if got := m["ze-ospf:debug-inject"]; got != "debug ip ospf inject opaque" {
		t.Fatalf("IPv4 inject proxy command = %q", got)
	}
	if got := m["ze-ospf:debug-ospfv3-inject"]; got != "debug ipv6 ospf inject lsa" {
		t.Fatalf("IPv6 inject proxy command = %q", got)
	}
}

func TestV3CommandsDistinctFromV4(t *testing.T) {
	m := registeredOSPFProxies(t)
	v4Commands := map[string]string{} // PluginCommand -> wire method
	var v6Methods []string
	for wm, cmd := range m {
		switch {
		case strings.HasPrefix(wm, "ze-ospf:show-ospfv3-"):
			v6Methods = append(v6Methods, wm)
			if !strings.HasPrefix(cmd, "show ospf ipv6") {
				t.Errorf("v6 method %q fronts a non-ipv6 command %q", wm, cmd)
			}
		case strings.HasPrefix(wm, "ze-ospf:show-"):
			v4Commands[cmd] = wm
		}
	}
	if len(v6Methods) == 0 {
		t.Fatalf("no ze-ospf:show-ospfv3-* methods registered")
	}
	// No v6 command noun may equal a v4 command noun.
	for wm, cmd := range m {
		if strings.HasPrefix(wm, "ze-show:ospfv3-") {
			if other, clash := v4Commands[cmd]; clash {
				t.Errorf("v6 command %q (%s) collides with v4 method %s", cmd, wm, other)
			}
		}
	}
}
