// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: spd_policy.go -- parseSPDPolicy and SPDPolicy.validate, the producers
// RFC: rfc/short/rfc4301.md -- SPD management (Section 4.4.1), port fields under ANY (Section 7.1)
package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// rfc4301Entry parses one policy entry with the leaves given and the two mandatory
// prefixes, and returns it, or the error the parser refused it with.
func rfc4301Entry(t *testing.T, name string, leaves map[string]string) (SPDPolicy, error) {
	t.Helper()
	cfg, err := ParseIPsecConfig(spdTree(name, leaves,
		map[string]string{"prefix": "192.0.2.0/24"},
		map[string]string{"prefix": "198.51.100.0/24"}))
	if err != nil {
		return SPDPolicy{}, err
	}
	p, ok := cfg.Policies[name]
	if !ok {
		t.Fatalf("the policy list parsed no entry named %s; got %v", name, cfg.Policies)
	}
	return p, nil
}

// VALIDATES: RFC4301-4.4.1-6. The administrator can write a DISCARD and a BYPASS entry in
// each half of the database the section names, SPD-I, SPD-O and both, and each reaches
// the typed model with the disposition and the direction as written. PROTECT is the third
// disposition, written under a site-to-site peer and carried by its Child SA.
// PREVENTS: a disposition or a half of the database no configuration can reach.
// RFC requirement: RFC4301-4.4.1-6 positive -- a bypass and a discard entry can each be written for SPD-I, SPD-O and both.
func TestRFC4301SPDPermitsEachDispositionInEachDatabase(t *testing.T) {
	for _, tc := range []struct {
		action    string
		direction string
		wantAct   dataplane.SPAction
		wantDir   SPDDirection
	}{
		{"bypass", "in", dataplane.SPActionBypass, SPDDirIn},
		{"bypass", "out", dataplane.SPActionBypass, SPDDirOut},
		{"bypass", "both", dataplane.SPActionBypass, SPDDirBoth},
		{"discard", "in", dataplane.SPActionDiscard, SPDDirIn},
		{"discard", "out", dataplane.SPActionDiscard, SPDDirOut},
		{"discard", "both", dataplane.SPActionDiscard, SPDDirBoth},
	} {
		t.Run(tc.action+"-"+tc.direction, func(t *testing.T) {
			p, err := rfc4301Entry(t, "e", map[string]string{"action": tc.action, "direction": tc.direction})
			if err != nil {
				t.Fatalf("ParseIPsecConfig: %v", err)
			}
			if p.Action != tc.wantAct {
				t.Errorf("action = %d, want %d", p.Action, tc.wantAct)
			}
			if p.Direction != tc.wantDir {
				t.Errorf("direction = %v, want %v", p.Direction, tc.wantDir)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.1-6. A disposition or a direction outside the vocabulary is
// refused, and an entry without a disposition is refused rather than read as PROTECT,
// which is the SPAction zero value.
// PREVENTS: an operator entry silently becoming a PROTECT with no transform behind it,
// or a direction the database has no half for.
// RFC requirement: RFC4301-4.4.1-6 negative -- an absent or unknown disposition and an unknown direction are refused.
func TestRFC4301SPDRefusesADispositionItDoesNotCarry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		leaves map[string]string
		want   string
	}{
		{"absent action", map[string]string{}, "action is required"},
		{"protect", map[string]string{"action": "protect"}, "not an SPD disposition"},
		{"unknown action", map[string]string{"action": "permit"}, "not an SPD disposition"},
		{"unknown direction", map[string]string{"action": "bypass", "direction": "sideways"}, "not a side of the IPsec boundary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rfc4301Entry(t, "e", tc.leaves)
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to say %q", err, tc.want)
			}
		})
	}
}

// VALIDATES: RFC4301-4.4.1-7. The management interface is the vpn ipsec policy list: an
// entry an administrator writes there is read into the SPD model with its order, its
// direction, its protocol and its selectors, and a second write of the same list with
// the entry gone removes it from the model.
// PREVENTS: an SPD whose entries can be installed but not managed, which is a database
// without an interface.
// RFC requirement: RFC4301-4.4.1-7 positive -- an entry written under vpn ipsec policy is read into the SPD with every field the operator set, and its removal removes it.
func TestRFC4301SPDIsManagedThroughTheConfiguration(t *testing.T) {
	tree := spdTree("mgmt",
		map[string]string{"action": "bypass", "order": "500", "direction": "out", "protocol": "6"},
		map[string]string{"prefix": "192.0.2.0/24", "port": "22"},
		map[string]string{"prefix": "198.51.100.0/24"})
	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	p, ok := cfg.Policies["mgmt"]
	if !ok {
		t.Fatalf("the policy list parsed no entry named mgmt; got %v", cfg.Policies)
	}
	if p.Order != 500 || p.Direction != SPDDirOut || p.Protocol != 6 {
		t.Fatalf("entry = order %d direction %v protocol %d, want 500 out 6", p.Order, p.Direction, p.Protocol)
	}
	if p.LocalPrefix.String() != "192.0.2.0/24" || p.RemotePrefix.String() != "198.51.100.0/24" {
		t.Fatalf("prefixes = %v %v, want 192.0.2.0/24 198.51.100.0/24", p.LocalPrefix, p.RemotePrefix)
	}
	if p.LocalPort != (PortSelector{Form: PortSingle, Port: 22}) || !p.RemotePort.IsAny() {
		t.Fatalf("ports = local %+v remote %+v, want local 22, remote any", p.LocalPort, p.RemotePort)
	}
	if err := cfg.ValidateSPDPolicies(); err != nil {
		t.Fatalf("a well-formed entry was refused: %v", err)
	}

	// Management is a two-way street: the list without the entry has no entry.
	empty, err := ParseIPsecConfig(spdTree("other", map[string]string{"action": "discard"},
		map[string]string{"prefix": "192.0.2.0/24"}, map[string]string{"prefix": "198.51.100.0/24"}))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if _, still := empty.Policies["mgmt"]; still {
		t.Fatal("an entry absent from the list is still in the SPD")
	}
}

// VALIDATES: RFC4301-4.4.1-7. The interface refuses an entry the dataplane could not
// program exactly, at verify time, rather than installing an approximation: a missing
// prefix, a mixed address family, and a malformed value each come back as an error
// naming the entry.
// PREVENTS: a management interface that accepts a rule and installs a different one.
// RFC requirement: RFC4301-4.4.1-7 negative -- an entry with a missing prefix, mixed families, or a malformed value is refused by name.
func TestRFC4301SPDInterfaceRefusesWhatItCannotProgram(t *testing.T) {
	for _, tc := range []struct {
		name   string
		leaves map[string]string
		local  map[string]string
		remote map[string]string
		want   string
	}{
		{"no local prefix", map[string]string{"action": "bypass"}, nil, map[string]string{"prefix": "198.51.100.0/24"}, "local prefix is required"},
		{"no remote prefix", map[string]string{"action": "bypass"}, map[string]string{"prefix": "192.0.2.0/24"}, nil, "remote prefix is required"},
		{"mixed families", map[string]string{"action": "bypass"}, map[string]string{"prefix": "192.0.2.0/24"}, map[string]string{"prefix": "2001:db8::/32"}, "different address families"},
		{"bad prefix", map[string]string{"action": "bypass"}, map[string]string{"prefix": "192.0.2.0/33"}, map[string]string{"prefix": "198.51.100.0/24"}, "local prefix"},
		{"bad order", map[string]string{"action": "bypass", "order": "soon"}, map[string]string{"prefix": "192.0.2.0/24"}, map[string]string{"prefix": "198.51.100.0/24"}, "is not a number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseIPsecConfig(spdTree("bad", tc.leaves, tc.local, tc.remote))
			if err == nil {
				err = cfg.ValidateSPDPolicies()
			}
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), `"bad"`) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to name the entry and say %q", err, tc.want)
			}
		})
	}
}

