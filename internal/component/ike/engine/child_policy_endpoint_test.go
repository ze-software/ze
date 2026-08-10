// VALIDATES: every Child SA policy names the tunnel endpoints of the SA it must
// resolve to. The inbound policy names remote to local. The outbound policy names
// local to remote. Each pair equals the Src/Dst of the matching ESP state
// (RFC 4301 Section 4.4.1.2, the tunnel header addresses).
// PREVENTS: the 0.0.0.0 template that reached the kernel when the policy carried no
// endpoints. The kernel resolved it to no state, so the tunnel forwarded nothing.
// The XFRM byte counters stayed at zero while both peers reported an established SA.

package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestChildSAPolicyCarriesTunnelEndpoints(t *testing.T) {
	const localAddr, remoteAddr = "10.0.0.1", "10.0.0.2"
	local, remote := net.ParseIP(localAddr), net.ParseIP(remoteAddr)

	dp := &mockDP{}
	child, err := createFirstChildSA(
		testSA(), testESPGroup(), localAddr, remoteAddr, 1, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if !child.ESPInstalled {
		t.Fatal("child SA reports no ESP installed")
	}
	if len(dp.policies) != 2 {
		t.Fatalf("installed %d policies, want 2", len(dp.policies))
	}

	for _, want := range []struct {
		dir      dataplane.SADir
		name     string
		src, dst net.IP
	}{
		{dataplane.SADirIn, "inbound", remote, local},
		{dataplane.SADirOut, "outbound", local, remote},
	} {
		t.Run(want.name, func(t *testing.T) {
			pol := policyForDir(t, dp.policies, want.dir)
			if pol.Mode != dataplane.ModeTunnel {
				t.Fatalf("mode = %d, want ModeTunnel (%d)", pol.Mode, dataplane.ModeTunnel)
			}
			if !pol.TunnelSrc.Equal(want.src) || !pol.TunnelDst.Equal(want.dst) {
				t.Errorf("tunnel endpoints = %v/%v, want %v/%v",
					pol.TunnelSrc, pol.TunnelDst, want.src, want.dst)
			}
			// The endpoints are the outer addresses. The selector stays the inner
			// traffic, and the two must not be the same pair of values by accident.
			if pol.Src == nil || pol.Dst == nil {
				t.Fatal("policy selector is nil")
			}
			// A policy resolves to a state through the endpoints, so an ESP state
			// with exactly this Src/Dst pair MUST exist.
			if !hasStateFor(dp.sas, want.src, want.dst) {
				t.Errorf("no ESP state with src=%v dst=%v, so the policy resolves to nothing",
					want.src, want.dst)
			}
		})
	}
}

// VALIDATES: each of the two ESP states says which direction it carries. The state
// keyed with the peer's send keys is inbound, and the one keyed with ours is outbound.
// PREVENTS: a backend that flags direction per SA guessing it. VPP selects an SA for
// inbound processing by IPSEC_API_SAD_FLAG_IS_INBOUND alone (vppSAFlags,
// ike/dataplane/vpp.go). An inbound state that reaches it unflagged decrypts nothing.
// The tunnel then establishes and carries no traffic, the same silent shape as the
// 0.0.0.0 template above.
func TestChildSAStatesCarryTheirDirection(t *testing.T) {
	const localAddr, remoteAddr = "10.0.0.1", "10.0.0.2"
	local, remote := net.ParseIP(localAddr), net.ParseIP(remoteAddr)

	dp := &mockDP{}
	if _, err := createFirstChildSA(
		testSA(), testESPGroup(), localAddr, remoteAddr, 1, dp, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("installed %d SAs, want 2", len(dp.sas))
	}
	// The direction is read from the ADDRESSES rather than from the install order. This
	// fails if the two states are labeled the wrong way round.
	for _, sa := range dp.sas {
		want := dataplane.SADirOut
		label := "outbound"
		if sa.Src.Equal(remote) && sa.Dst.Equal(local) {
			want, label = dataplane.SADirIn, "inbound"
		}
		if sa.Dir != want {
			t.Errorf("%s SA (src=%v dst=%v) Dir = %d, want %d", label, sa.Src, sa.Dst, sa.Dir, want)
		}
	}
}

// VALIDATES: no Child SA policy leaves a tunnel endpoint unspecified.
// PREVENTS: a regression to 0.0.0.0, the value that produced the silent defect.
func TestChildSAPolicyEndpointsAreSpecified(t *testing.T) {
	dp := &mockDP{}
	if _, err := createFirstChildSA(
		testSA(), testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	for i, pol := range dp.policies {
		for _, ep := range []struct {
			name string
			ip   net.IP
		}{{"TunnelSrc", pol.TunnelSrc}, {"TunnelDst", pol.TunnelDst}} {
			if len(ep.ip) == 0 {
				t.Errorf("policy %d: %s is empty", i, ep.name)
				continue
			}
			if ep.ip.IsUnspecified() {
				t.Errorf("policy %d: %s = %v, want a real tunnel endpoint", i, ep.name, ep.ip)
			}
		}
	}
}

func policyForDir(t *testing.T, policies []dataplane.SPParams, dir dataplane.SADir) dataplane.SPParams {
	t.Helper()
	for _, p := range policies {
		if p.Dir == dir {
			return p
		}
	}
	t.Fatalf("no policy installed for direction %d", dir)
	return dataplane.SPParams{}
}

func hasStateFor(sas []dataplane.SAParams, src, dst net.IP) bool {
	for i := range sas {
		if sas[i].Src.Equal(src) && sas[i].Dst.Equal(dst) {
			return true
		}
	}
	return false
}
