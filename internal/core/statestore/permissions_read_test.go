// Design: docs/architecture/api/process-protocol.md -- plugin state ownership.
package statestore

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// TestReadGrantRefusesWrite proves a read-only grant reads and cannot write,
// remove or increment, and never makes its holder an owner.
//
// Method: in an isolated registry, give the writer a write grant on the IRR
// pointer template and the reader a read grant on the same template. The
// reader's key is readable and not writable; the writer's is both; a key
// outside the template is neither for the reader; PluginKeyOwners names the
// writer alone. A later write grant over the reader's read grant widens it.
func TestReadGrantRefusesWrite(t *testing.T) {
	isolateGrants(t)
	RegisterPluginKeys("writer", zefs.KeyIRRSet)
	RegisterPluginReadKeys("reader", zefs.KeyIRRSet)
	key := zefs.KeyIRRSet.Key("AS-TEST")
	if !PluginKeyReadable("reader", key) {
		t.Fatalf("reader cannot read %s under its read grant", key)
	}
	if PluginKeyWritable("reader", key) {
		t.Fatalf("reader can write %s under a read-only grant", key)
	}
	if !PluginKeyReadable("writer", key) || !PluginKeyWritable("writer", key) {
		t.Fatalf("writer lost read or write on %s", key)
	}
	if PluginKeyReadable("reader", "meta/irr-asn/65001") {
		t.Fatal("reader reads a key outside its grant")
	}
	if got := PluginKeyOwners(key); !slices.Equal(got, []string{"writer"}) {
		t.Fatalf("PluginKeyOwners(%s) = %q, want [writer]: a read grant names no owner", key, got)
	}
	RegisterPluginKeys("reader", zefs.KeyIRRSet)
	if !PluginKeyWritable("reader", key) {
		t.Fatal("a write grant registered over a read grant did not widen it")
	}
}

// TestInstanceGrantAdmitsOnlyItsKey proves a grant on one instance of a
// template admits that key and no other instance, so one IRR consumer cannot
// write another consumer's interest key.
//
// Method: grant two consumers their own interest key; each reads and writes
// its own and neither reads nor writes the other's, and each key names its
// own consumer as its only owner.
func TestInstanceGrantAdmitsOnlyItsKey(t *testing.T) {
	isolateGrants(t)
	RegisterPluginKeyInstance("bgp-filter-irr", zefs.KeyIRRInterest, "bgp-filter-irr")
	RegisterPluginKeyInstance("firewall-irr", zefs.KeyIRRInterest, "firewall-irr")
	own := zefs.KeyIRRInterest.Key("bgp-filter-irr")
	other := zefs.KeyIRRInterest.Key("firewall-irr")
	if !PluginKeyReadable("bgp-filter-irr", own) || !PluginKeyWritable("bgp-filter-irr", own) {
		t.Fatalf("bgp-filter-irr cannot read and write its own %s", own)
	}
	if PluginKeyReadable("bgp-filter-irr", other) || PluginKeyWritable("bgp-filter-irr", other) {
		t.Fatalf("bgp-filter-irr reaches the other consumer's %s", other)
	}
	if got := PluginKeyOwners(other); !slices.Equal(got, []string{"firewall-irr"}) {
		t.Fatalf("PluginKeyOwners(%s) = %q, want [firewall-irr]", other, got)
	}
}
