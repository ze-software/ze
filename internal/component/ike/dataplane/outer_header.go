// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- outer tunnel header treatment
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit, Section 5.1.2.1 tunnel header construction
// Related: dataplane.go -- SAParams carries the treatment; xfrm_linux.go and vpp.go project it

package dataplane

import (
	"errors"
	"fmt"
	"strconv"
)

// DFTreatment is what an encapsulator writes into the DF bit of the outer IPv4 header
// of a tunnel-mode SA.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA."
//
// The zero value is DFCopy, and that zero is a deliberate valid value rather than an
// unset one. Section 8.1 makes copy the option every implementation MUST support, it
// is what Linux XFRM does when no flag is set (xfrm4_tunnel_encap_add copies
// XFRM_MODE_SKB_CB(skb)->frag_off & htons(IP_DF)), and every SA ze installed before
// this type existed was installed with it. A caller that states nothing, such as the
// RFC 4552 OSPFv3 installer (plugins/ospf/ipsec_install.go), therefore builds the same
// state it built before.
type DFTreatment uint8

const (
	// DFCopy copies the DF bit of the inner IPv4 header into the outer one.
	DFCopy DFTreatment = iota
	// DFClear writes a clear DF bit, so a router on the path may fragment the
	// tunnel packet.
	DFClear
	// DFSet writes a set DF bit whatever the inner header carries.
	DFSet
)

// String names the treatment as the config spells it.
func (t DFTreatment) String() string {
	switch t {
	case DFCopy:
		return "copy"
	case DFClear:
		return "clear"
	case DFSet:
		return "set"
	}
	return unknownName
}

// valid reports whether t is one of the three named treatments.
func (t DFTreatment) valid() bool {
	switch t {
	case DFCopy, DFClear, DFSet:
		return true
	}
	return false
}

// OuterDSCP is what an encapsulator writes into the DSCP of the outer header of a
// tunnel-mode SA: the inner header's value, or one fixed value for every packet.
//
// RFC 4301 Section 5.1.2.1 note 5: "If the packet will immediately enter a domain for
// which the DSCP value in the outer header is not appropriate, that value MUST be
// mapped to an appropriate value for the domain".
//
// The zero value is OuterDSCPCopy, the "copied from inner hdr" construction the same
// table gives the DS Field, and what Linux XFRM does with no flag set. A fixed value
// is held as 0x80 | dscp, so the six DSCP bits keep their own value and the zero
// value cannot be mistaken for "fixed at 0". Build one with FixedOuterDSCP.
type OuterDSCP uint8

// OuterDSCPCopy copies the inner header's DSCP into the outer header.
const OuterDSCPCopy OuterDSCP = 0

// outerDSCPFixedFlag marks a fixed value. The low six bits hold the DSCP.
const outerDSCPFixedFlag OuterDSCP = 0x80

// DSCPMax is the largest DSCP value: the DS field's six high bits (RFC 2474 Section 3).
const DSCPMax = 63

// FixedOuterDSCP is a treatment that writes dscp into every outer header. It refuses a
// value that does not fit the six DSCP bits.
func FixedOuterDSCP(dscp uint8) (OuterDSCP, error) {
	if dscp > DSCPMax {
		return OuterDSCPCopy, fmt.Errorf("DSCP %d does not fit the six DSCP bits (0 to %d)", dscp, DSCPMax)
	}
	return outerDSCPFixedFlag | OuterDSCP(dscp), nil
}

// Fixed reports the fixed DSCP this treatment writes, and false for copy.
func (d OuterDSCP) Fixed() (uint8, bool) {
	if d&outerDSCPFixedFlag == 0 {
		return 0, false
	}
	return uint8(d &^ outerDSCPFixedFlag), true
}

// String names the treatment as the config spells it: copy, or the decimal value.
func (d OuterDSCP) String() string {
	v, fixed := d.Fixed()
	if !fixed {
		return "copy"
	}
	return strconv.Itoa(int(v))
}

// valid reports whether d is copy or a fixed value that fits six bits.
func (d OuterDSCP) valid() bool {
	if d == OuterDSCPCopy {
		return true
	}
	v, fixed := d.Fixed()
	if !fixed {
		return false
	}
	return v <= DSCPMax
}

// ErrSAOptionRefused means a loaded, working backend cannot apply one option an SA or
// policy asks for: a DF treatment, an outer DSCP, the RFC 4301 Section 6.2 ICMP error
// processing, or an ICMP type and code selector (Section 4.4.1.1, vppICMPSelectorError).
//
// It deliberately does NOT wrap ErrNotSupported. The engine reads ErrNotSupported as a
// platform with no IPsec dataplane at all and keeps that Child SA as a degraded,
// control-plane-only one (engine isXFRMUnsupported, warnDegraded). An option the
// backend refuses is a configuration the operator must change, and an SA kept up
// without it would report the tunnel established while its ESP goes nowhere, so the
// Child SA is refused instead. Config verify refuses the same options at commit
// (SAOptionChecker), so an install meets this error only for a configuration the
// daemon started with, which no verify judged.
var ErrSAOptionRefused = errors.New("dataplane: the backend cannot apply an option this SA asks for")

