package irr

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestIRROwnerRegistered proves the owner is discoverable the way the engine
// finds it: by name (the consumers' Dependencies edge), by the system irr
// container (auto-load), and with the command set its Stage 1 declaration
// carries. Method: read the compiled registration init() filled.
func TestIRROwnerRegistered(t *testing.T) {
	reg := registry.Lookup(pluginName)
	if reg == nil {
		t.Fatalf("registry has no %q plugin", pluginName)
	}
	if !slices.Contains(reg.ConfigRoots, configPathIRR) {
		t.Errorf("ConfigRoots = %v, want it to hold %q so system irr auto-loads the owner", reg.ConfigRoots, configPathIRR)
	}
	if reg.RunEngine == nil {
		t.Error("RunEngine is nil: the owner would register and never run")
	}
	var declared []string
	for _, decl := range reg.Commands {
		declared = append(declared, decl.Name)
	}
	for _, want := range []string{cmdShowIRR, cmdUpdateIRRAll, cmdUpdateIRRAsn, cmdUpdateIRRAsSet, cmdClearIRRAsn, cmdClearIRRAsSet} {
		if !slices.Contains(declared, want) {
			t.Errorf("registration declares %v, missing %q", declared, want)
		}
	}
}

// TestIRROwnerGrantsItsKeysOnly proves the owner holds the write grant on the
// dataset, pointer, binding and generation keys and on the legacy
// meta/irr/{name} keys it converts and removes (AC-17), a read grant and no
// write grant on a consumer's interest key (the consumer is that key's only
// writer), and nothing on any other key. Method: ask the predicates the
// daemon's state dispatch asks (statestore.PluginKeyWritable for put, remove
// and increment, statestore.PluginKeyReadable for get).
func TestIRROwnerGrantsItsKeysOnly(t *testing.T) {
	// testDigest is a digest segment in the form a dataset key carries.
	const testDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, key := range []string{
		"meta/irr-data/whois.radb.net/AS-TEST/" + testDigest,
		"meta/irr-data/127.0.0.1:4343/RIPE::AS-TEST/" + testDigest,
		"meta/irr-set/AS-TEST",
		"meta/irr-asn/65001",
		"meta/irr-generation",
		"meta/irr/AS-TEST",
	} {
		if !statestore.PluginKeyWritable(pluginName, key) {
			t.Errorf("owner has no write grant on %s", key)
		}
	}
	interest := "meta/irr-interest/bgp-filter-irr"
	if !statestore.PluginKeyReadable(pluginName, interest) {
		t.Errorf("owner cannot read %s", interest)
	}
	if statestore.PluginKeyWritable(pluginName, interest) {
		t.Errorf("owner can write %s, which only its consumer writes", interest)
	}
	// The refusals name a key a live plugin owns, firewall-domain's stored DNS
	// answer: meta/bgp/irr-cache, which firewall-irr owned, is now owned by
	// nobody, so it alone would no longer show the owner refused another
	// plugin's key.
	for _, key := range []string{
		zefs.KeyFirewallDomainGroup.Key("office", "www.example.com", "ipv4"),
		"meta/bgp/irr-cache",
		"meta/irr-data/whois.radb.net",
		"meta/irr-data/whois.radb.net/AS-TEST",
	} {
		if statestore.PluginKeyReadable(pluginName, key) {
			t.Errorf("owner holds a grant on %s it must not hold", key)
		}
	}
}
