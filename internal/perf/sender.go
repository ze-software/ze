// Design: (none -- new tool, predates documentation)
// Related: benchmark.go -- benchmark orchestration using sender

package perf

import (
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// forceMPRawAttr wraps pre-encoded attribute bytes for the force-MP path.
type forceMPRawAttr struct {
	flags attribute.AttributeFlags
	code  attribute.AttributeCode
	data  []byte
}

func (r *forceMPRawAttr) Code() attribute.AttributeCode   { return r.code }
func (r *forceMPRawAttr) Flags() attribute.AttributeFlags { return r.flags }
func (r *forceMPRawAttr) Len() int                        { return len(r.data) }

func (r *forceMPRawAttr) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], r.data)
}

// WriteToWithContext writes pre-encoded data (context-independent).
func (r *forceMPRawAttr) WriteToWithContext(buf []byte, off int, _, _ *bgpctx.EncodingContext) int {
	return r.WriteTo(buf, off)
}

// Compile-time check: forceMPRawAttr must implement attribute.Attribute.
var _ attribute.Attribute = (*forceMPRawAttr)(nil)

// SenderConfig holds the parameters for building UPDATE messages in perf tests.
type SenderConfig struct {
	// ASN is the local autonomous system number.
	ASN uint32

	// IsEBGP indicates whether this is an eBGP session.
	IsEBGP bool

	// NextHop is the next-hop address for announced routes.
	NextHop netip.Addr

	// Family is the address family ("ipv4/unicast" or "ipv6/unicast").
	Family string

	// ForceMP forces use of MP_REACH_NLRI even for IPv4/unicast.
	// Some DUTs require MP encoding for all families.
	ForceMP bool
}

// Sender builds UPDATE messages for route announcements.
type Sender struct {
	builder *message.UpdateBuilder
	cfg     SenderConfig
}

// NewSender creates a new Sender with the given config.
func NewSender(cfg SenderConfig) *Sender {
	isIBGP := !cfg.IsEBGP
	return &Sender{
		builder: message.NewUpdateBuilder(cfg.ASN, isIBGP, true, false),
		cfg:     cfg,
	}
}

// BuildRoute constructs a serialized UPDATE for a single prefix.
//
// For ipv4/unicast without force-mp: uses BuildUnicast with inline NLRI.
// For ipv4/unicast with force-mp: builds UPDATE with MP_REACH_NLRI (AFI=1/SAFI=1).
// For ipv6/unicast: uses BuildUnicast which automatically uses MP_REACH_NLRI.
func (s *Sender) BuildRoute(prefix netip.Prefix) []byte {
	if s.cfg.Family == "ipv4/unicast" && s.cfg.ForceMP {
		return s.buildForceMPRoute(prefix)
	}

	// Standard path: BuildUnicast handles inline NLRI (IPv4) or MP_REACH_NLRI (IPv6).
	params := message.UnicastParams{
		Prefix:  prefix,
		NextHop: s.cfg.NextHop,
		Origin:  attribute.OriginIGP,
	}

	update := s.builder.BuildUnicast(&params)
	if update == nil {
		return nil
	}

	return SerializeMsg(update)
}

// buildForceMPRoute builds an UPDATE with MP_REACH_NLRI for IPv4/unicast.
// This places the NLRI inside MP_REACH_NLRI (type 14) instead of the
// trailing NLRI field, which some DUTs require.
func (s *Sender) buildForceMPRoute(prefix netip.Prefix) []byte {
	isIBGP := !s.cfg.IsEBGP

	// Build attribute list.
	var attrs []attribute.Attribute

	// ORIGIN (type 1)
	attrs = append(attrs, attribute.OriginIGP)

	// AS_PATH (type 2)
	var asPath *attribute.ASPath
	if isIBGP {
		asPath = &attribute.ASPath{}
	} else {
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{s.cfg.ASN}},
			},
		}
	}

	// Encode AS_PATH to bytes (ASN4=true).
	asPathBytes := make([]byte, asPath.LenWithASN4(true))
	asPath.WriteToWithASN4(asPathBytes, 0, true)

	attrs = append(attrs, &forceMPRawAttr{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBytes,
	})

	// LOCAL_PREF (type 5) for iBGP.
	if isIBGP {
		attrs = append(attrs, attribute.LocalPref(100))
	}

	// MP_REACH_NLRI (type 14) with AFI=1/SAFI=1.
	inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	nlriBytes := make([]byte, nlri.LenWithContext(inet, false))
	nlri.WriteNLRI(inet, nlriBytes, 0, false)

	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFIIPv4,
		SAFI:     attribute.SAFIUnicast,
		NextHops: []netip.Addr{s.cfg.NextHop},
		NLRI:     nlriBytes,
	}
	attrs = append(attrs, mpReach)

	// Sort by type code per RFC 4271 Appendix F.3.
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	// Pack attributes.
	attrSize := attribute.AttributesSize(attrs)
	attrBytes := make([]byte, attrSize)
	attribute.WriteAttributesOrdered(attrs, attrBytes, 0)

	// Build UPDATE with no trailing NLRI (all NLRI is in MP_REACH_NLRI).
	update := &message.Update{
		PathAttributes: attrBytes,
	}

	return SerializeMsg(update)
}
