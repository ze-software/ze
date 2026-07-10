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

	p := SPParams{
		Src:        src,
		Dst:        dst,
		Dir:        SADirOut, // 2
		Proto:      50,       // ESP
		Mode:       2,        // tunnel
		IfID:       5,
		ReqID:      100,
		UpperProto: 89, // OSPF
		IfIndex:    3,
	}

	pol := xfrmPolicyFromParams(p)
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
	if tmpl.Proto != netlink.Proto(50) || tmpl.Mode != netlink.Mode(2) || tmpl.Reqid != 100 {
		t.Errorf("Tmpl = proto %d mode %d reqid %d, want 50/2/100", tmpl.Proto, tmpl.Mode, tmpl.Reqid)
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
