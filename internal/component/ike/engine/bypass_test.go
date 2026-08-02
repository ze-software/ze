package engine

import (
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// bypassDP records both halves of the policy traffic, which mockDP (child_test.go)
// does not: it drops removals on the floor, and this file has to assert that what
// shutdown releases is exactly what start installed.
type bypassDP struct {
	installed  []dataplane.SPParams
	removed    []dataplane.SPParams
	installErr error

	// removedPolicies records RemovePolicy (the three-argument form the Child SA
	// teardown uses). mockDP in child_test.go drops those on the floor, and whether
	// a policy removal HAPPENS is the whole subject of the rekey tests.
	removedPolicies []removedPolicy
	removedSAs      []uint32
}

type removedPolicy struct {
	src, dst string
	dir      dataplane.SADir
}

func (d *bypassDP) InstallSA(_ dataplane.SAParams) error         { return nil }
func (d *bypassDP) ListSAs(_ uint32) ([]dataplane.SAInfo, error) { return nil, nil }
func (d *bypassDP) Close() error                                 { return nil }

func (d *bypassDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	d.removedSAs = append(d.removedSAs, spi)
	return nil
}

func (d *bypassDP) RemovePolicy(src, dst *net.IPNet, dir dataplane.SADir) error {
	d.removedPolicies = append(d.removedPolicies, removedPolicy{
		src: src.String(), dst: dst.String(), dir: dir,
	})
	return nil
}

func (d *bypassDP) InstallPolicy(p dataplane.SPParams) error {
	if d.installErr != nil {
		return d.installErr
	}
	d.installed = append(d.installed, p)
	return nil
}

func (d *bypassDP) RemovePolicyParams(p dataplane.SPParams) error {
	d.removed = append(d.removed, p)
	return nil
}

// spKey is the identity of a policy for set comparison: everything the kernel uses
// to tell one policy from another (vendor netlink selFromPolicy + Dir).
type spKey struct {
	src, dst         string
	dir              dataplane.SADir
	upperProto       uint8
	srcPort, dstPort dataplane.PortMatch
}

func keyOf(p dataplane.SPParams) spKey {
	return spKey{
		src:        p.Src.String(),
		dst:        p.Dst.String(),
		dir:        p.Dir,
		upperProto: p.UpperProto,
		srcPort:    p.SrcPort,
		dstPort:    p.DstPort,
	}
}

// VALIDATES: the IKE bypass set exempts ze's own IKE sockets and nothing else --
// UDP, both IKE ports, both directions, the local port pinned and the remote port
// left free, at a priority that outranks a Child SA policy.
// PREVENTS: the defect this exists for, where a negotiated selector covering the
// peer's IKE address captures the exchange that maintains its own tunnel. Also
// prevents the two tempting narrowings that break under NAT: pinning the REMOTE port
// (a NAT rewrites it) and pinning the peer ADDRESS (MOBIKE and NAT rebinding move
// it).
func TestIKEBypassPoliciesSelectorAndPriority(t *testing.T) {
	for _, fam := range []struct {
		name    string
		sample  net.IP
		wantAny string
	}{
		{"ipv4", net.IPv4zero, "0.0.0.0/0"},
		{"ipv6", net.IPv6zero, "::/0"},
	} {
		t.Run(fam.name, func(t *testing.T) {
			got := ikeBypassPolicies(fam.sample)
			if len(got) != 4 {
				t.Fatalf("bypass policies = %d, want 4 (2 ports x 2 directions); the sweep below would assert nothing", len(got))
			}

			seenOut := map[uint16]bool{}
			seenIn := map[uint16]bool{}
			for i, p := range got {
				if p.Action != dataplane.SPActionBypass {
					t.Errorf("policy[%d] action = %d, want SPActionBypass (%d); a protect policy here would hand ze's own IKE to ESP",
						i, p.Action, dataplane.SPActionBypass)
				}
				if p.Priority != dataplane.PriorityIKEBypass {
					t.Errorf("policy[%d] priority = %d, want PriorityIKEBypass (%d)",
						i, p.Priority, dataplane.PriorityIKEBypass)
				}
				// The IANA number for UDP, written out rather than compared against
				// protoUDP. Comparing a value against the constant that produced it
				// asserts nothing: a mutation of the constant moves both sides
				// together and the test stays green. 0 in this field means "any
				// protocol" to the kernel (net/xfrm/xfrm_selector_match: `|| !sel->proto`),
				// which would exempt any flow that merely used 500 as a port.
				const ianaUDP = 17
				if p.UpperProto != ianaUDP {
					t.Errorf("policy[%d] upper proto = %d, want UDP (%d); 0 means any protocol and would exempt traffic ze was asked to protect",
						i, p.UpperProto, ianaUDP)
				}
				if p.Src.String() != fam.wantAny || p.Dst.String() != fam.wantAny {
					t.Errorf("policy[%d] selector = %s -> %s, want %s -> %s on both halves",
						i, p.Src, p.Dst, fam.wantAny, fam.wantAny)
				}
				// A bypass carries no template, so every template field must stay unset.
				// dataplane.tunnelEndpoints would reject a tunnel-mode template with no
				// endpoints, and a reqid would name a state that must never be resolved.
				if p.Mode != 0 || p.ReqID != 0 || len(p.TunnelSrc) != 0 || len(p.TunnelDst) != 0 {
					t.Errorf("policy[%d] carries template state mode=%d reqid=%d tsrc=%v tdst=%v; a bypass has no template",
						i, p.Mode, p.ReqID, p.TunnelSrc, p.TunnelDst)
				}

				switch p.Dir {
				case dataplane.SADirOut:
					if p.SrcPort.Mask != 0xffff {
						t.Errorf("outbound policy[%d] does not pin the LOCAL source port (mask %#04x)", i, p.SrcPort.Mask)
					}
					if !p.DstPort.IsAny() {
						t.Errorf("outbound policy[%d] pins the remote port %d; a NAT rewrites it and the exemption would stop matching",
							i, p.DstPort.Port)
					}
					seenOut[p.SrcPort.Port] = true
				case dataplane.SADirIn:
					if p.DstPort.Mask != 0xffff {
						t.Errorf("inbound policy[%d] does not pin the LOCAL destination port (mask %#04x)", i, p.DstPort.Mask)
					}
					if !p.SrcPort.IsAny() {
						t.Errorf("inbound policy[%d] pins the remote port %d; a NAT rewrites it and the exemption would stop matching",
							i, p.SrcPort.Port)
					}
					seenIn[p.DstPort.Port] = true
				default:
					t.Errorf("policy[%d] direction = %d; IKE is locally originated and delivered, so only in and out apply (a fwd bypass would exempt transit traffic)",
						i, p.Dir)
				}
			}

			// RFC 7296 Section 2.23: IKE starts on 500 and floats to 4500 behind a NAT.
			// A set covering only one of them leaves the SA unable to rekey on the other.
			for _, port := range []uint16{transport.IKEPort, transport.NATTPort} {
				if !seenOut[port] {
					t.Errorf("no outbound bypass for local port %d", port)
				}
				if !seenIn[port] {
					t.Errorf("no inbound bypass for local port %d", port)
				}
			}
		})
	}
}

// VALIDATES: the bypass outranks a Child SA policy under the kernel's ordering rule.
// PREVENTS: the two policies tying. Both sat at priority 0 before this change, and
// XFRM breaks a tie by insertion order (net/xfrm/xfrm_policy.c, xfrm_policy_insert),
// so the exemption would have won or lost depending on install sequence.
// The kernel's ordering itself is measured in QEMU, not asserted here:
// TestXFRMIKEBypassOutranksChildSAPolicyInKernel (xfrm_bypass_integration_linux_test.go).
func TestIKEBypassOutranksChildSAPolicy(t *testing.T) {
	// LOWER VALUE MEANS HIGHER PRECEDENCE.
	if dataplane.PriorityIKEBypass >= dataplane.PriorityChildSA {
		t.Fatalf("PriorityIKEBypass (%d) must be strictly less than PriorityChildSA (%d): the kernel takes the lowest-numbered matching policy",
			dataplane.PriorityIKEBypass, dataplane.PriorityChildSA)
	}
	if dataplane.PriorityIKEBypass == 0 || dataplane.PriorityChildSA == 0 {
		t.Error("neither priority may be 0: 0 is the best possible rank, so nothing could ever be installed ahead of it")
	}
}

// VALIDATES: the Child SA policies the install emits carry PriorityChildSA, so the
// bypass above actually has something to outrank.
// PREVENTS: the bypass being ranked against a Child SA policy still at the implicit
// 0, which no priority can beat.
func TestChildSAPoliciesCarryChildPriority(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 3, dp, log); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.policies) == 0 {
		t.Fatal("no policies installed; the sweep below would assert nothing")
	}
	for i, p := range dp.policies {
		if p.Action != dataplane.SPActionProtect {
			t.Errorf("child policy[%d] action = %d, want SPActionProtect", i, p.Action)
		}
		if p.Priority != dataplane.PriorityChildSA {
			t.Errorf("child policy[%d] priority = %d, want PriorityChildSA (%d)",
				i, p.Priority, dataplane.PriorityChildSA)
		}
	}
}

