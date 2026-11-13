// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact selector installation
// Related: vpp_policy.go -- vppSPDSelector refuses ignored selectors
// Upstream: https://github.com/FDio/vpp/blob/master/src/vnet/ipsec/ipsec_output.h -- policy match reads ports only for TCP, UDP and SCTP.

//go:build ze_vpp

package dataplane

import (
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestVPPRefusesIgnoredDefinedSelectors prevents MH types and UDP-Lite ports
// being installed as a selector VPP silently treats as ANY.
func TestVPPRefusesIgnoredDefinedSelectors(t *testing.T) {
	_, local, _ := net.ParseCIDR("2001:db8:1::/64")
	_, remote, _ := net.ParseCIDR("2001:db8:2::/64")
	for _, proto := range []uint8{135, 136} {
		p := SPParams{
			Src: local, Dst: remote, Dir: SADirOut, Action: SPActionDiscard,
			Priority: 1000, UpperProto: proto,
			SrcPort: ExactPortMatch(5), DstPort: AnyPortMatch(),
		}
		if _, err := vppSPDSelector(p); !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
			t.Errorf("protocol %d exact selector: error %v, want ErrSAOptionRefused", proto, err)
		}
		p.SrcPort = AnyPortMatch()
		if _, err := vppSPDSelector(p); err != nil {
			t.Errorf("protocol %d ANY selector refused: %v", proto, err)
		}
	}
}

// TestVPPInstallRefusesIgnoredDefinedSelectors drives the public backend
// boundary. A refused selector must send no API request, create no SPD and
// record no policy for later removal, for either direction or constrained side.
func TestVPPInstallRefusesIgnoredDefinedSelectors(t *testing.T) {
	for _, proto := range []uint8{135, 136} {
		for _, dir := range []SADir{SADirIn, SADirOut} {
			for _, remote := range []bool{false, true} {
				b, ch := newCapturingBackend()
				p := testSPParams()
				p.Action, p.Dir, p.UpperProto = SPActionDiscard, dir, proto
				_, p.Src, _ = net.ParseCIDR("2001:db8:1::/64")
				_, p.Dst, _ = net.ParseCIDR("2001:db8:2::/64")
				p.SrcPort, p.DstPort = ExactPortMatch(5), AnyPortMatch()
				if remote {
					p.SrcPort, p.DstPort = AnyPortMatch(), ExactPortMatch(5)
				}
				err := b.InstallPolicy(p)
				if !errors.Is(err, ErrSAOptionRefused) {
					t.Fatalf("protocol %d direction %d remote=%v: error %v, want ErrSAOptionRefused", proto, dir, remote, err)
				}
				if errors.Is(err, ErrNotSupported) {
					t.Fatalf("protocol %d refusal looks like a platform without IPsec: %v", proto, err)
				}
				if _, ok := errors.AsType[*opererr.Error](err); !ok {
					t.Errorf("protocol %d refusal %T lacks an operator repair", proto, err)
				}
				if len(ch.sent) != 0 || len(ch.dumps) != 0 {
					t.Errorf("refused protocol %d sent %d API requests and %d dumps", proto, len(ch.sent), len(ch.dumps))
				}
				if len(b.spdEntries) != 0 || b.spdID != 0 || len(b.spdBound) != 0 {
					t.Errorf("refused protocol %d retained %d policies, SPD %d and %d bindings", proto, len(b.spdEntries), b.spdID, len(b.spdBound))
				}
			}
		}
	}
}

// TestVPPInstallAllowsAnyDefinedSelectors is the matching accepted-input
// control: the public boundary emits the requested protocol with full ranges.
func TestVPPInstallAllowsAnyDefinedSelectors(t *testing.T) {
	for _, proto := range []uint8{135, 136} {
		for _, dir := range []SADir{SADirIn, SADirOut} {
			b, ch := newCapturingBackend()
			p := testSPParams()
			p.Action, p.Dir, p.UpperProto = SPActionDiscard, dir, proto
			_, p.Src, _ = net.ParseCIDR("2001:db8:1::/64")
			_, p.Dst, _ = net.ParseCIDR("2001:db8:2::/64")
			p.SrcPort, p.DstPort = AnyPortMatch(), AnyPortMatch()
			if err := b.InstallPolicy(p); err != nil {
				t.Fatalf("protocol %d direction %d ANY refused: %v", proto, dir, err)
			}
			got := sentSPD(t, ch).Entry
			if uint8(got.Protocol) != proto || got.IsOutbound != (dir == SADirOut) {
				t.Errorf("protocol %d direction %d emitted %+v", proto, dir, got)
			}
			if got.LocalPortStart != 0 || got.LocalPortStop != 65535 || got.RemotePortStart != 0 || got.RemotePortStop != 65535 {
				t.Errorf("protocol %d ANY emitted local %d-%d remote %d-%d", proto, got.LocalPortStart, got.LocalPortStop, got.RemotePortStart, got.RemotePortStop)
			}
		}
	}
}
