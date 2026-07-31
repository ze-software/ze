// VALIDATES: the XFRM translation helpers — xfrmPolicyFromParams maps SPParams
// onto a netlink.XfrmPolicy (direction offset, upper-proto and interface
// selectors, and the single transform template), and the algorithm-name mappers
// (xfrmEncName / xfrmAEADName / xfrmAuthName / xfrmAuthTruncLen) return the
// kernel transform names and truncation lengths, defaults included.
// PREVENTS: an install/delete selector mismatch (kernel identifies a policy by
// its whole selector) or a wrong cipher/auth transform string reaching the kernel.

//go:build linux

package dataplane

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestXfrmPolicyFromParams(t *testing.T) {
	_, src, _ := net.ParseCIDR("192.0.2.0/24")
	_, dst, _ := net.ParseCIDR("198.51.100.0/24")

	// A tunnel-mode policy MUST name its tunnel endpoints. This test formerly built
	// one without them and accepted the result. It asserted the defect. The template
	// reached the kernel as 0.0.0.0 and matched no state.
	tunSrc := net.ParseIP("172.28.0.2")
	tunDst := net.ParseIP("172.28.0.3")

	p := SPParams{
		Src:        src,
		Dst:        dst,
		Dir:        SADirOut, // 2
		Proto:      50,       // ESP
		Mode:       ModeTunnel,
		IfID:       5,
		ReqID:      100,
		UpperProto: 89, // OSPF
		IfIndex:    3,
		TunnelSrc:  tunSrc,
		TunnelDst:  tunDst,
	}

	pol, err := xfrmPolicyFromParams(p)
	if err != nil {
		t.Fatalf("xfrmPolicyFromParams: %v", err)
	}
	if pol.Src != src || pol.Dst != dst {
		t.Errorf("Src/Dst not threaded through: %v/%v", pol.Src, pol.Dst)
	}
	if int(pol.Dir) != int(p.Dir)-1 {
		t.Errorf("Dir = %d, want %d (p.Dir-1)", int(pol.Dir), int(p.Dir)-1)
	}
	if pol.Proto != netlink.Proto(89) {
		t.Errorf("Proto = %d, want 89 (upper-proto selector)", pol.Proto)
	}
	if pol.Ifindex != 3 {
		t.Errorf("Ifindex = %d, want 3", pol.Ifindex)
	}
	if pol.Ifid != 5 {
		t.Errorf("Ifid = %d, want 5", pol.Ifid)
	}
	if len(pol.Tmpls) != 1 {
		t.Fatalf("Tmpls len = %d, want 1", len(pol.Tmpls))
	}
	tmpl := pol.Tmpls[0]
	// The template mode MUST be the kernel number, not the Ze constant. ModeTunnel
	// is 2 in Ze's vocabulary and XFRM_MODE_TUNNEL is 1 on the wire. Asserting the
	// Ze value here is what let the off-by-one reach the kernel unnoticed.
	if tmpl.Proto != netlink.Proto(50) || tmpl.Mode != netlink.XFRM_MODE_TUNNEL || tmpl.Reqid != 100 {
		t.Errorf("Tmpl = proto %d mode %d reqid %d, want 50/%d/100",
			tmpl.Proto, tmpl.Mode, tmpl.Reqid, netlink.XFRM_MODE_TUNNEL)
	}
	if !tmpl.Src.Equal(tunSrc) || !tmpl.Dst.Equal(tunDst) {
		t.Errorf("Tmpl endpoints = %v/%v, want %v/%v", tmpl.Src, tmpl.Dst, tunSrc, tunDst)
	}
}

// VALIDATES: a tunnel-mode policy template carries the tunnel endpoints, the outer
// IP header addresses of the encapsulated packet (RFC 4301 Section 4.4.1.2). The
// kernel resolves a policy to a state through these addresses.
// PREVENTS: the silent 0.0.0.0 template. It resolves to no state, so the policy
// protects nothing, no packet enters the tunnel, and the SA byte counters stay 0.
func TestXfrmPolicyTunnelEndpoints(t *testing.T) {
	_, selLocal, _ := net.ParseCIDR("192.0.2.0/24")
	_, selRemote, _ := net.ParseCIDR("198.51.100.0/24")
	local := net.ParseIP("172.28.0.2")
	remote := net.ParseIP("172.28.0.3")

	pol, err := xfrmPolicyFromParams(SPParams{
		Src:       selLocal,
		Dst:       selRemote,
		Dir:       SADirOut,
		Proto:     ProtoESP,
		Mode:      ModeTunnel,
		ReqID:     1,
		TunnelSrc: local,
		TunnelDst: remote,
	})
	if err != nil {
		t.Fatalf("xfrmPolicyFromParams: %v", err)
	}
	tmpl := pol.Tmpls[0]
	if !tmpl.Src.Equal(local) {
		t.Errorf("Tmpl.Src = %v, want %v (tunnel endpoint, not 0.0.0.0)", tmpl.Src, local)
	}
	if !tmpl.Dst.Equal(remote) {
		t.Errorf("Tmpl.Dst = %v, want %v (tunnel endpoint, not 0.0.0.0)", tmpl.Dst, remote)
	}
	// The endpoints are the outer addresses. The selector stays the inner traffic.
	if tmpl.Src.Equal(selLocal.IP) || tmpl.Dst.Equal(selRemote.IP) {
		t.Errorf("template carries the selector %v/%v, want the tunnel endpoints",
			selLocal.IP, selRemote.IP)
	}
	if pol.Src != selLocal || pol.Dst != selRemote {
		t.Errorf("selector = %v/%v, want %v/%v", pol.Src, pol.Dst, selLocal, selRemote)
	}
}

