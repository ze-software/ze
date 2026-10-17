// Design: docs/architecture/api/process-protocol.md -- NLRI blocks in the filter text protocol
// Related: filter_format.go -- AppendUpdateForFilter, the producer of the blocks read here
// Related: filter_delta_handlers.go -- the MP_REACH_NLRI / MP_UNREACH_NLRI handlers applying AttrModReplaceNLRI
// Related: forward_build.go -- buildModifiedPayload, which writes the legacy NLRI override
// RFC: rfc/short/rfc4760.md -- MP_REACH_NLRI and MP_UNREACH_NLRI

package reactor

import (
	"bytes"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// nlriRefusal names why a filter's modify answer cannot be applied to an NLRI
// section. The caller drops the route (fail-closed) and logs both fields, so an
// operator learns which family and which rule the answer broke.
type nlriRefusal struct {
	family string
	reason string
}

// Refusal reasons. Static strings: the refusal is logged, never built.
const (
	refuseNonCIDR   = "the engine rewrites only CIDR families (ipv4/ipv6 unicast, multicast, mpls-label); declare raw=true and return the rewritten UPDATE body, or reject the whole UPDATE"
	refuseNotSubset = "the answer names a prefix the section does not carry, or reorders it; a modify answer may only remove prefixes, keeping the order it was given"
	refuseNoSection = "the answer carries an mp block for a family the UPDATE has no MP_REACH_NLRI or MP_UNREACH_NLRI for; the engine cannot build one"
	refuseDuplicate = "the answer carries two blocks for one section; send each of nlri add, nlri del, mp <family> add and mp <family> del at most once"
	refuseSyntax    = "a block of the answer is neither \"nlri add|del <prefix>...\" nor \"mp <family> add|del <prefix>...\", or the answer carries more blocks than an UPDATE has sections"
	refuseMalformed = "the section the answer rewrites does not parse"
)

// nlriSection names the UPDATE section an NLRI block of the filter text was
// rendered from, and is rewritten into.
type nlriSection uint8

const (
	sectionAnnounce  nlriSection = iota // RFC 4271 Section 4.3 Network Layer Reachability Information: "nlri add"
	sectionWithdrawn                    // RFC 4271 Section 4.3 Withdrawn Routes: "nlri del"
	sectionMPReach                      // RFC 4760 Section 3 MP_REACH_NLRI: "mp <family> add"
	sectionMPUnreach                    // RFC 4760 Section 4 MP_UNREACH_NLRI: "mp <family> del"
	sectionCount
)

// nlriBlock is one block of the NLRI value: its section, its family token (an
// mp block only), its whole text and its prefix tokens. Every string is a
// window of the filter text the table was parsed from.
type nlriBlock struct {
	family  string
	text    string
	body    string
	section nlriSection
}

// maxNLRIBlocks bounds the table. An UPDATE renders at most one block per
// section (four); the slack lets an answer with too many blocks be refused for
// what it is rather than read in part.
const maxNLRIBlocks = 8

// nlriBlockTable is the NLRI value of one filter text, tokenized once into its
// blocks. It lives on the caller's stack: parsing it allocates nothing.
type nlriBlockTable struct {
	blk [maxNLRIBlocks]nlriBlock
	n   int
	bad bool // a block head is malformed, or the value has more than maxNLRIBlocks blocks
}

// parse reads the NLRI value of text: every block from the first "nlri" or
// "mp" keyword up to the next attribute name, which is where the value ends
// for parseFilterAttrsInto too.
func (t *nlriBlockTable) parse(text string) {
	t.n, t.bad = 0, false
	scan := filterTokens{text: text}
	more := scan.next()
	for more && !isNLRIKeyword(text[scan.start:scan.end]) {
		more = scan.next()
	}
	for more && isNLRIKeyword(text[scan.start:scan.end]) {
		if t.n == len(t.blk) {
			t.bad = true
			return
		}
		blk := &t.blk[t.n]
		*blk = nlriBlock{}
		start := scan.start
		mp := text[scan.start:scan.end] == policyAttrMP
		if mp {
			if more = scan.next(); !more {
				t.bad = true
				return
			}
			blk.family = text[scan.start:scan.end]
		}
		if more = scan.next(); !more {
			t.bad = true
			return
		}
		switch text[scan.start:scan.end] {
		case nlriOpAdd:
			blk.section = sectionAnnounce
			if mp {
				blk.section = sectionMPReach
			}
		case nlriOpDel:
			blk.section = sectionWithdrawn
			if mp {
				blk.section = sectionMPUnreach
			}
		default:
			t.bad = true
			return
		}
		end, bodyStart := scan.end, -1
		for more = scan.next(); more && !isPolicyAttrName(text[scan.start:scan.end]); more = scan.next() {
			if bodyStart < 0 {
				bodyStart = scan.start
			}
			end = scan.end
		}
		blk.text = text[start:end]
		if bodyStart >= 0 {
			blk.body = text[bodyStart:end]
		}
		t.n++
	}
}

// find returns the index of the block of section (and, for an mp block, of
// family), or -1.
func (t *nlriBlockTable) find(section nlriSection, fam string) int {
	for i := range t.n {
		if t.blk[i].section == section && t.blk[i].family == fam {
			return i
		}
	}
	return -1
}

// text returns the text of block i, or "" for -1.
func (t *nlriBlockTable) text(i int) string {
	if i < 0 {
		return ""
	}
	return t.blk[i].text
}

// body returns the prefix tokens of block i, or "" for -1: an absent block
// keeps no prefix.
func (t *nlriBlockTable) body(i int) string {
	if i < 0 {
		return ""
	}
	return t.blk[i].body
}

// isNLRIKeyword reports whether token opens an NLRI block.
func isNLRIKeyword(token string) bool {
	return token == policyAttrNLRI || token == policyAttrMP
}

// sectionNLRIDelta applies a filter's modify answer to every NLRI section of the
// UPDATE. It is the per-section half of the filter text protocol: the subject
// renders the legacy NLRI as "nlri add", the legacy Withdrawn Routes as
// "nlri del", each MP_REACH_NLRI as "mp <family> add" and each MP_UNREACH_NLRI
// as "mp <family> del" (filter_format.go), and an answer that changes one of
// those blocks changes that section and no other.
//
// A changed block becomes:
//   - legacy NLRI: the returned override, which buildModifiedPayload writes in
//     place of the source NLRI (an empty, non-nil override drops them all; nil
//     keeps the source);
//   - legacy Withdrawn Routes: a withdrawn rewrite on mods (SetWithdrawnRewrite);
//   - MP_REACH_NLRI / MP_UNREACH_NLRI: an AttrModReplaceNLRI op on code 14 / 15,
//     which keeps AFI, SAFI and the next hop, and drops the attribute when the
//     answer kept no prefix.
//
// The rewritten section is the subset of the source section's own NLRI whose
// prefix the answer kept, so a path identifier (RFC 7911) or a label stack
// (RFC 8277) rides along with its prefix. The answer MUST keep the order of the
// subject and MUST NOT add a prefix: either is refused, as is a block the
// UPDATE has no section for, two blocks for one section, a block that does not
// parse, and any change to a family whose NLRI is not a CIDR prefix. ok ==
// false means the modify MUST NOT be applied and the caller MUST drop the route.
func sectionNLRIDelta(scratch *valueScratch, original, modified string, wireUpdate *wireu.WireUpdate, mods *filterapi.ModAccumulator) ([]byte, nlriRefusal, bool) {
	if original == modified {
		return nil, nlriRefusal{}, true
	}
	var orig, mod nlriBlockTable
	orig.parse(original)
	if orig.bad {
		return nil, nlriRefusal{reason: refuseSyntax}, false
	}
	mod.parse(modified)
	if mod.bad {
		return nil, nlriRefusal{reason: refuseSyntax}, false
	}

	ctx := bgpctx.Registry.Get(wireUpdate.SourceCtxID())
	v4 := family.IPv4Unicast
	v4AddPath := ctx != nil && ctx.AddPath(v4)
	var override []byte

	// RFC 4271 Section 4.3: the legacy Network Layer Reachability Information.
	o, m := orig.find(sectionAnnounce, ""), mod.find(sectionAnnounce, "")
	if orig.text(o) != mod.text(m) {
		raw, err := wireUpdate.NLRI()
		if err != nil {
			return nil, nlriRefusal{family: v4.String(), reason: refuseMalformed}, false
		}
		kept, ok := subsetNLRI(scratch, v4, raw, v4AddPath, false, mod.body(m))
		if !ok {
			return nil, nlriRefusal{family: v4.String(), reason: refuseNotSubset}, false
		}
		override = kept
		if override == nil {
			override = []byte{} // the source carried none, and the answer keeps none
		}
	}

	// RFC 4271 Section 4.3: the legacy Withdrawn Routes field.
	o, m = orig.find(sectionWithdrawn, ""), mod.find(sectionWithdrawn, "")
	if orig.text(o) != mod.text(m) {
		raw, err := wireUpdate.Withdrawn()
		if err != nil {
			return nil, nlriRefusal{family: v4.String(), reason: refuseMalformed}, false
		}
		kept, ok := subsetNLRI(scratch, v4, raw, v4AddPath, true, mod.body(m))
		if !ok {
			return nil, nlriRefusal{family: v4.String(), reason: refuseNotSubset}, false
		}
		mods.SetWithdrawnRewrite(kept)
	}

	var reachName, unreachName string
	if mp, err := wireUpdate.MPReach(); err == nil && mp != nil {
		fam := mp.Family()
		reachName = fam.String()
		if refusal, ok := mpSectionDelta(scratch, &orig, &mod, fam, reachName, sectionMPReach, mp.NLRIBytes(), ctx != nil && ctx.AddPath(fam), mods); !ok {
			return nil, refusal, false
		}
	}
	if mpu, err := wireUpdate.MPUnreach(); err == nil && mpu != nil {
		fam := mpu.Family()
		unreachName = fam.String()
		if refusal, ok := mpSectionDelta(scratch, &orig, &mod, fam, unreachName, sectionMPUnreach, mpu.WithdrawnBytes(), ctx != nil && ctx.AddPath(fam), mods); !ok {
			return nil, refusal, false
		}
	}

	// Every block of the answer names a section the UPDATE carries, once. An
	// mp block the subject never carried has no section to rewrite, and the
	// engine cannot invent an MP_REACH_NLRI next hop for it.
	var seen [sectionCount]bool
	for i := range mod.n {
		blk := &mod.blk[i]
		switch blk.section {
		case sectionMPReach:
			if blk.family != reachName {
				return nil, nlriRefusal{family: blk.family, reason: refuseNoSection}, false
			}
		case sectionMPUnreach:
			if blk.family != unreachName {
				return nil, nlriRefusal{family: blk.family, reason: refuseNoSection}, false
			}
		case sectionAnnounce, sectionWithdrawn, sectionCount:
			// A legacy section always exists, empty or not: subsetNLRI judged it.
		}
		if seen[blk.section] {
			return nil, nlriRefusal{family: blk.family, reason: refuseDuplicate}, false
		}
		seen[blk.section] = true
	}
	return override, nlriRefusal{}, true
}

// mpSectionDelta records the rewrite of one MP section when its block changed.
func mpSectionDelta(scratch *valueScratch, orig, mod *nlriBlockTable, fam family.Family, name string, section nlriSection, nlri []byte, addPath bool, mods *filterapi.ModAccumulator) (nlriRefusal, bool) {
	o, m := orig.find(section, name), mod.find(section, name)
	if orig.text(o) == mod.text(m) {
		return nlriRefusal{}, true
	}
	if !filtertext.CIDRFamily(fam) {
		return nlriRefusal{family: name, reason: refuseNonCIDR}, false
	}
	withdraw := section == sectionMPUnreach
	kept, ok := subsetNLRI(scratch, fam, nlri, addPath, withdraw, mod.body(m))
	if !ok {
		return nlriRefusal{family: name, reason: refuseNotSubset}, false
	}
	code := attribute.AttrMPReachNLRI
	if withdraw {
		code = attribute.AttrMPUnreachNLRI
	}
	mods.Op(uint8(code), filterapi.AttrModReplaceNLRI, kept)
	return nlriRefusal{}, true
}

// cidrEntry frames the NLRI at off of a section of a CIDR family (filtertext.CIDRFamily),
// returning the whole NLRI, its part after any path identifier, and the offset
// past it. ok == false when the section is truncated at off.
//
// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending the
// Path Identifier field, which is of four octets."
// RFC 4760 Section 5: "The Length field indicates the length, in bits, of the
// address prefix."
// RFC 8277 Section 2.2: "It specifies the length in bits of the remainder of the
// NLRI field." A label stack, or a withdrawal's Compatibility field, sits
// inside the length, so every CIDR family frames the same way.
//
//	+0 [Path Identifier(4), ADD-PATH only] Length(1) Prefix(ceil(Length/8))
func cidrEntry(section []byte, off int, addPath bool) (entry, cidr []byte, next int, ok bool) {
	pos := off
	if addPath {
		pos += 4
	}
	if pos >= len(section) {
		return nil, nil, 0, false
	}
	end := pos + 1 + (int(section[pos])+7)/8
	if end > len(section) {
		return nil, nil, 0, false
	}
	return section[off:end], section[pos:end], end, true
}

// subsetNLRI returns the NLRI of section, in source order, whose prefix appears
// in keep, the space-separated prefix tokens of the answer's block, which MUST
// list the kept prefixes in the same order. Each source NLRI is copied whole,
// path identifier and label stack included. The result is carved from scratch;
// an empty result means "keep none".
//
// The walk is one merge of the two ordered lists, bounded by the section
// length, and allocates nothing. ok == false when keep names a prefix the
// section does not carry at that position, a token is not a prefix of fam's
// address family, or the section does not parse.
func subsetNLRI(scratch *valueScratch, fam family.Family, section []byte, addPath, withdraw bool, keep string) ([]byte, bool) {
	routeKey := nlrisplit.GetPrefixKey(fam) // filtertext.CIDRFamily: a CIDR key, never the opaque default
	out := scratch.carveBytes(len(section))[:0]
	var want, have [nlrisplit.PrefixKeyScratchSize]byte
	tokens := filterTokens{text: keep}
	wantKey, ok := nextPrefixKey(&tokens, fam, want[:0])
	if !ok {
		return nil, false
	}
	for off := 0; off < len(section) && wantKey != nil; {
		entry, cidr, next, framed := cidrEntry(section, off, addPath)
		if !framed {
			return nil, false
		}
		off = next
		key, err := routeKey(cidr, have[:], withdraw)
		if err != nil {
			return nil, false
		}
		if !bytes.Equal(key, wantKey) {
			continue // dropped by the filter
		}
		out = append(out, entry...)
		if wantKey, ok = nextPrefixKey(&tokens, fam, want[:0]); !ok {
			return nil, false
		}
	}
	if wantKey != nil {
		return nil, false // a kept prefix the section never reached
	}
	return out, true
}

// nextPrefixKey encodes the next token of tokens as the CIDR key
// nlrisplit.GetPrefixKey answers: the prefix length octet, then the masked
// prefix octets. A nil key with ok means the tokens are exhausted; ok == false
// means the token is not a prefix of fam's address family.
func nextPrefixKey(tokens *filterTokens, fam family.Family, buf []byte) ([]byte, bool) {
	if !tokens.next() {
		return nil, true
	}
	p, err := netip.ParsePrefix(tokens.text[tokens.start:tokens.end])
	if err != nil {
		return nil, false
	}
	if p.Addr().Is4() != (fam.AFI == family.AFIIPv4) {
		return nil, false
	}
	p = p.Masked()
	bits := p.Bits()
	buf = append(buf, byte(bits))
	if p.Addr().Is4() {
		a := p.Addr().As4()
		return append(buf, a[:(bits+7)/8]...), true
	}
	a := p.Addr().As16()
	return append(buf, a[:(bits+7)/8]...), true
}

// appendCIDRPrefixes appends " <prefix>" for every NLRI of a CIDR-family
// section, framed by cidrEntry and named by the family's route key, so the
// subject shows exactly the prefixes subsetNLRI matches an answer against: a
// path identifier or a label stack is never read as prefix bits. A section that
// does not frame or key ends with the token "invalid", which no prefix filter
// accepts and no answer can keep.
func appendCIDRPrefixes(buf []byte, fam family.Family, section []byte, addPath, withdraw bool) []byte {
	routeKey := nlrisplit.GetPrefixKey(fam)
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	for off := 0; off < len(section); {
		_, cidr, next, ok := cidrEntry(section, off, addPath)
		if !ok {
			return append(buf, " invalid"...)
		}
		off = next
		key, err := routeKey(cidr, scratch[:], withdraw)
		if err != nil {
			return append(buf, " invalid"...)
		}
		p := cidrKeyPrefix(fam, key)
		if !p.IsValid() {
			return append(buf, " invalid"...)
		}
		buf = append(buf, ' ')
		buf = p.AppendTo(buf)
	}
	return buf
}

// cidrKeyPrefix reads a CIDR route key (length octet, then the prefix octets)
// back as the prefix it names. The zero Prefix, which IsValid refuses, answers
// a key that names none.
func cidrKeyPrefix(fam family.Family, key []byte) netip.Prefix {
	if len(key) == 0 {
		return netip.Prefix{}
	}
	var a [16]byte
	copy(a[:], key[1:])
	if fam.AFI == family.AFIIPv4 {
		return netip.PrefixFrom(netip.AddrFrom4([4]byte(a[:4])), int(key[0]))
	}
	return netip.PrefixFrom(netip.AddrFrom16(a), int(key[0]))
}