// SAOptions are the per-peer options a Child SA carries to the dataplane, as config
// verify asks a backend about them before any SA exists.
type SAOptions struct {
	// DF is the df-bit leaf (RFC 4301 Section 8.1).
	DF DFTreatment
	// OuterDSCP is the dscp-map leaf (RFC 4301 Section 5.1.2.1).
	OuterDSCP OuterDSCP
	// ICMPErrorCheck is the icmp-error-check leaf (RFC 4301 Section 6.2): the inbound
	// payload check (SAParams.ICMPErrorCheck) and the outbound return-SA mapping
	// (SPParams.ICMPErrorReturn) together.
	ICMPErrorCheck bool
}

// SAOptionChecker is implemented by a backend that states, ahead of any install,
// whether it can apply a peer's per-SA options. Config verify asks it for every
// site-to-site peer (engine verifyPeerSAOptions), so an option the loaded backend
// cannot honor is refused at commit rather than discovered when IKE has already
// negotiated the Child SA. The install path refuses the same options with
// ErrSAOptionRefused, from the same predicate, for a configuration no verify judged.
//
// A backend that does not implement it has declared nothing, and the engine refuses
// any option other than the defaults on it, as CatchAllInstaller's absence refuses a
// discard catch-all.
type SAOptionChecker interface {
	// SAOptionsSupported returns nil when the backend applies every option exactly,
	// and an error wrapping ErrSAOptionRefused that names the option, the reason and
	// the fix when it cannot.
	SAOptionsSupported(o SAOptions) error
}

// outerHeaderTreatmentError refuses an SA whose outer-header treatment no backend can
// apply exactly, whatever the backend. A backend calls it before it projects the
// treatment, then refuses what its own primitive cannot express.
//
// Only a tunnel-mode SA builds an outer header, and only the encapsulating side
// writes one. A transport-mode SA, or an inbound state, that carries a treatment other
// than the default would install as if the treatment applied and silently ignore it.
func outerHeaderTreatmentError(p SAParams) error {
	if err := outerDSCPError(p); err != nil {
		return err
	}
	if !p.DF.valid() {
		return fmt.Errorf("%w: SA spi=%d carries DF treatment %d, which names none of copy, clear or set",
			ErrSAOptionRefused, p.SPI, p.DF)
	}
	if p.DF == DFCopy {
		return nil
	}
	// RFC 4301 Section 8.1: "All IPsec implementations MUST support the option of
	// copying the DF bit from an outbound packet to the tunnel mode header that it
	// emits, when traffic is carried via a tunnel mode SA."
	if p.Mode != ModeTunnel {
		return fmt.Errorf("%w: SA spi=%d asks for DF %s, but it is not a tunnel-mode SA and builds no outer header to carry the bit; configure df-bit only on a tunnel-mode peer",
			ErrSAOptionRefused, p.SPI, p.DF)
	}
	if p.Dir == SADirIn {
		return fmt.Errorf("%w: SA spi=%d asks for DF %s, but it is an inbound SA, which decapsulates and emits no outer header",
			ErrSAOptionRefused, p.SPI, p.DF)
	}
	// RFC 4301 Section 8.1: "This applies to SAs where both inner and outer headers
	// are IPv4."
	if p.Src.To4() == nil {
		return fmt.Errorf("%w: SA spi=%d asks for DF %s, but its outer header is not IPv4 (src %v), and only an IPv4 header carries a DF bit",
			ErrSAOptionRefused, p.SPI, p.DF, p.Src)
	}
	return nil
}

// outerDSCPError refuses a DSCP treatment on an SA that builds no outer header. The
// DS Field exists in IPv4 and IPv6 alike, so the outer family does not matter here.
func outerDSCPError(p SAParams) error {
	if !p.OuterDSCP.valid() {
		return fmt.Errorf("%w: SA spi=%d carries DSCP treatment %#x, which is neither copy nor a value 0 to %d",
			ErrSAOptionRefused, p.SPI, uint8(p.OuterDSCP), DSCPMax)
	}
	if p.OuterDSCP == OuterDSCPCopy {
		return nil
	}
	// RFC 4301 Section 5.1.2.1 note 5 binds "the DSCP value in the outer header", and
	// only a tunnel-mode SA builds one.
	if p.Mode != ModeTunnel {
		return fmt.Errorf("%w: SA spi=%d asks for outer DSCP %s, but it is not a tunnel-mode SA and builds no outer header; configure dscp-map only on a tunnel-mode peer",
			ErrSAOptionRefused, p.SPI, p.OuterDSCP)
	}
	if p.Dir == SADirIn {
		return fmt.Errorf("%w: SA spi=%d asks for outer DSCP %s, but it is an inbound SA, which decapsulates and emits no outer header",
			ErrSAOptionRefused, p.SPI, p.OuterDSCP)
	}
	return nil
}
