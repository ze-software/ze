// Design: docs/architecture/l2tp/bng-5-pppoe.md -- Linux PPPoE receive identity
// Related: prepare.go -- validate native CP delivery before installing the map
package vpp

import (
	"strings"
	"testing"
)

// Native PPPoE requires PACKET_HOST on the CP device. The VPP dispatch node
// preserves the inbound Ethernet destination, so a different CP MAC cannot
// receive frames addressed to the advertised DP MAC.
func TestVPPPrepareRefusesDifferentControlMAC(t *testing.T) {
	fixture := newAccessPreparationFixture()
	fixture.control.mac = [6]byte{2, 3, 4, 5, 6, 7}
	access, err := fixture.prepare(t.Context())
	if access != nil {
		if closeErr := access.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("different CP and DP MACs published an access")
	}
	if err == nil || !strings.Contains(err.Error(), "MAC") {
		t.Fatalf("missing MAC mismatch refusal: %v", err)
	}
	if fixture.wire.added != 0 || fixture.wire.removed != 0 || len(fixture.wire.bindings) != 0 {
		t.Fatalf("MAC mismatch reached map mutation: adds=%d, deletes=%d, map=%v", fixture.wire.added, fixture.wire.removed, fixture.wire.bindings)
	}
	if len(fixture.preparations.records) != 0 {
		t.Fatal("MAC mismatch published a prepared access")
	}
}

// A common MAC does not collapse the distinct DP, VPP CP and Linux identities.
func TestVPPPrepareMatchingMACPreservesBothDevices(t *testing.T) {
	fixture := newAccessPreparationFixture()
	fixture.control.mac = [6]byte{2, 1, 1, 1, 1, 1}
	access, err := fixture.prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	if access.Device() != "Ethernet0.100" || access.ControlDevice() != "cp0.100" || access.DiscoveryIfIndex() != 73 || access.Namespace() != "dataplane" {
		t.Fatalf("native discovery identity changed: %s/%s/%d/%s", access.Device(), access.ControlDevice(), access.DiscoveryIfIndex(), access.Namespace())
	}
	if access.HWAddr() != fixture.control.mac || fixture.wire.bindings[11] != 42 {
		t.Fatalf("common MAC/pair not preserved: MAC=%v, map=%v", access.HWAddr(), fixture.wire.bindings)
	}
	preparation := fixture.preparations.records["wan"]
	snapshot, err := preparation.borrow(t.Context(), fixture.request())
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.release()
	if snapshot.dpIndex != 11 || snapshot.localMAC != fixture.control.mac || snapshot.api != fixture.connection {
		t.Fatalf("session identity changed: index=%d, MAC=%v, API=%p", snapshot.dpIndex, snapshot.localMAC, snapshot.api)
	}
}
