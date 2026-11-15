// Design: docs/architecture/irr-owner.md -- the consumer's state grants
package filter_irr

import (
	"testing"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestFilterIRRGrantsReadOwnerWriteOwnInterest proves the filter reads every
// key the irr plugin writes and writes none of them, and writes its own
// interest key and no other consumer's.
//
// Method: ask the predicates the daemon's state dispatch asks
// (statestore.PluginKeyReadable for get and list, statestore.PluginKeyWritable
// for put, remove and increment) about each key, as registered by init.
func TestFilterIRRGrantsReadOwnerWriteOwnInterest(t *testing.T) {
	for _, key := range []string{
		zefs.KeyIRRDataset.Key("whois.radb.net", "AS-TEST"),
		zefs.KeyIRRSet.Key("AS-TEST"),
		zefs.KeyIRRASN.Key("65001"),
		zefs.KeyIRRGeneration.Key(),
	} {
		if !statestore.PluginKeyReadable(pluginName, key) {
			t.Errorf("%s cannot read the owner's %s", pluginName, key)
		}
		if statestore.PluginKeyWritable(pluginName, key) {
			t.Errorf("%s can write the owner's %s", pluginName, key)
		}
	}
	own := zefs.KeyIRRInterest.Key(pluginName)
	if !statestore.PluginKeyWritable(pluginName, own) {
		t.Errorf("%s cannot write its own %s", pluginName, own)
	}
	other := zefs.KeyIRRInterest.Key("firewall-irr")
	if statestore.PluginKeyReadable(pluginName, other) {
		t.Errorf("%s reaches another consumer's %s", pluginName, other)
	}
}
