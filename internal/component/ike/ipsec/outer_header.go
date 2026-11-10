// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec cross-reference validation
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit, Section 5.1.2.1 tunnel header construction
// Related: ../dataplane/outer_header.go -- the treatment types each backend projects

package ipsec

import (
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// parseDFBit reads a peer's df-bit leaf. An absent leaf is dataplane.DFCopy, the YANG
// default, which is the option RFC 4301 Section 8.1 makes mandatory.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA."
func parseDFBit(peerName, value string, present bool) (dataplane.DFTreatment, error) {
	if !present {
		return dataplane.DFCopy, nil
	}
	switch value {
	case "copy":
		return dataplane.DFCopy, nil
	case "clear":
		return dataplane.DFClear, nil
	case "set":
		return dataplane.DFSet, nil
	}
	return dataplane.DFCopy, fmt.Errorf("ipsec peer %q df-bit: unsupported value %q (valid: copy, clear, set)",
		peerName, value)
}

// parseDSCPMap reads a peer's dscp-map leaf: copy, or a DSCP value 0 to 63. An absent
// leaf is dataplane.OuterDSCPCopy, the YANG default and the DS Field construction RFC
// 4301 Section 5.1.2.1 gives the outer header, "copied from inner hdr".
//
// RFC 4301 Section 5.1.2.1 note 5: "If the packet will immediately enter a domain for
// which the DSCP value in the outer header is not appropriate, that value MUST be
// mapped to an appropriate value for the domain [NiBlBaBL98]."
func parseDSCPMap(peerName, value string, present bool) (dataplane.OuterDSCP, error) {
	if !present {
		return dataplane.OuterDSCPCopy, nil
	}
	if value == "copy" {
		return dataplane.OuterDSCPCopy, nil
	}
	n, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return dataplane.OuterDSCPCopy, fmt.Errorf("ipsec peer %q dscp-map: %q is neither copy nor a DSCP value 0 to %d",
			peerName, value, dataplane.DSCPMax)
	}
	dscp, err := dataplane.FixedOuterDSCP(uint8(n))
	if err != nil {
		return dataplane.OuterDSCPCopy, fmt.Errorf("ipsec peer %q dscp-map: %w", peerName, err)
	}
	return dscp, nil
}

// ValidateOuterHeader refuses a peer whose outer tunnel header treatment cannot be
// applied exactly to the Child SAs it builds.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA.  This applies to SAs where both inner and outer headers are IPv4."
//
// Three treatments are refused at commit rather than at the first SA install, where a
// refusal would leave a peer that commits and never carries traffic:
//
//   - set, on every dataplane. Linux XFRM copies the inner bit or clears it
//     (xfrm4_tunnel_encap_add), and VPP's ESP encrypt path never reads ENCAP_SET_DF.
//   - anything but copy on a transport-mode peer, which builds no outer header.
//   - anything but copy on a peer whose tunnel endpoints are IPv6, because an IPv6
//     header has no DF bit to treat.
//
// The leaf stays copy on such a peer, which the dataplane projects as its default.
func (c *IPsecConfig) ValidateOuterHeader() error {
	for name := range c.Peers {
		peer := c.Peers[name]
		if err := validatePeerDFBit(name, peer.DFBit, peer.Mode, peer.LocalAddress, peer.RemoteAddress); err != nil {
			return err
		}
		// RFC 4301 Section 5.1.2.1 binds "the DSCP value in the outer header", which a
		// transport-mode SA does not build.
		if peer.OuterDSCP != dataplane.OuterDSCPCopy && peer.Mode == dataplane.ModeTransport {
			return fmt.Errorf("ipsec peer %q: dscp-map %s needs a tunnel-mode Child SA, and a transport-mode "+
				"SA builds no outer header. Remove dscp-map or set mode tunnel", name, peer.OuterDSCP)
		}
	}
	return nil
}

// validatePeerDFBit refuses, at commit, a df-bit leaf no dataplane can apply to the
// peer's Child SAs: set on any peer, and clear on a peer whose SAs build no IPv4 outer
// header (transport mode, or an IPv6 local-address or remote-address literal). Copy,
// the default, always passes.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA.  This applies to SAs where both inner and outer headers are IPv4."
func validatePeerDFBit(name string, df dataplane.DFTreatment, mode uint8, local, remote string) error {
	if df == dataplane.DFCopy {
		return nil
	}
	if df == dataplane.DFSet {
		return fmt.Errorf("ipsec peer %q: df-bit set cannot be applied by any dataplane Ze drives: "+
			"the Linux kernel copies the inner DF bit or clears it and has no flag that sets it, "+
			"and VPP's ESP encrypt path ignores its set-DF flag. Configure df-bit copy or df-bit clear",
			name)
	}
	if mode == dataplane.ModeTransport {
		return fmt.Errorf("ipsec peer %q: df-bit %s needs a tunnel-mode Child SA, and a transport-mode "+
			"SA builds no outer header to carry the bit. Remove df-bit or set mode tunnel",
			name, df)
	}
	for _, side := range []struct{ leaf, value string }{{"local-address", local}, {"remote-address", remote}} {
		addr, err := netip.ParseAddr(side.value)
		if err != nil {
			// Not an address literal: an unset or symbolic endpoint, whose family is
			// learned at runtime. The dataplane refuses what it cannot express then.
			continue
		}
		if addr.Unmap().Is4() {
			continue
		}
		return fmt.Errorf("ipsec peer %q: df-bit %s applies only where the outer header is IPv4 "+
			"(RFC 4301 Section 8.1), and %s %s is IPv6, whose header has no DF bit. Remove df-bit",
			name, df, side.leaf, side.value)
	}
	return nil
}
