// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- OSPFv3 IPsec SPD and SA install.
// Related: ipsec_install.go -- buildIPsecPolicies, the per-interface out/in/fwd policies.
// Related: ipsec_install_test.go -- the installer fixture (testInstaller, espIface).
//
// VALIDATES: RFC 4552 Section 11, "The IPsec protection barrier MUST be around the OSPF
// protocol.": every OSPF policy an enabled interface installs protects its traffic through
// the configured transform, in the outbound, inbound and forward directions.
// PREVENTS: an installer that lays bypass (template-free) policies, or protect policies
// with no transform, which leaves OSPF traffic outside the barrier while the policy count
// and selectors still look right.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection checks the barrier's disposition.
// Goal: every OSPF policy of an IPsec-enabled interface protects, never bypasses, and names
// the interface's own transform and SA request id, so OSPF traffic cannot cross unprotected.
// Method: install an esp and an ah interface through the installer fixture and read back
// each policy's action, transform protocol, request id and upper-layer protocol.
// RFC requirement: RFC4552-11-1 positive -- the out, in and fwd OSPF (proto 89) policies of
// an enabled interface are protect policies whose template is the configured ESP or AH
// transform and whose request id is the one the interface's SAs carry.
// RFC requirement: RFC4552-11-3 positive -- rules 2 and 3 of the enabled-interface SPD:
// OSPF traffic out of the interface is protected by the ESP or AH transform, and OSPF
// traffic into it is protected, so it must arrive under that transform.
func TestRFC4552BarrierPoliciesProtectOSPFInEveryDirection(t *testing.T) {
	cases := []struct {
		name  string
		iface interfaceConfig
		proto uint8
	}{
		{name: "esp", iface: espIface(256), proto: dataplane.ProtoESP},
		{name: "ah", iface: ahIface(256), proto: dataplane.ProtoAH},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
			inst.setConfig([]interfaceConfig{tc.iface})
			inst.onInterfaceUp(testIfIndex, "eth1")

			if len(fake.sas) == 0 {
				t.Fatal("the enabled interface installed no SA, so no policy template can resolve")
			}
			reqid := fake.sas[0].ReqID
			dirs := map[dataplane.SADir]bool{}
			for _, p := range fake.pols {
				dirs[p.Dir] = true
				if p.Action != dataplane.SPActionProtect {
					t.Errorf("policy dir=%d action = %d, want protect (%d)", p.Dir, p.Action, dataplane.SPActionProtect)
				}
				if p.Proto != tc.proto {
					t.Errorf("policy dir=%d transform proto = %d, want %d", p.Dir, p.Proto, tc.proto)
				}
				if p.ReqID != reqid {
					t.Errorf("policy dir=%d reqid = %d, want the SAs' reqid %d", p.Dir, p.ReqID, reqid)
				}
				if p.UpperProto != ospfv3transport.Protocol {
					t.Errorf("policy dir=%d upper proto = %d, want OSPF (%d)", p.Dir, p.UpperProto, ospfv3transport.Protocol)
				}
			}
			for _, d := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn, dataplane.SADirFwd} {
				if !dirs[d] {
					t.Errorf("no OSPF policy in direction %d: that direction is outside the barrier", d)
				}
			}
		})
	}
}
