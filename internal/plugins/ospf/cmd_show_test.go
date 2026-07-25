// VALIDATES: spec-ospf-ext-14 AC-1/AC-2/AC-26, A-4, R-9 -- the new IPv4 (ze-show:ospf-*) and
// IPv6 (ze-show:ospfv3-*) deep-introspection proxies and the two inject proxies are
// registered and reachable; the v6 wire methods and command nouns do not collide with the
// v4 ones.
// PREVENTS: a command that registers no proxy (unreachable), or a v6 method that clobbers a
// v4 one in the shared internal/plugins/ospf package.
package ospf

import (
	"strings"
	"testing"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

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
	if got := m["ze-show:ospf-database-opaque-area-detail"]; got != "show ospf database opaque-area detail" {
		t.Fatalf("IPv4 detail proxy command = %q", got)
	}
	if got := m["ze-show:ospf-spf-detail"]; got != "show ospf spf detail" {
		t.Fatalf("IPv4 spf-detail proxy command = %q", got)
	}
}

func TestShowOSPFv3DatabaseDetailWired(t *testing.T) {
	m := registeredOSPFProxies(t)
	if got := m["ze-show:ospfv3-database-detail"]; got != "show ospf ipv6 database detail" {
		t.Fatalf("IPv6 detail proxy command = %q", got)
	}
	if got := m["ze-show:ospfv3-instance"]; got != "show ospf ipv6 instance" {
		t.Fatalf("IPv6 instance proxy command = %q", got)
	}
}

func TestDebugInjectWired(t *testing.T) {
	m := registeredOSPFProxies(t)
	if got := m["ze-debug:ospf-inject"]; got != "debug ip ospf inject opaque" {
		t.Fatalf("IPv4 inject proxy command = %q", got)
	}
	if got := m["ze-debug:ospfv3-inject"]; got != "debug ipv6 ospf inject lsa" {
		t.Fatalf("IPv6 inject proxy command = %q", got)
	}
}

func TestV3CommandsDistinctFromV4(t *testing.T) {
	m := registeredOSPFProxies(t)
	v4Commands := map[string]string{} // PluginCommand -> wire method
	var v6Methods []string
	for wm, cmd := range m {
		switch {
		case strings.HasPrefix(wm, "ze-show:ospfv3-"):
			v6Methods = append(v6Methods, wm)
			if !strings.HasPrefix(cmd, "show ospf ipv6") {
				t.Errorf("v6 method %q fronts a non-ipv6 command %q", wm, cmd)
			}
		case strings.HasPrefix(wm, "ze-show:ospf-"):
			v4Commands[cmd] = wm
		}
	}
	if len(v6Methods) == 0 {
		t.Fatalf("no ze-show:ospfv3-* methods registered")
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
