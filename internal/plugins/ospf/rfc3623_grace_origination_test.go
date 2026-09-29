// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- the OSPF restarter.
// Related: gr_restarter.go -- maybeUnplannedRestart and grOriginateGraceLSAs, the producers.
// Related: gr_lsa.go -- grSharedMedia and grV4Body.
//
// VALIDATES: RFC 3623 on the Grace-LSAs the restarter really originates, decoded from the
// link-scope LSDB: the IP interface address TLV on every shared-media segment type
// (section A), the unplanned restart reason carried in the LSA (section 5), and the
// operator's off switch for unplanned recovery, read from the configuration text
// (section 5).
// PREVENTS: a segment-type mapping that drops the interface address TLV on NBMA or
// point-to-multipoint, an origination reason that differs from the recorded one, and a
// config parser that maps every value to planned-and-unplanned.
package ospf

import (
	"testing"
	"time"

	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc3623OriginatedGrace decodes the one Grace-LSA the engine holds for iface.
func rfc3623OriginatedGrace(t *testing.T, e *engine, iface string) ospfpacket.GraceLSA {
	t.Helper()
	var bodies [][]byte
	for _, l := range e.lsdb.LinkLSAs(iface) {
		if l.OpaqueType() == ospfpacket.GraceOpaqueType {
			bodies = append(bodies, l.Body)
		}
	}
	if len(bodies) != 1 {
		t.Fatalf("interface %s holds %d Grace-LSAs, want 1", iface, len(bodies))
	}
	g, err := ospfpacket.DecodeGraceLSA(bodies[0])
	if err != nil {
		t.Fatalf("interface %s Grace-LSA does not decode: %v", iface, err)
	}
	return g
}

// rfc3623RunningEngine is a GR-enabled IPv4 engine with one running interface for each
// named network type.
func rfc3623RunningEngine(t *testing.T, networkTypes map[string]networkType) *engine {
	t.Helper()
	e := grEnableEngine(t, false, time.Unix(1_000_000, 0))
	rid := ospftypes.RouterID{10, 0, 0, 1}
	e.mu.Lock()
	e.cfg.RouterID = rid
	for name, networkType := range networkTypes {
		e.running[name] = interfaceConfig{Name: name, AreaID: ospftypes.BackboneArea, Enabled: true, NetworkType: networkType}
	}
	e.mu.Unlock()
	e.lsdb.SetSelfRouterID(rid)
	return e
}

// RFC requirement: RFC3623-A-4 positive -- the restarter's origination pass gives the Grace-LSA
// of a broadcast, an NBMA and a point-to-multipoint interface the IP interface address TLV
// (type 3), each decoded from the LSA installed in that interface's link-scope LSDB.
func TestRFC3623SharedMediaGraceLSACarriesInterfaceAddress(t *testing.T) {
	// Goal: the segment type reaches the TLV on the real origination path. Method: one running
	// interface per shared-media type, grOriginateGraceLSAs, decode each installed Grace-LSA.
	shared := map[string]networkType{
		"eth0": ospftypes.NetworkBroadcast,
		"eth1": ospftypes.NetworkNBMA,
		"eth2": ospftypes.NetworkPointToMultipoint,
	}
	e := rfc3623RunningEngine(t, shared)
	if ifs := e.grOriginateGraceLSAs(120, grReasonReload, false); len(ifs) != len(shared) {
		t.Fatalf("grOriginateGraceLSAs touched %v, want %d interfaces", ifs, len(shared))
	}
	for name, networkType := range shared {
		if !rfc3623OriginatedGrace(t, e, name).HasInterfaceAddr {
			t.Fatalf("%s (%s) Grace-LSA has no IP interface address TLV", name, networkType)
		}
	}
}

// RFC requirement: RFC3623-5-4 positive -- on an unplanned restart the Grace-LSA that
// maybeUnplannedRestart originates carries restart reason 3 (switch to redundant control
// processor), decoded from the installed LSA, and it is the reason the restarter records.
func TestRFC3623UnplannedGraceLSACarriesUnplannedReason(t *testing.T) {
	// Goal: the reason IN the Grace-LSA. Method: planned-and-unplanned support, a running
	// interface, the cold-boot path, decode the originated LSA.
	e := rfc3623RunningEngine(t, map[string]networkType{"eth0": ospftypes.NetworkPointToPoint})
	unplanned := grTestConfig()
	unplanned.RestarterSupport = grSupportPlannedAndUnplanned
	e.gr.configure(unplanned)

	e.gr.maybeUnplannedRestart()
	if !e.gr.inRestart() {
		t.Fatal("planned-and-unplanned: the cold boot did not enter in-restart")
	}
	g := rfc3623OriginatedGrace(t, e, "eth0")
	if g.Reason != grReasonRedundantCP {
		t.Fatalf("Grace-LSA reason = %d, want 3 (switch to redundant control processor)", g.Reason)
	}
	if g.Reason != e.gr.reason {
		t.Fatalf("Grace-LSA reason %d differs from the recorded reason %d", g.Reason, e.gr.reason)
	}
}

// rfc3623UnplannedFromConfig resolves a graceful-restart restarter support value from the
// configuration text, applies it the way setConfig does, and runs the cold-boot path.
func rfc3623UnplannedFromConfig(t *testing.T, support string) bool {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1",`+
		`"graceful-restart":{"restarter":{"support":"`+support+`"}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	e := rfc3623RunningEngine(t, map[string]networkType{"eth0": ospftypes.NetworkPointToPoint})
	e.gr.configure(cfg.GracefulRestart)
	e.gr.maybeUnplannedRestart()
	return e.gr.inRestart()
}

// RFC requirement: RFC3623-5-1 positive -- the operator turns unplanned recovery on with the
// configuration text `restarter support planned-and-unplanned`: the cold boot enters
// in-restart.
// RFC requirement: RFC3623-5-1 negative -- the operator turns it off with `support planned`
// or `support disabled`: the same cold boot does not enter in-restart.
func TestRFC3623OperatorTurnsUnplannedRecoveryOff(t *testing.T) {
	// Goal: the off switch is reachable from the operator surface. Method: parse each support
	// value from configuration text and drive maybeUnplannedRestart.
	if !rfc3623UnplannedFromConfig(t, "planned-and-unplanned") {
		t.Fatal("support planned-and-unplanned: the cold boot did not enter in-restart")
	}
	for _, off := range []string{"planned", "disabled"} {
		if rfc3623UnplannedFromConfig(t, off) {
			t.Fatalf("support %s: the cold boot entered in-restart, want unplanned recovery off", off)
		}
	}
}