// VALIDATES: RFC4301-7.1-3. An entry whose protocol selector is ANY (0, the parser's
// default) is accepted with both port fields ANY, and the ports it carries are the ANY
// form rather than a zero read as port 0.
// PREVENTS: a protocol-ANY entry whose port fields hold an undefined value the kernel
// would match against whatever the transport decode left in the flow key.
// RFC requirement: RFC4301-7.1-3 positive -- a protocol-ANY entry is accepted with both port fields ANY.
func TestRFC4301ProtocolAnyEntryCarriesAnyPorts(t *testing.T) {
	p, err := rfc4301Entry(t, "wide", map[string]string{"action": "bypass"})
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if p.Protocol != 0 {
		t.Fatalf("protocol = %d, want ANY (0)", p.Protocol)
	}
	if !p.LocalPort.IsAny() || !p.RemotePort.IsAny() {
		t.Fatalf("ports = local %+v remote %+v, want both ANY", p.LocalPort, p.RemotePort)
	}
	if err := p.validate(); err != nil {
		t.Fatalf("a protocol-ANY entry with ANY ports was refused: %v", err)
	}
}

// VALIDATES: RFC4301-7.1-3. An entry whose protocol selector is ANY and whose port field
// names a port is refused at verify, on either side, and so is one naming a protocol that
// carries no port fields (Appendix D.2).
// PREVENTS: a port selector under protocol ANY reaching the kernel.
// RFC requirement: RFC4301-7.1-3 negative -- a port under protocol ANY, or under a portless protocol, is refused.
func TestRFC4301ProtocolAnyEntryRefusesAPort(t *testing.T) {
	for _, tc := range []struct {
		name   string
		leaves map[string]string
		local  map[string]string
		remote map[string]string
	}{
		{"local port under ANY", map[string]string{"action": "bypass"}, map[string]string{"prefix": "192.0.2.0/24", "port": "22"}, map[string]string{"prefix": "198.51.100.0/24"}},
		{"remote port under ANY", map[string]string{"action": "bypass"}, map[string]string{"prefix": "192.0.2.0/24"}, map[string]string{"prefix": "198.51.100.0/24", "port": "443"}},
		{"port under ICMP", map[string]string{"action": "bypass", "protocol": "1"}, map[string]string{"prefix": "192.0.2.0/24", "port": "22"}, map[string]string{"prefix": "198.51.100.0/24"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseIPsecConfig(spdTree("narrow", tc.leaves, tc.local, tc.remote))
			if err != nil {
				t.Fatalf("ParseIPsecConfig: %v", err)
			}
			err = cfg.ValidateSPDPolicies()
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), "a port selector needs a protocol that carries ports") {
				t.Fatalf("error = %q, want the port-under-protocol refusal", err)
			}
		})
	}
}
