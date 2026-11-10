// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: spd_policy.go -- parseICMPSelector and validateICMP, the producers
// RFC: rfc/short/rfc4301.md -- the ICMP type and code selector (Section 4.4.1.1)
package ipsec

import (
	"errors"
	"strings"
	"testing"
)

// icmpSPD builds one discard entry between two IPv4 prefixes with the given protocol
// and ICMP leaves; an empty value leaves the leaf out of the tree.
func icmpSPD(t *testing.T, proto, icmpType, icmpCode string) (*IPsecConfig, error) {
	t.Helper()
	return icmpSPDBetween(t, proto, icmpType, icmpCode, "192.0.2.0/24", "198.51.100.0/24")
}

// icmpSPDBetween is icmpSPD between the given local and remote prefixes.
func icmpSPDBetween(t *testing.T, proto, icmpType, icmpCode, local, remote string) (*IPsecConfig, error) {
	t.Helper()
	leaves := map[string]string{"action": "discard", "direction": "in"}
	for k, v := range map[string]string{"protocol": proto, "icmp-type": icmpType, "icmp-code": icmpCode} {
		if v != "" {
			leaves[k] = v
		}
	}
	tree := spdTree("icmp", leaves,
		map[string]string{"prefix": local},
		map[string]string{"prefix": remote})
	return ParseIPsecConfig(tree)
}

