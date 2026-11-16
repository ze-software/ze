// Design: docs/architecture/behavior/peer-lifecycle.md -- reload judges the OPEN actually sent.
// Related: session_negotiate.go -- prepareOpen; peer_settings_negotiation.go -- pluginCapabilitiesKept.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestPrepareOpenKeepsPluginSnapshotAcrossRollback proves a reload's restoration
// is judged against the plugin bytes actually placed in OPEN. Method: publish B
// immediately after the first atomic store read returns A, build OPEN through
// the real Peer getters, then restore A before the reload probe. A coherent
// build may carry either generation; the recorded declaration and keep decision
// MUST agree with whichever generation its wire bytes actually carry.
func TestPrepareOpenKeepsPluginSnapshotAcrossRollback(t *testing.T) {
	settings := negotiationSettings(capIPv4())
	r, peer := newSwapTestReactor(t, settings, settings)
	const code = 200
	a := plugin.InjectedCapability{Code: code, Value: []byte{0x11}, Plugin: "snapshot-test"}
	b := plugin.InjectedCapability{Code: code, Value: []byte{0x22}, Plugin: "snapshot-test"}
	store := &pluginStore{}
	store.set(a)
	reads := 0
	r.pluginCapabilitySeam = func(selectors ...string) []plugin.InjectedCapability {
		declared := store.get(selectors...)
		reads++
		if reads == 1 {
			// A transaction publishes B after this read took its snapshot;
			// no clock, goroutine schedule or sleep controls the interleaving.
			store.set(b)
		}
		return declared
	}
	session := establishNegotiatedSession(t, peer, capIPv4(), capability.NewPlugin(code, []byte{0x33}))
	session.setPluginCapabilitySnapshotGetter(peer.pluginCapabilitySnapshot)
	if err := session.prepareOpen(); err != nil {
		t.Fatal(err)
	}
	caps, err := capability.ParseFromOptionalParams(session.proposal.message.OptionalParams, session.proposal.message.ExtendedParams)
	if err != nil {
		t.Fatal(err)
	}
	var wire []byte
	for _, offered := range caps {
		if uint8(offered.Code()) == code {
			encoded := make([]byte, offered.Len())
			offered.WriteTo(encoded, 0)
			wire = encoded[2:]
		}
	}
	if len(wire) != 1 {
		t.Fatalf("OPEN plugin value = %x, want one byte from snapshot A or B", wire)
	}
	if !bytes.Equal(wire, a.Value) && !bytes.Equal(wire, b.Value) {
		t.Fatalf("OPEN plugin value = %x, want snapshot A=%x or B=%x", wire, a.Value, b.Value)
	}
	recorded := session.proposal.pluginDeclarations[code]
	if len(recorded) != 1 {
		t.Fatalf("recorded plugin values = %x, want one wire value", recorded)
	}
	if !bytes.Equal([]byte(recorded[0]), wire) {
		t.Errorf("recorded plugin value %x differs from actual OPEN value %x", recorded[0], wire)
	}
	// A failed transaction restores A before BGP reconciles. If OPEN carried B,
	// the peer advertised this code, so keeping that session loses the rollback.
	store.set(a)
	wantKeep := bytes.Equal(wire, a.Value)
	if kept := session.pluginCapabilitiesKept(); kept != wantKeep {
		t.Fatalf("kept=%t after restoring %x, actual OPEN carries %x; want kept=%t", kept, a.Value, wire, wantKeep)
	}
}
