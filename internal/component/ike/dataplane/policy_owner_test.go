package dataplane

import (
	"errors"
	"net"
	"syscall"
	"testing"
)

// TestPolicyOwnersRecordRoleAndSPD proves the ledger keeps the role and SPD name of
// every entry Ze installs, template-free ones included, joins them back onto a kernel
// row by its selector, names every entry of an interface SPD from the one that
// carries the name, and reads a row Ze holds no record for as role "unknown" in an SPD
// that is still named. It then proves the label follows the kernel's lifetime: a
// confirmed removal drops it, a refused one keeps it. Method: label entries the way
// InstallPolicy does after the kernel accepted them, then read them back as
// policyInfoFromKernel and ListPolicies do (annotate, then nameInterfaceSPDs).
func TestPolicyOwnersRecordRoleAndSPD(t *testing.T) {
	const ifID = 7
	any4 := overlapTestNet(t, "0.0.0.0/0")
	child := SPParams{
		Src: overlapTestNet(t, "10.1.0.0/16"), Dst: overlapTestNet(t, "10.2.0.0/16"),
		Dir: SADirOut, Mode: ModeTunnel, IfID: ifID, Owner: "peer-a", Role: PolicyRoleChildSA,
	}
	catchAll := SPParams{
		Src: any4, Dst: any4, Dir: SADirOut, Action: SPActionDiscard,
		IfID: ifID, Role: PolicyRoleInterfaceCatchAll, SPD: "xfrm0",
	}
	bypass := SPParams{
		Src: any4, Dst: any4, Dir: SADirIn, Action: SPActionBypass,
		UpperProto: 17, DstPort: ExactPortMatch(500), Role: PolicyRoleIKEBypass,
	}

	var o policyOwners
	if _, err := o.claim(child); err != nil {
		t.Fatalf("claim(child): %v", err)
	}
	for _, p := range []SPParams{child, catchAll, bypass} {
		o.label(p)
	}

	asRead := func(p SPParams) PolicyInfo {
		return PolicyInfo{
			Src: p.Src, Dst: p.Dst, Dir: p.Dir, UpperProto: p.UpperProto,
			SrcPort: p.SrcPort, DstPort: p.DstPort, IfIndex: p.IfIndex, IfID: p.IfID,
		}
	}
	foreignSameSPD := asRead(SPParams{Src: overlapTestNet(t, "192.0.2.0/24"), Dst: any4, Dir: SADirFwd, IfID: ifID})
	foreignOtherSPD := asRead(SPParams{Src: any4, Dst: any4, Dir: SADirOut, IfID: 9})
	foreignNode := asRead(SPParams{Src: overlapTestNet(t, "198.51.100.0/24"), Dst: any4, Dir: SADirOut})
	rows := []PolicyInfo{asRead(child), asRead(catchAll), asRead(bypass), foreignSameSPD, foreignOtherSPD, foreignNode}
	for i := range rows {
		o.annotate(&rows[i])
	}
	nameInterfaceSPDs(rows)

	want := []struct {
		name, role, spd string
		ownerKnown      bool
	}{
		{"child sa", "child-sa", "xfrm0", true},
		{"interface catch-all", "interface-catch-all", "xfrm0", false},
		{"ike bypass", "ike-bypass", "node", false},
		{"foreign entry in a bound SPD", "unknown", "xfrm0", false},
		{"foreign entry in an unbound SPD", "unknown", "if-id 9", false},
		{"foreign entry in the node SPD", "unknown", "node", false},
	}
	for i, w := range want {
		if got := rows[i].Role.String(); got != w.role {
			t.Errorf("%s: role %q, want %q", w.name, got, w.role)
		}
		if got := rows[i].SPDName(); got != w.spd {
			t.Errorf("%s: spd %q, want %q", w.name, got, w.spd)
		}
		if rows[i].OwnerKnown != w.ownerKnown {
			t.Errorf("%s: owner-known %v, want %v", w.name, rows[i].OwnerKnown, w.ownerKnown)
		}
	}

	roleOf := func(p SPParams) PolicyRole {
		info := asRead(p)
		o.annotate(&info)
		return info.Role
	}
	// A removal the kernel refused leaves the entry, so its label stays.
	refused := errors.New("kernel refused")
	if err := o.deleteThenRelease(catchAll, func() error { return refused }); !errors.Is(err, refused) {
		t.Fatalf("deleteThenRelease(refused) = %v, want the kernel's refusal", err)
	}
	if got := roleOf(catchAll); got != PolicyRoleInterfaceCatchAll {
		t.Fatalf("after a refused removal the catch-all reads %v, want it kept", got)
	}
	// ENOENT says the kernel holds no such entry, so the label goes with it.
	if err := o.deleteThenRelease(catchAll, func() error { return syscall.ENOENT }); !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("deleteThenRelease(ENOENT) = %v, want ENOENT passed on", err)
	}
	if got := roleOf(catchAll); got != PolicyRoleUnspecified {
		t.Fatalf("after the kernel lost the catch-all it reads %v, want unknown", got)
	}
	if err := o.deleteThenRelease(child, func() error { return nil }); err != nil {
		t.Fatalf("deleteThenRelease(child): %v", err)
	}
	if got := roleOf(child); got != PolicyRoleUnspecified {
		t.Fatalf("after removal the child SA policy reads %v, want unknown", got)
	}
	o.releaseBySelector(bypass.Src, bypass.Dst, bypass.Dir)
	if got := roleOf(bypass); got != PolicyRoleUnspecified {
		t.Fatalf("after releaseBySelector the bypass reads %v, want unknown", got)
	}
}

func overlapTestNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", cidr, err)
	}
	return n
}

// TestPolicySelectorsOverlapExported proves the exported predicate the IKE engine uses
// for its cross-peer tunnel check (RFC 3948 Section 5.1) answers exactly what the
// unexported selectorsOverlap answers over policyKey, on identical, overlapping,
// disjoint, different-if_id, different-protocol and port inputs, and that the verdict is
// symmetric. Method: a table of SPParams pairs, each judged both ways and against the
// unexported predicate.
func TestPolicySelectorsOverlapExported(t *testing.T) {
	base := func(src, dst string) SPParams {
		return SPParams{
			Src: overlapTestNet(t, src), Dst: overlapTestNet(t, dst),
			Dir: SADirOut, Mode: ModeTunnel, IfID: 1, Owner: "a",
		}
	}
	cases := []struct {
		name string
		a, b SPParams
		want bool
	}{
		{"identical", base("10.0.0.0/24", "10.1.2.3/32"), base("10.0.0.0/24", "10.1.2.3/32"), true},
		{"host inside wildcard", base("0.0.0.0/0", "10.1.2.3/32"), base("192.0.2.10/32", "10.1.2.3/32"), true},
		{"ipv6 host inside wildcard", base("::/0", "2001:db8::1/128"), base("2001:db8:1::/48", "::/0"), true},
		{"adjacent hosts", base("0.0.0.0/0", "10.1.2.3/32"), base("0.0.0.0/0", "10.1.2.4/32"), false},
		{"disjoint local sides", base("192.0.2.10/32", "10.1.2.3/32"), base("198.51.100.0/24", "10.1.2.3/32"), false},
		{"different if_id", base("0.0.0.0/0", "0.0.0.0/0"), func() SPParams {
			p := base("0.0.0.0/0", "0.0.0.0/0")
			p.IfID = 2
			return p
		}(), false},
		{"different protocol", func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.UpperProto = 6
			return p
		}(), func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.UpperProto = 17
			return p
		}(), false},
		{"port 65535 versus any", func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.DstPort = ExactPortMatch(65535)
			return p
		}(), base("0.0.0.0/0", "10.1.2.3/32"), true},
		{"port 179 versus 180", func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.DstPort = ExactPortMatch(179)
			return p
		}(), func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.DstPort = ExactPortMatch(180)
			return p
		}(), false},
		{"owner and priority do not count", base("0.0.0.0/0", "10.1.2.3/32"), func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.Owner = "b"
			p.Priority = 1500
			return p
		}(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PolicySelectorsOverlap(tc.a, tc.b); got != tc.want {
				t.Fatalf("PolicySelectorsOverlap(a, b) = %v, want %v", got, tc.want)
			}
			if got := PolicySelectorsOverlap(tc.b, tc.a); got != tc.want {
				t.Fatalf("PolicySelectorsOverlap(b, a) = %v, want %v (not symmetric)", got, tc.want)
			}
			if got := selectorsOverlap(policyKey(tc.a), policyKey(tc.b)); got != tc.want {
				t.Fatalf("selectorsOverlap = %v, want %v", got, tc.want)
			}
		})
	}
}
