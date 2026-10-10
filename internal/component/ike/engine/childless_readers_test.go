// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: rfc7296_auth_child_refusal_test.go -- how an IKE SA becomes childless

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestChildlessSAMaintenance drives every reader of the session's Child SA on a
// session that holds an established IKE SA and no Child SA.
//
// Goal: RFC 7296 Section 2.21.2 makes a childless IKE SA a normal state (AC-10), so no
// reader may assume a Child SA exists. Method: an established PSK SA is put on a
// session with no Child SA, and each reader runs: the ESP metric, Info for the CLI,
// the MOBIKE Child SA migration, a peer Delete naming an unknown ESP SPI, and the
// cleanup that ends the session. Each MUST answer "no Child SA" and none MUST touch
// the dataplane.
//
// MUTATION: drop the `child != nil` test from espInstalled (metrics.go) and this test
// goes red with a nil dereference.
func TestChildlessSAMaintenance(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, _, _ := establishPSK(t)
	ps := &PeerSession{peerName: "ze", peerCfg: ini.PeerCfg, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	ps.setSA(ini)
	dp := &mockDP{}

	if espInstalled(map[string]*PeerSession{"ze": ps}, "ze") {
		t.Error("the ESP metric reports an installed Child SA on a childless session")
	}
	if info := ps.Info(); info.HasChild || info.ChildInSPI != 0 || info.ChildOutSPI != 0 {
		t.Errorf("Info reports a Child SA on a childless session: has=%v in=%#08x out=%#08x",
			info.HasChild, info.ChildInSPI, info.ChildOutSPI)
	}
	if err := ps.migrateMobikeChild(ini, dp); err != nil {
		t.Errorf("the MOBIKE migration of a childless session failed: %v", err)
	}
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolESP, SPISize: 4, NumSPIs: 1, SPIs: []byte{0, 0, 0, 7}}
	if paired, down := ps.closeDesignatedChildSAs(del, dp, log); len(paired) != 0 || down {
		t.Errorf("a Delete naming an unknown SPI on a childless session paired %v and child-down=%v", paired, down)
	}
	ps.cleanupChild(dp, nil, log)
	if len(dp.sas) != 0 {
		t.Errorf("the readers left %d ESP states in the dataplane, want 0", len(dp.sas))
	}
	if ps.getChildSA() != nil {
		t.Error("a reader created a Child SA")
	}
}