// VALIDATES: xfrmPolicyFromParams translates each Ze mode to the kernel XFRM mode
// number, and rejects a mode it does not know instead of defaulting.
// PREVENTS: a policy template whose mode disagrees with the state it resolves to,
// and the zero-value trap where an unset Mode field reaches the kernel as a valid
// XFRM_MODE_TRANSPORT.
func TestXfrmPolicyFromParamsMode(t *testing.T) {
	_, src, _ := net.ParseCIDR("192.0.2.0/24")
	_, dst, _ := net.ParseCIDR("198.51.100.0/24")

	for _, tc := range []struct {
		name    string
		mode    uint8
		want    netlink.Mode
		wantErr bool
	}{
		{"transport", ModeTransport, netlink.XFRM_MODE_TRANSPORT, false},
		{"tunnel", ModeTunnel, netlink.XFRM_MODE_TUNNEL, false},
		{"unset zero value", 0, 0, true},
		{"kernel routeoptimization leaking through", 3, 0, true},
		{"out of range", 255, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := SPParams{
				Src: src, Dst: dst, Dir: SADirOut, Proto: 50, Mode: tc.mode, ReqID: 1,
			}
			// Only tunnel mode carries endpoints. RFC 4301 Section 4.4.1.2 leaves a
			// transport-mode template's addresses unused, and the guard rejects them.
			if tc.mode == ModeTunnel {
				p.TunnelSrc = net.ParseIP("172.28.0.2")
				p.TunnelDst = net.ParseIP("172.28.0.3")
			}
			pol, err := xfrmPolicyFromParams(p)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("mode %d accepted, want rejection", tc.mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("mode %d: %v", tc.mode, err)
			}
			if pol.Tmpls[0].Mode != tc.want {
				t.Errorf("mode %d -> kernel %d, want %d", tc.mode, pol.Tmpls[0].Mode, tc.want)
			}
		})
	}
}

// VALIDATES: InstallSA refuses an SA whose mode it cannot translate, before it
// reaches the kernel.
// PREVENTS: a silent install in the wrong IPsec mode.
func TestXFRMInstallSARejectsUnknownMode(t *testing.T) {
	b := &xfrmBackend{}
	err := b.InstallSA(SAParams{
		SPI:   0x1000,
		Src:   net.ParseIP("192.0.2.1"),
		Dst:   net.ParseIP("198.51.100.1"),
		Proto: ProtoESP,
		Mode:  0, // unset
	})
	if err == nil {
		t.Fatal("InstallSA accepted an unset mode, want rejection")
	}
}

func TestXfrmEncName(t *testing.T) {
	for _, tc := range []struct{ algo, want string }{
		{"aes128", "cbc(aes)"},
		{"aes256", "cbc(aes)"},
		{"3des", "cbc(des3_ede)"},
		{"null", "ecb(cipher_null)"},
		{"unknown", "cbc(aes)"},
	} {
		if got := xfrmEncName(tc.algo); got != tc.want {
			t.Errorf("xfrmEncName(%q) = %q, want %q", tc.algo, got, tc.want)
		}
	}
}

func TestXfrmAEADName(t *testing.T) {
	for _, tc := range []struct{ algo, want string }{
		{"aes128gcm", "rfc4106(gcm(aes))"},
		{"aes256gcm", "rfc4106(gcm(aes))"},
		{"chacha20poly1305", "rfc7539esp(chacha20,poly1305)"},
		{"unknown", "rfc4106(gcm(aes))"},
	} {
		if got := xfrmAEADName(tc.algo); got != tc.want {
			t.Errorf("xfrmAEADName(%q) = %q, want %q", tc.algo, got, tc.want)
		}
	}
}

func TestXfrmAuthName(t *testing.T) {
	for _, tc := range []struct{ algo, want string }{
		{"sha256", "hmac(sha256)"},
		{"sha384", "hmac(sha384)"},
		{"sha512", "hmac(sha512)"},
		{"sha1", "hmac(sha1)"},
		{"unknown", "hmac(sha256)"},
	} {
		if got := xfrmAuthName(tc.algo); got != tc.want {
			t.Errorf("xfrmAuthName(%q) = %q, want %q", tc.algo, got, tc.want)
		}
	}
}

func TestXfrmAuthTruncLen(t *testing.T) {
	for _, tc := range []struct {
		algo string
		want int
	}{
		{"sha256", 128},
		{"sha384", 192},
		{"sha512", 256},
		{"sha1", 96},
		{"unknown", 128},
	} {
		if got := xfrmAuthTruncLen(tc.algo); got != tc.want {
			t.Errorf("xfrmAuthTruncLen(%q) = %d, want %d", tc.algo, got, tc.want)
		}
	}
}
