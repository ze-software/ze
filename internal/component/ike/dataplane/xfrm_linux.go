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
	mode, ok := kernelXFRMMode(p.Mode)
	if !ok {
		return fmt.Errorf("xfrm: state add spi=%d: unknown mode %d, want ModeTransport (%d) or ModeTunnel (%d)",
			p.SPI, p.Mode, ModeTransport, ModeTunnel)
	}
	state := &netlink.XfrmState{
		Src:   p.Src,
		Dst:   p.Dst,
		Proto: netlink.Proto(p.Proto),
		Mode:  netlink.Mode(mode),
		Spi:   int(p.SPI),
		Reqid: int(p.ReqID),
		Ifid:  int(p.IfID),
	}
	if p.ReplayWin > 0 {
		state.ReplayWindow = int(p.ReplayWin)
	}

	// RFC 4552 OSPFv3: an explicit state selector (x->sel) lets one wildcard-address
	// SA (Src=Dst=::) be resolved for any OSPF flow (ff02::5, ff02::6, neighbor
	// unicast) instead of only flows whose daddr equals p.Dst. IKE child SAs leave
	// p.Sel nil, so msg.Sel stays the zero value (byte-identical to before).
	if p.Sel != nil {
		state.Selector = &netlink.XfrmPolicy{
			Src:   p.Sel.Src,
			Dst:   p.Sel.Dst,
			Proto: netlink.Proto(p.Sel.UpperProto), // 0 = any, 89 = OSPF
		}
	}

	// RFC 4302 (AH) vs RFC 4303 (ESP): AH sets an integrity transform only, ESP
	// sets encryption + integrity, and a combined-mode algorithm sets a single
	// AEAD transform. planStateAlgos isolates that decision from netlink so it is
	// unit-testable on any platform.
	plan := planStateAlgos(p)
	if plan.AEAD {
		state.Aead = &netlink.XfrmStateAlgo{
			Name:   xfrmAEADName(p.EncAlgo),
			Key:    p.EncKey,
			ICVLen: 128,
		}
	}
	if plan.Crypt {
		state.Crypt = &netlink.XfrmStateAlgo{
			Name: xfrmEncName(p.EncAlgo),
			Key:  p.EncKey,
		}
	}
	if plan.Auth {
		state.Auth = &netlink.XfrmStateAlgo{
			Name:        xfrmAuthName(p.AuthAlgo),
			Key:         p.AuthKey, //nolint:gosec // AH/ESP integrity key, not a credential
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
	pol, err := xfrmPolicyFromParams(p)
	if err != nil {
		return fmt.Errorf("xfrm: policy add: %w", err)
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

func (b *xfrmBackend) RemovePolicyParams(p SPParams) error {
	pol, err := xfrmPolicyFromParams(p)
	if err != nil {
		return fmt.Errorf("xfrm: policy del: %w", err)
	}
	if err := netlink.XfrmPolicyDel(pol); err != nil {
		return fmt.Errorf("xfrm: policy del: %w", err)
	}
	return nil
}

// xfrmPolicyFromParams builds the netlink policy shared by install and delete so
// the delete selector (Src, Dst, Dir, upper-layer Proto, Ifid) matches the
// installed policy exactly; the kernel identifies a policy by its whole selector.
// It rejects an unknown mode, because the template mode must agree with the mode
// of the state it resolves to.
//
// The template also carries the tunnel endpoints in tunnel mode. Those addresses
// are how the kernel resolves the policy to a state (RFC 4301 Section 4.4.1.2).
// tunnelEndpoints rejects an absent pair, so a 0.0.0.0 template never reaches the
// kernel. Such a template matched no state and the tunnel forwarded nothing.
func xfrmPolicyFromParams(p SPParams) (*netlink.XfrmPolicy, error) {
	mode, ok := kernelXFRMMode(p.Mode)
	if !ok {
		return nil, fmt.Errorf("unknown mode %d, want ModeTransport (%d) or ModeTunnel (%d)",
			p.Mode, ModeTransport, ModeTunnel)
	}
	tmplSrc, tmplDst, err := tunnelEndpoints(p)
	if err != nil {
		return nil, err
	}
	srcPort, err := xfrmSelectorPort("source", p.SrcPort)
	if err != nil {
		return nil, err
	}
	dstPort, err := xfrmSelectorPort("destination", p.DstPort)
	if err != nil {
		return nil, err
	}
	return &netlink.XfrmPolicy{
		Src:     p.Src,
		Dst:     p.Dst,
		Dir:     netlink.Dir(p.Dir - 1),
		Proto:   netlink.Proto(p.UpperProto), // upper-layer selector (0 = any, 89 = OSPF)
		SrcPort: srcPort,
		DstPort: dstPort,
		Ifindex: p.IfIndex, // RFC 4552 §6 interface-based selector (0 = node-wide)
		Tmpls: []netlink.XfrmPolicyTmpl{{
			// Src/Dst are the outer tunnel-header addresses, not the selector above.
			// They stay nil in transport mode, where the kernel leaves them unused.
			Src:   tmplSrc,
			Dst:   tmplDst,
			Proto: netlink.Proto(p.Proto),
			Mode:  netlink.Mode(mode),
			Reqid: int(p.ReqID),
		}},
		Ifid: int(p.IfID),
	}, nil
}

// xfrmSelectorPort converts a PortMatch to the port number netlink writes into the XFRM
// selector, and REFUSES a match the selector cannot express.
//
// The kernel selector is a port plus a mask, but selFromPolicy derives the mask from the
// port: it sets a full mask only when the port is non-zero
// (vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go, selFromPolicy). So exactly
// two matches are expressible through this API:
//
//	any port      -> port 0, mask 0.
//	one port N>=1 -> port N, mask 0xffff.
//
// A match asking for "exactly port 0" cannot be built: writing 0 yields mask 0, which
// matches EVERY port. That is the OPAQUE port form of RFC 7296 Section 3.13.1, and
// installing it as any-port would protect more traffic than was negotiated. So it is
// refused here rather than widened (ai/rules/exact-or-reject.md).
func xfrmSelectorPort(side string, p PortMatch) (int, error) {
	if p.IsAny() {
		return 0, nil
	}
	if p.Mask != 0xffff {
		return 0, fmt.Errorf(
			"xfrm: %s port mask %#04x is not expressible: the kernel selector this backend builds carries either no port constraint or one exact port, so the mask must be 0 or 0xffff",
			side, p.Mask)
	}
	if p.Port == 0 {
		return 0, fmt.Errorf(
			"xfrm: %s port selector asks for exactly port 0, which this backend cannot express: netlink derives the port mask from the port value, so port 0 always matches every port; RFC 7296 Section 3.13.1 OPAQUE ports have no XFRM encoding",
			side)
	}
	return int(p.Port), nil
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
	case "null":
		// RFC 4552 §3 / RFC 2410: ESP with NULL encryption (authentication-only ESP).
		// NOTE: verify against target kernel -- the kernel's ealg registry names this
		// transform "cipher_null" (net/xfrm/xfrm_algo.c: ealg_list "cipher_null"), and
		// some kernels/iproute2 reject the "ecb(cipher_null)" spelling. Validate the
		// accepted string on the appliance kernel in QEMU (cannot be exercised here).
		return "ecb(cipher_null)"
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
