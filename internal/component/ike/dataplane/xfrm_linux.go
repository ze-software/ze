// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// RFC: rfc/short/rfc4303.md -- ESP SA parameters

//go:build linux

package dataplane

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

type xfrmBackend struct{}

func newXFRMBackend() (Dataplane, error) {
	return &xfrmBackend{}, nil
}

func (b *xfrmBackend) InstallSA(p SAParams) error {
	state := &netlink.XfrmState{
		Src:   p.Src,
		Dst:   p.Dst,
		Proto: netlink.Proto(p.Proto),
		Mode:  netlink.Mode(p.Mode),
		Spi:   int(p.SPI),
		Reqid: int(p.ReqID),
		Ifid:  int(p.IfID),
	}
	if p.ReplayWin > 0 {
		state.ReplayWindow = int(p.ReplayWin)
	}

	if p.IsAEAD {
		state.Aead = &netlink.XfrmStateAlgo{
			Name:   xfrmAEADName(p.EncAlgo),
			Key:    p.EncKey,
			ICVLen: 128,
		}
	} else {
		state.Crypt = &netlink.XfrmStateAlgo{
			Name: xfrmEncName(p.EncAlgo),
			Key:  p.EncKey,
		}
		state.Auth = &netlink.XfrmStateAlgo{
			Name:        xfrmAuthName(p.AuthAlgo),
			Key:         p.AuthKey, //nolint:gosec // ESP integrity key, not a credential
			TruncateLen: xfrmAuthTruncLen(p.AuthAlgo),
		}
	}

	// RFC 3948: UDP encapsulation for NAT-T.
	if p.UDPEncap {
		state.Encap = &netlink.XfrmStateEncap{
			Type:    netlink.XFRM_ENCAP_ESPINUDP,
			SrcPort: int(p.UDPEncapSPort),
			DstPort: int(p.UDPEncapDPort),
		}
	}

	if err := netlink.XfrmStateAdd(state); err != nil {
		return fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
	}
	return nil
}

func (b *xfrmBackend) RemoveSA(spi uint32, dst net.IP, proto uint8) error {
	state := &netlink.XfrmState{
		Dst:   dst,
		Proto: netlink.Proto(proto),
		Spi:   int(spi),
	}
	if err := netlink.XfrmStateDel(state); err != nil {
		return fmt.Errorf("xfrm: state del spi=%d: %w", spi, err)
	}
	return nil
}

func (b *xfrmBackend) InstallPolicy(p SPParams) error {
	pol := &netlink.XfrmPolicy{
		Src: p.Src,
		Dst: p.Dst,
		Dir: netlink.Dir(p.Dir - 1),
		Tmpls: []netlink.XfrmPolicyTmpl{{
			Proto: netlink.Proto(p.Proto),
			Mode:  netlink.Mode(p.Mode),
			Reqid: int(p.ReqID),
		}},
		Ifid: int(p.IfID),
	}
	if err := netlink.XfrmPolicyAdd(pol); err != nil {
		return fmt.Errorf("xfrm: policy add: %w", err)
	}
	return nil
}

func (b *xfrmBackend) RemovePolicy(src, dst *net.IPNet, dir SADir) error {
	pol := &netlink.XfrmPolicy{
		Src: src,
		Dst: dst,
		Dir: netlink.Dir(dir - 1),
	}
	if err := netlink.XfrmPolicyDel(pol); err != nil {
		return fmt.Errorf("xfrm: policy del: %w", err)
	}
	return nil
}

func (b *xfrmBackend) ListSAs(ifID uint32) ([]SAInfo, error) {
	states, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("xfrm: state list: %w", err)
	}
	var out []SAInfo
	for i := range states {
		s := &states[i]
		if ifID != 0 && uint32(s.Ifid) != ifID {
			continue
		}
		out = append(out, SAInfo{
			SPI:  uint32(s.Spi),
			Src:  s.Src,
			Dst:  s.Dst,
			IfID: uint32(s.Ifid),
		})
	}
	return out, nil
}

func (b *xfrmBackend) Close() error { return nil }

func xfrmEncName(algo string) string {
	switch algo {
	case "aes128", "aes256":
		return "cbc(aes)"
	case "3des":
		return "cbc(des3_ede)"
	default:
		return "cbc(aes)"
	}
}

func xfrmAEADName(algo string) string {
	switch algo {
	case "aes128gcm", "aes256gcm":
		return "rfc4106(gcm(aes))"
	case "chacha20poly1305":
		return "rfc7539esp(chacha20,poly1305)"
	default:
		return "rfc4106(gcm(aes))"
	}
}

func xfrmAuthName(algo string) string {
	switch algo {
	case "sha256":
		return "hmac(sha256)"
	case "sha384":
		return "hmac(sha384)"
	case "sha512":
		return "hmac(sha512)"
	case "sha1":
		return "hmac(sha1)"
	default:
		return "hmac(sha256)"
	}
}

func xfrmAuthTruncLen(algo string) int {
	switch algo {
	case "sha256":
		return 128
	case "sha384":
		return 192
	case "sha512":
		return 256
	case "sha1":
		return 96
	default:
		return 128
	}
}
