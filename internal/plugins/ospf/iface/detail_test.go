// VALIDATES: spec-ospf-ext-14 AC-10/AC-11 -- Interface.DetailSnapshot exposes the full
// interface state the summary omits: the retransmit timer and (OSPFv3) the local Interface
// ID + Instance ID.
// PREVENTS: an interface detail view that drops the retransmit timer or the v3 identity.
package iface

import "testing"

func TestInterfaceDetailSnapshot(t *testing.T) {
	cfg := baseConfig(t)
	cfg.RetransmitInterval = 5
	cfg.IsV6 = true
	cfg.InterfaceID = 42
	cfg.InstanceID = 3
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	d := ifc.DetailSnapshot()
	if d.Name != "eth0" || d.RetransmitInterval != 5 {
		t.Fatalf("interface detail = %+v", d)
	}
	if !d.IsV6 || d.InterfaceID != 42 || d.InstanceID != 3 {
		t.Fatalf("v3 interface identity = %+v", d)
	}
	if d.HelloInterval == 0 || d.DeadInterval == 0 {
		t.Fatalf("timers must be populated: %+v", d)
	}
}