// VALIDATES: the three forms RFC 4301 Section 4.4.1.1 gives the ICMP selector parse
// into ICMPSelector exactly, at both ends of the 8-bit type and code ranges.
// PREVENTS: a boundary value (type 0, echo reply; code 255) read as ANY, which would
// install a selector matching every ICMP message.
func TestSPDPolicyICMPSelectorForms(t *testing.T) {
	for _, tc := range []struct {
		name, typ, code string
		want            ICMPSelector
	}{
		{"absent leaves are any", "", "", ICMPSelector{}},
		{"any type any code", "any", "any", ICMPSelector{}},
		{"type 0 any code", "0", "", ICMPSelector{HasType: true, Type: 0}},
		{"type 255 any code", "255", "any", ICMPSelector{HasType: true, Type: 255}},
		{"single code 0", "3", "0", ICMPSelector{HasType: true, Type: 3, HasCode: true, CodeStart: 0, CodeEnd: 0}},
		{"single code 255", "3", "255", ICMPSelector{HasType: true, Type: 3, HasCode: true, CodeStart: 255, CodeEnd: 255}},
		{"code range", "3", "1-13", ICMPSelector{HasType: true, Type: 3, HasCode: true, CodeStart: 1, CodeEnd: 13}},
		{"whole code range", "3", "0-255", ICMPSelector{HasType: true, Type: 3, HasCode: true, CodeStart: 0, CodeEnd: 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := icmpSPD(t, "1", tc.typ, tc.code)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := cfg.Policies["icmp"].ICMP; got != tc.want {
				t.Fatalf("ICMP selector = %+v, want %+v", got, tc.want)
			}
			if err := cfg.ValidateSPDPolicies(); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

// VALIDATES: a type or code outside 0..255, and a code range written backwards, are
// refused at parse.
// PREVENTS: a value truncated to 8 bits (256 read as 0) installing a selector for a
// type the operator never wrote.
func TestSPDPolicyICMPSelectorBoundsRefused(t *testing.T) {
	for _, tc := range []struct{ name, typ, code string }{
		{"type 256", "256", ""},
		{"type negative", "-1", ""},
		{"code 256", "3", "256"},
		{"range end 256", "3", "0-256"},
		{"range backwards", "3", "9-2"},
		{"range open", "3", "4-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := icmpSPD(t, "1", tc.typ, tc.code)
			if !errors.Is(err, ErrSPDPolicy) {
				t.Fatalf("parse error = %v, want an ErrSPDPolicy refusal", err)
			}
		})
	}
}

// VALIDATES: an ICMP selector is refused under a protocol other than ICMP or ICMPv6,
// and a code is refused under ANY type, while protocol 58 is accepted.
// PREVENTS: the kernel comparing an ICMP type against a TCP port, and a code-only
// selector, which is none of the three forms RFC 4301 Section 4.4.1.1 allows.
func TestSPDPolicyICMPSelectorNeedsICMPAndAType(t *testing.T) {
	for _, tc := range []struct {
		name, proto, typ, code string
		refused                bool
	}{
		{"type under protocol 0", "", "8", "", true},
		{"type under TCP", "6", "8", "", true},
		{"code under any type", "1", "", "3", true},
		{"type under ICMPv6", "58", "135", "0", false},
		{"type under ICMP", "1", "8", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local, remote := "192.0.2.0/24", "198.51.100.0/24"
			if tc.proto == "58" {
				// ICMPv6 is IPv6's ICMP: the family check refuses it between IPv4
				// prefixes (TestSPDPolicyICMPSelectorFamilyMismatchRefused).
				local, remote = "2001:db8:1::/48", "2001:db8:2::/48"
			}
			cfg, err := icmpSPDBetween(t, tc.proto, tc.typ, tc.code, local, remote)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = cfg.ValidateSPDPolicies()
			if tc.refused && !errors.Is(err, ErrSPDPolicy) {
				t.Fatalf("validate = %v, want an ErrSPDPolicy refusal", err)
			}
			if !tc.refused && err != nil {
				t.Fatalf("validate refused a valid entry: %v", err)
			}
		})
	}
}

// VALIDATES: an entry with an outbound half (direction out or both) whose ICMP selector
// holds type 0 code 0 is refused at commit, under ICMP and ICMPv6 and for either action,
// and the refusal names the entry, the reason and both fixes. The same type inbound only,
// type 0 with a code range starting at 1, and another type with code 0 are accepted.
// PREVENTS: an outbound BYPASS or DISCARD that also takes every forwarded ICMP fragment.
// Linux reads type 0 code 0 from a fragment and classifies forwarded traffic before
// reassembly, so such an entry would pass in the clear, or drop, fragments of ICMP
// messages the operator never named (RFC 4301 Section 7.4).
func TestSPDPolicyICMPTypeZeroOutboundRefused(t *testing.T) {
	const v4Local, v4Remote = "192.0.2.0/24", "198.51.100.0/24"
	const v6Local, v6Remote = "2001:db8:1::/48", "2001:db8:2::/48"
	for _, tc := range []struct {
		name, action, direction, proto, typ, code string
		refused                                   bool
	}{
		{"discard both type 0 any code", "discard", "both", "1", "0", "", true},
		{"bypass out type 0 code 0", "bypass", "out", "1", "0", "0", true},
		{"bypass both type 0 codes 0-3", "bypass", "both", "1", "0", "0-3", true},
		{"discard both ICMPv6 type 0", "discard", "both", "58", "0", "any", true},
		{"bypass in type 0 any code", "bypass", "in", "1", "0", "", false},
		{"discard in type 0 code 0", "discard", "in", "1", "0", "0", false},
		{"bypass both type 0 codes 1-255", "bypass", "both", "1", "0", "1-255", false},
		{"discard out type 3 code 0", "discard", "out", "1", "3", "0", false},
		{"bypass both any type", "bypass", "both", "1", "any", "any", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local, remote := v4Local, v4Remote
			if tc.proto == "58" {
				local, remote = v6Local, v6Remote
			}
			leaves := map[string]string{"action": tc.action, "direction": tc.direction, "protocol": tc.proto, "icmp-type": tc.typ}
			if tc.code != "" {
				leaves["icmp-code"] = tc.code
			}
			cfg, err := ParseIPsecConfig(spdTree("frag", leaves,
				map[string]string{"prefix": local}, map[string]string{"prefix": remote}))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = cfg.ValidateSPDPolicies()
			if !tc.refused {
				if err != nil {
					t.Fatalf("validate refused an entry no fragment matches outbound: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrSPDPolicy) {
				t.Fatalf("validate = %v, want an ErrSPDPolicy refusal", err)
			}
			for _, part := range []string{`"frag"`, "type 0 code 0", "every IP fragment", local, "Set direction in", "starts at 1"} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("refusal %q does not name %q", err, part)
				}
			}
		})
	}
}

// VALIDATES: an ICMP type and code selector under the ICMP protocol of the other
// address family is refused at commit, and the refusal names the entry, the
// mismatch and the protocol that fixes it; the matching pairs are accepted.
// PREVENTS: protocol 1 on IPv6 prefixes, or 58 on IPv4 ones, installing a policy
// that matches no packet: ICMPv4 and ICMPv6 number their message types apart, so
// the operator's BYPASS or DISCARD would silently never apply.
// Method: one entry per family and protocol through ParseIPsecConfig and
// ValidateSPDPolicies, reading the error text for the three parts.
func TestSPDPolicyICMPSelectorFamilyMismatchRefused(t *testing.T) {
	const v4Local, v4Remote = "192.0.2.0/24", "198.51.100.0/24"
	const v6Local, v6Remote = "2001:db8:1::/48", "2001:db8:2::/48"
	for _, tc := range []struct {
		name, proto, local, remote string
		fix                        string
	}{
		{"ICMP on IPv6", "1", v6Local, v6Remote, "Set protocol to 58"},
		{"ICMPv6 on IPv4", "58", v4Local, v4Remote, "Set protocol to 1"},
		{"ICMP on IPv4", "1", v4Local, v4Remote, ""},
		{"ICMPv6 on IPv6", "58", v6Local, v6Remote, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := icmpSPDBetween(t, tc.proto, "3", "4", tc.local, tc.remote)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = cfg.ValidateSPDPolicies()
			if tc.fix == "" {
				if err != nil {
					t.Fatalf("validate refused a matching family and protocol: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrSPDPolicy) {
				t.Fatalf("validate = %v, want an ErrSPDPolicy refusal", err)
			}
			for _, part := range []string{`"icmp"`, "protocol " + tc.proto, tc.local, "would match no packet", tc.fix} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("refusal %q does not name %q", err, part)
				}
			}
		})
	}
}
