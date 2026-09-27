// VALIDATES: RFC 5392 Section 4 at the configuration seam: an interface that carries an
// `inter-as` TE block is passive, so the engine runs no Hello and forms no adjacency on it,
// whether or not the operator also wrote `passive true`.
// PREVENTS: an inter-AS link left active, so OSPF Hellos go out to the remote AS and an
// adjacency forms across the AS boundary.
package ospf

import "testing"

// rfc5392Interfaces parses one eth0 interface carrying te, and returns the parsed interface
// and whether the engine runs it actively (Hellos, neighbor FSM).
func rfc5392Interfaces(t *testing.T, te string) (interfaceConfig, bool) {
	t.Helper()
	cfg := parseTECfg(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point",
	    "traffic-engineering":`+te+`}}}}}`)
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("interfaces = %+v", cfg.Interfaces)
	}
	active := false
	for _, ic := range cfg.activeInterfaces() {
		if ic.Name == "eth0" {
			active = true
		}
	}
	return cfg.Interfaces[0], active
}

// TestRFC5392InterASInterfaceIsPassive parses an interface with an inter-as block and no
// `passive` leaf, then the same interface with intra-AS TE only.
func TestRFC5392InterASInterfaceIsPassive(t *testing.T) {
	// RFC requirement: RFC5392-4-1 negative -- an interface carrying an inter-as block is
	// passive even without `passive true`: it is not an active interface, so no Hello is
	// sent over the inter-AS link.
	// RFC requirement: RFC5392-4-2 negative -- the same interface runs no neighbor FSM, so
	// no OSPF adjacency is formed over the inter-AS link.
	ic, active := rfc5392Interfaces(t, `{"enable":true,"inter-as":{"remote-as":"65001","remote-asbr-ipv4":"203.0.113.9"}}`)
	if !ic.Passive || active {
		t.Fatalf("an inter-AS TE interface must be passive and not active: passive %v active %v", ic.Passive, active)
	}

	// Control: intra-AS TE alone does not make an interface passive, so the assertion above
	// is carried by the inter-as block and not by the TE block or the fixture.
	ic, active = rfc5392Interfaces(t, `{"enable":true}`)
	if ic.Passive || !active {
		t.Fatalf("an intra-AS TE interface must stay active: passive %v active %v", ic.Passive, active)
	}
}
