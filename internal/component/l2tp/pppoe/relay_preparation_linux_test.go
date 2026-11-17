// Design: docs/architecture/l2tp/followup-l2tp-call.md -- accepted relay lease ownership
// Related: server_test.go -- driverlessFixture drives discovery with real descriptor owners
//go:build linux

package pppoe

import "testing"

// TestServiceNameRelayReleasesLeaseAtBridge drives the real PADR entry point,
// then the sink's preparation callback. Acceptance retains the unused local
// lease; preparation releases both descriptors while retaining the transport.
func TestServiceNameRelayReleasesLeaseAtBridge(t *testing.T) {
	f := newDriverlessFixture(t, "pppoe { service-name wholesale; interface eth0 {} }")
	f.padr("wholesale")
	pads := f.onlyPADS(t)
	f.sink.mu.Lock()
	preparation := f.sink.relays[0].BeforeBridge
	f.sink.mu.Unlock()
	if preparation == nil {
		t.Fatal("accepted relay has no bridge preparation")
	}
	if f.chann.closedByServer(t) {
		t.Fatal("relay acceptance closed the local channel before bridge preparation")
	}
	if f.unit.closedByServer(t) {
		t.Fatal("relay acceptance closed the local unit before bridge preparation")
	}
	if err := preparation.PrepareBridge(); err != nil {
		t.Fatal(err)
	}
	if !f.chann.closedByServer(t) {
		t.Fatal("bridge preparation left the unused local channel open")
	}
	if !f.unit.closedByServer(t) {
		t.Fatal("bridge preparation left the unused local unit open")
	}
	if f.pppox.closedByServer(t) {
		t.Fatal("bridge preparation closed the PPPoX transport needed by the bridge")
	}
	if err := preparation.PrepareBridge(); err != nil {
		t.Fatalf("repeated preparation: %v", err)
	}
	f.srv.handleSessionDown(f.srv.sessions.Lookup(pads.SID))
	if !f.pppox.closedByServer(t) {
		t.Fatal("relay teardown left the transport open")
	}
}

// TestServiceNameRelayDepartureBeforeBridge drives PADT before the worker is
// ready. Teardown releases the retained lease and preparation refuses the
// departed subscriber rather than lending a closed descriptor to the worker.
func TestServiceNameRelayDepartureBeforeBridge(t *testing.T) {
	f := newDriverlessFixture(t, "pppoe { service-name wholesale; interface eth0 {} }")
	f.padr("wholesale")
	pads := f.onlyPADS(t)
	f.sink.mu.Lock()
	preparation := f.sink.relays[0].BeforeBridge
	f.sink.mu.Unlock()
	if preparation == nil {
		t.Fatal("accepted relay has no bridge preparation")
	}
	f.srv.handlePADT(&Packet{Code: CodePADT, SID: pads.SID, SrcMAC: f.srcMAC, DstMAC: f.hwAddr})
	if !f.chann.closedByServer(t) {
		t.Fatal("departure before preparation leaked the unused local channel")
	}
	if !f.unit.closedByServer(t) {
		t.Fatal("departure before preparation leaked the unused local unit")
	}
	if !f.pppox.closedByServer(t) {
		t.Fatal("departure before preparation leaked the transport")
	}
	if err := preparation.PrepareBridge(); err == nil {
		t.Fatal("bridge preparation accepted a departed subscriber")
	}
}
