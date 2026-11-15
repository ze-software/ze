// Design: docs/architecture/irr-owner.md -- the consumer's state grants
package irr

import (
	"testing"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestFirewallIRRGrantsReadOwnerWriteOwnInterest proves firewall-irr reads
// every key the irr plugin writes and writes none of them, and writes its own
// interest key and no other consumer's.
//
// Method: ask the predicates the daemon's state dispatch asks
// (statestore.PluginKeyReadable for get and list, statestore.PluginKeyWritable
// for put, remove and increment) about each key, as registered by init.
func TestFirewallIRRGrantsReadOwnerWriteOwnInterest(t *testing.T) {
	const consumer = "firewall-irr"
	for _, key := range []string{
		zefs.KeyIRRDataset.Key("whois.radb.net", "AS-TEST", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
		zefs.KeyIRRSet.Key("AS-TEST"),
		zefs.KeyIRRASN.Key("65001"),
		zefs.KeyIRRGeneration.Key(),
	} {
		if !statestore.PluginKeyReadable(consumer, key) {
			t.Errorf("%s cannot read the owner's %s", consumer, key)
		}
		if statestore.PluginKeyWritable(consumer, key) {
			t.Errorf("%s can write the owner's %s", consumer, key)
		}
	}
	own := zefs.KeyIRRInterest.Key(consumer)
	if !statestore.PluginKeyWritable(consumer, own) {
		t.Errorf("%s cannot write its own %s", consumer, own)
	}
	other := zefs.KeyIRRInterest.Key("bgp-filter-irr")
	if statestore.PluginKeyReadable(consumer, other) {
		t.Errorf("%s reaches another consumer's %s", consumer, other)
	}
}

// TestFirewallIRRHoldsNoLegacyKey proves the plugin no longer keeps lists of
// its own: it holds no grant on the per-name keys it once wrote, which the irr
// plugin converts and removes (AC-17), nor on the old BGP cache blob.
//
// Method: ask the same predicates about one key of each legacy pattern.
func TestFirewallIRRHoldsNoLegacyKey(t *testing.T) {
	const consumer = "firewall-irr"
	for _, key := range []string{
		zefs.KeyIRRPrefixCache.Key("AS-TEST"),
		"meta/bgp/irr-cache",
	} {
		if statestore.PluginKeyReadable(consumer, key) {
			t.Errorf("%s still holds a grant on the legacy %s", consumer, key)
		}
	}
}
