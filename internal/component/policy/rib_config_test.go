package policy_test

import (
	"encoding/json"
	"net/netip"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
	_ "github.com/ze-software/ze/internal/component/sysrib"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	_ "github.com/ze-software/ze/internal/plugins/connected"
	_ "github.com/ze-software/ze/internal/plugins/isis"
	_ "github.com/ze-software/ze/internal/plugins/ospf"
	_ "github.com/ze-software/ze/internal/plugins/static"
	staticevents "github.com/ze-software/ze/internal/plugins/static/events"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// configSections parses text through the registered YANG modules and hands
// the policy plugin each root it reads, as a commit delivers them.
func configSections(t *testing.T, text string) []sdk.ConfigSection {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var sections []sdk.ConfigSection
	for _, root := range append([]string{policy.RootName}, policy.RIBImportRoots()...) {
		sub := tree.GetContainer(root)
		if sub == nil {
			continue
		}
		data, err := json.Marshal(map[string]any{root: sub.ToPluginMap()})
		if err != nil {
			t.Fatal(err)
		}
		sections = append(sections, sdk.ConfigSection{Root: root, Data: string(data)})
	}
	return sections
}

const ribPolicy = `
policy {
	prefix-list BOGONS {
		entry 192.0.2.0/24 { action reject; }
		entry 0.0.0.0/0 { le 32; action accept; }
	}
	prefix-list ALL {
		entry 0.0.0.0/0 { le 32; action accept; }
	}
}
`

// TestConfigRIBImportEachAttachment: every Loc-RIB attachment parses a
// `filter { import { ... } }` chain through its real YANG, and the policy
// plugin's transaction accepts it.
func TestConfigRIBImportEachAttachment(t *testing.T) {
	t.Cleanup(func() { ribpolicy.Set(nil) })
	for _, root := range []string{"isis", "ospf", "static", "connected", "rib"} {
		t.Run(root, func(t *testing.T) {
			text := ribPolicy + root + " {\n\tfilter {\n\t\timport {\n\t\t\tfilters [ BOGONS ];\n\t\t\tfinal accept;\n\t\t}\n\t}\n}\n"
			if err := policy.ApplyRIBImportsForTest(configSections(t, text)); err != nil {
				t.Fatalf("%s filter import refused: %v", root, err)
			}
			if ribpolicy.Current() == nil {
				t.Fatalf("%s filter import published no snapshot", root)
			}
		})
	}
}

// TestConfigRIBCentralChainHidesStaticThenReadmits is the config-level wiring
// test: `rib { filter { import ... } }` reaches the published snapshot and
// hides a matching static path (AC-7 at config level); a commit that relaxes
// the chain re-admits it with no new insert (AC-5 at config level), and a
// config with no chain publishes nil.
func TestConfigRIBCentralChainHidesStaticThenReadmits(t *testing.T) {
	rib := locrib.Default()
	if rib == nil {
		t.Skip("forked plugin process: no Loc-RIB")
	}
	t.Cleanup(func() { locrib.PublishPolicy(nil) })
	prefix := netip.MustParsePrefix("192.0.2.0/24")

	strict := ribPolicy + "rib {\n\tfilter {\n\t\timport {\n\t\t\tfilters [ BOGONS ];\n\t\t\tfinal accept;\n\t\t}\n\t}\n}\n"
	if err := policy.ApplyRIBImportsForTest(configSections(t, strict)); err != nil {
		t.Fatal(err)
	}
	rib.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: staticevents.ProtocolID, NextHop: netip.MustParseAddr("198.51.100.1")})
	t.Cleanup(func() { rib.Remove(family.IPv4Unicast, prefix, staticevents.ProtocolID, 0) })
	if best, ok := rib.Best(family.IPv4Unicast, prefix); ok {
		t.Fatalf("rib import BOGONS left the static path selected: %+v", best)
	}

	relaxed := ribPolicy + "rib {\n\tfilter {\n\t\timport {\n\t\t\tfilters [ ALL ];\n\t\t\tfinal accept;\n\t\t}\n\t}\n}\n"
	if err := policy.ApplyRIBImportsForTest(configSections(t, relaxed)); err != nil {
		t.Fatal(err)
	}
	if _, ok := rib.Best(family.IPv4Unicast, prefix); !ok {
		t.Fatal("relaxing the rib chain did not re-admit the hidden static path")
	}

	if err := policy.ApplyRIBImportsForTest(configSections(t, ribPolicy)); err != nil {
		t.Fatal(err)
	}
	if ribpolicy.Current() != nil {
		t.Fatal("a config with no import chain must publish the nil snapshot")
	}
}
