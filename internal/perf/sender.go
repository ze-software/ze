// Design: (none -- new tool, predates documentation)
// Overview: benchmark.go -- benchmark orchestration using sender

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
	if s.cfg.Family == FamilyIPv4Unicast && s.cfg.ForceMP {
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

// BuildBatch constructs a serialized UPDATE containing multiple NLRIs.
// All prefixes share the same path attributes (origin, AS_PATH, next-hop).
// For ipv4/unicast without force-mp: packs NLRIs in the inline NLRI field.
// For ipv6/unicast or force-mp: packs NLRIs inside MP_REACH_NLRI.
func (s *Sender) BuildBatch(prefixes []netip.Prefix) []byte {
	if len(prefixes) == 0 {
		return nil
	}

	if len(prefixes) == 1 {
		return s.BuildRoute(prefixes[0])
	}

	var b []byte
	if s.cfg.Family == FamilyIPv4Unicast && !s.cfg.ForceMP {
		b = s.buildInlineBatch(prefixes)
	} else {
		b = s.buildMPBatch(prefixes)
	}

	// Post-serialization safety: reject if batch size computation was off.
	if len(b) > message.MaxMsgLen {
		return nil
	}

	return b
}

// buildInlineBatch packs multiple IPv4/unicast prefixes into the inline NLRI field.
func (s *Sender) buildInlineBatch(prefixes []netip.Prefix) []byte {
	// Build attributes from a single-prefix UPDATE (attributes are prefix-independent).
	dummy := s.builder.BuildUnicast(&message.UnicastParams{
		Prefix:  prefixes[0],
		NextHop: s.cfg.NextHop,
		Origin:  attribute.OriginIGP,
	})
	if dummy == nil {
		return nil
	}

	// Compute total NLRI size, allocate once.
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	nlriTotal := 0
	for _, p := range prefixes {
		nlriTotal += nlri.LenWithContext(nlri.NewINET(family, p, 0), false)
	}

	nlriBytes := make([]byte, nlriTotal)
	off := 0

	for _, p := range prefixes {
		inet := nlri.NewINET(family, p, 0)
		off += nlri.WriteNLRI(inet, nlriBytes, off, false)
	}

	return SerializeMsg(&message.Update{
		PathAttributes: dummy.PathAttributes,
		NLRI:           nlriBytes[:off],
	})
}

// buildMPBatch packs multiple prefixes into MP_REACH_NLRI (force-MP or IPv6).
func (s *Sender) buildMPBatch(prefixes []netip.Prefix) []byte {
	isIBGP := !s.cfg.IsEBGP

	var attrs []attribute.Attribute

	attrs = append(attrs, attribute.OriginIGP)

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

	asPathBytes := make([]byte, asPath.LenWithASN4(true))
	asPath.WriteToWithASN4(asPathBytes, 0, true)

	attrs = append(attrs, &forceMPRawAttr{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBytes,
	})

	if isIBGP {
		attrs = append(attrs, attribute.LocalPref(100))
	}

	// Determine family.
	var family nlri.Family

	var afi attribute.AFI

	var safi attribute.SAFI

	switch s.cfg.Family {
	case FamilyIPv4Unicast:
		family = nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
		afi = attribute.AFIIPv4
		safi = attribute.SAFIUnicast
	case FamilyIPv6Unicast:
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
		afi = attribute.AFIIPv6
		safi = attribute.SAFIUnicast
	default: // Unsupported family -- caller handles nil return.
		return nil
	}

	// Build concatenated NLRIs.
	nlriTotal := 0
	for _, p := range prefixes {
		nlriTotal += nlri.LenWithContext(nlri.NewINET(family, p, 0), false)
	}

	nlriBytes := make([]byte, nlriTotal)
	off := 0

	for _, p := range prefixes {
		inet := nlri.NewINET(family, p, 0)
		off += nlri.WriteNLRI(inet, nlriBytes, off, false)
	}

	mpReach := &attribute.MPReachNLRI{
		AFI:      afi,
		SAFI:     safi,
		NextHops: []netip.Addr{s.cfg.NextHop},
		NLRI:     nlriBytes[:off],
	}
	attrs = append(attrs, mpReach)

	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	attrSize := attribute.AttributesSize(attrs)
	attrBytes := make([]byte, attrSize)
	attribute.WriteAttributesOrdered(attrs, attrBytes, 0)

	return SerializeMsg(&message.Update{
		PathAttributes: attrBytes,
	})
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