// VALIDATES: engine start installs the exemption for BOTH address families, and
// shutdown releases exactly what start installed.
// PREVENTS: kernel state surviving a clean shutdown, and the family guess this
// deliberately avoids -- the bypass is installed before any peer exists, so the
// family of the selectors that will be negotiated is not yet known.
func TestInstallAndRemoveIKEBypassCoverBothFamilies(t *testing.T) {
	log := slogutil.DiscardLogger()

	dp := &bypassDP{}
	installIKEBypass(dp, log)
	if len(dp.installed) != 8 {
		t.Fatalf("installed %d bypass policies, want 8 (2 families x 2 ports x 2 directions)", len(dp.installed))
	}

	v4, v6 := 0, 0
	for _, p := range dp.installed {
		if p.Dst.IP.To4() != nil {
			v4++
		} else {
			v6++
		}
	}
	if v4 != 4 || v6 != 4 {
		t.Errorf("family split = %d v4 / %d v6, want 4 / 4", v4, v6)
	}

	removeDP := &bypassDP{}
	removeIKEBypass(removeDP, log)

	want := map[spKey]bool{}
	for _, p := range dp.installed {
		want[keyOf(p)] = true
	}
	got := map[spKey]bool{}
	for _, p := range removeDP.removed {
		got[keyOf(p)] = true
	}
	if len(want) != len(got) {
		t.Fatalf("shutdown released %d distinct policies, start installed %d", len(got), len(want))
	}
	for k := range want {
		if !got[k] {
			t.Errorf("shutdown did not release the policy installed at start: %+v", k)
		}
	}
}

// VALIDATES: a platform with no XFRM is tolerated, exactly as createFirstChildSA
// tolerates it, and a nil dataplane is a no-op.
// PREVENTS: engine start dying on a host that can run the control plane but not
// program a dataplane. Where no policy can be installed, no policy can capture the
// IKE traffic either, so there is nothing to exempt.
func TestInstallIKEBypassToleratesNoDataplane(t *testing.T) {
	log := slogutil.DiscardLogger()

	installIKEBypass(nil, log) // must not panic
	removeIKEBypass(nil, log)  // must not panic

	unsupported := &bypassDP{installErr: fmt.Errorf("xfrm: policy add: %w", syscall.EPROTONOSUPPORT)}
	installIKEBypass(unsupported, log)
	if len(unsupported.installed) != 0 {
		t.Errorf("recorded %d installs on a platform that refuses every policy", len(unsupported.installed))
	}
}
