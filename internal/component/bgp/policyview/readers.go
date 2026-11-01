// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// RFC: rfc/short/rfc7606.md -- attribute length checks (Section 7)
// Related: view.go -- the view and its caches
// Related: register.go -- which field each reader serves
//
// The field readers. Each one takes the bytes it needs from the UPDATE,
// checks them against the RFC 7606 shape of the attribute, and answers them
// borrowed. The ingress pass already applied RFC 7606 to the UPDATE; these
// checks are the second half of that pair (docs/contributing/ze-go-style.md,
// "Pair the check"), so a value the view answers is one the RFC calls well
// formed, and a malformed one fails the route instead of reading as zero.

package policyview

import (
	"encoding/binary"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

var (
	errNoContext     = errors.New("the UPDATE's encoding context is not registered")
	errTwoOctet      = errors.New("the UPDATE is not in the four-octet AS form the ingest collapse produces")
	errLocalASNUnset = errors.New("the session's local AS is not known")
)

// readPrefix serves prefix: the NLRI sections the UPDATE announces.
func readPrefix(v *View) (Value, error) {
	body, mpReach, err := v.nlriSections()
	if err != nil {
		return Value{}, err
	}
	return prefixValue(body, mpReach), nil
}

// prefixValue is prefix over the body NLRI and the MP_REACH_NLRI value, the
// sections as received or as View.SetNLRI overlaid them.
func prefixValue(body []byte, mpReach wireu.MPReachWire) Value {
	return Value{present: len(body) > 0 || mpReach != nil, wire: body, mpReach: mpReach}
}

// readFamily serves family: IPv4 unicast for the body NLRI (RFC 4271 carries
// only that family there), and the MP_REACH_NLRI family through MPReach.
func readFamily(v *View) (Value, error) {
	body, mpReach, err := v.nlriSections()
	if err != nil {
		return Value{}, err
	}
	return familyValue(body, mpReach), nil
}

// familyValue is family over the body NLRI and the MP_REACH_NLRI value, the
// sections as received or as View.SetNLRI overlaid them.
func familyValue(body []byte, mpReach wireu.MPReachWire) Value {
	x := Value{present: len(body) > 0 || mpReach != nil, mpReach: mpReach}
	if len(body) > 0 {
		x.family = family.IPv4Unicast
	}
	return x
}

// readNextHop serves next-hop: the NEXT_HOP attribute for the body NLRI, and
// the MP_REACH_NLRI next hop through MPReach.
func readNextHop(v *View) (Value, error) {
	_, mpReach, err := v.nlriSections()
	if err != nil {
		return Value{}, err
	}
	raw, err := v.raw(attribute.AttrNextHop)
	if err != nil {
		return Value{}, err
	}
	// RFC 7606 Section 7.3: "The attribute is considered malformed if its
	// length is not 4 [RFC4271]."
	if raw != nil && len(raw) != 4 {
		return Value{}, malformed(attribute.AttrNextHop, "length is not 4")
	}
	return nextHopValue(raw, mpReach), nil
}

// nextHopValue is next-hop over the NEXT_HOP attribute value and the
// MP_REACH_NLRI value, as received or as View.SetNLRI overlaid it. NEXT_HOP is
// an attribute of its own, so no NLRI change removes it.
func nextHopValue(raw []byte, mpReach wireu.MPReachWire) Value {
	return Value{present: raw != nil || mpReach != nil, wire: raw, mpReach: mpReach}
}

// readProtocol serves protocol: every route on this view came from BGP, so
// the value is whether its session is eBGP. It decodes nothing.
func readProtocol(v *View) (Value, error) {
	return Value{present: true, ebgp: v.session.EBGP}, nil
}

// readOrigin serves origin.
func readOrigin(v *View) (Value, error) {
	raw, err := v.raw(attribute.AttrOrigin)
	if raw == nil {
		return Value{}, err
	}
	// RFC 7606 Section 7.1: "The attribute is considered malformed if its
	// length is not 1 or if it has an undefined value [RFC4271]."
	if len(raw) != 1 {
		return Value{}, malformed(attribute.AttrOrigin, "length is not 1")
	}
	if raw[0] > uint8(attribute.OriginIncomplete) {
		return Value{}, malformed(attribute.AttrOrigin, "undefined value")
	}
	return Value{present: true, wire: raw, number: uint64(raw[0]), numbered: true}, nil
}

// readASPath serves as-path.
func readASPath(v *View) (Value, error) {
	path, present, err := v.reconstructedPath()
	if err != nil {
		return Value{}, err
	}
	return Value{present: present, path: path}, nil
}

// readOriginAS serves origin-as.
//
// RFC 6811 Section 2: "Route Origin ASN: The origin AS number derived from a
// Route as follows: * the rightmost AS in the final segment of the AS_PATH
// attribute in the Route if that segment is of type AS_SEQUENCE, or * the BGP
// speaker's own AS number if that segment is of type AS_CONFED_SEQUENCE or
// AS_CONFED_SET or if the AS_PATH is empty, or * the distinguished value
// "NONE" if the final segment of the AS_PATH attribute is of any other type."
//
// NONE is answered as a present value with no number, never as zero.
func readOriginAS(v *View) (Value, error) {
	path, _, err := v.reconstructedPath()
	if err != nil {
		return Value{}, err
	}
	if path != nil && len(path.Segments) > 0 {
		last := path.Segments[len(path.Segments)-1]
		if last.Type == attribute.ASSequence {
			if len(last.ASNs) == 0 {
				// RFC 7606 Section 7.2: a segment is malformed if "It has a
				// Path Segment Length field of zero." The parser refuses it;
				// this keeps a peer from reaching an index out of range.
				return Value{}, malformed(attribute.AttrASPath, "empty segment")
			}
			return Value{present: true, number: uint64(last.ASNs[len(last.ASNs)-1]), numbered: true}, nil
		}
		if last.Type != attribute.ASConfedSequence && last.Type != attribute.ASConfedSet {
			return Value{present: true}, nil // NONE: AS_SET, or a type RFC 6811 names "any other"
		}
	}
	ctx, err := v.context()
	if err != nil {
		return Value{}, err
	}
	local := ctx.LocalASN()
	if local == 0 {
		return Value{}, errLocalASNUnset
	}
	return Value{present: true, number: uint64(local), numbered: true}, nil
}

// readMED serves med.
func readMED(v *View) (Value, error) {
	// RFC 7606 Section 7.4: "The attribute is considered malformed if its
	// length is not 4 [RFC4271]."
	return v.fixed4(attribute.AttrMED)
}

// readLocalPreference serves local-preference.
func readLocalPreference(v *View) (Value, error) {
	// RFC 7606 Section 7.5: "if received from an internal neighbor, it SHALL
	// be considered malformed if its length is not equal to 4." From an
	// external neighbor the ingress pass discarded it, so any one left is
	// judged by the same length.
	return v.fixed4(attribute.AttrLocalPref)
}

// readAtomicAggregate serves atomic-aggregate: present or not.
func readAtomicAggregate(v *View) (Value, error) {
	raw, err := v.raw(attribute.AttrAtomicAggregate)
	if raw == nil {
		return Value{}, err
	}
	// RFC 7606 Section 7.6: "The attribute SHALL be considered malformed if
	// its length is not 0 [RFC4271]."
	if len(raw) != 0 {
		return Value{}, malformed(attribute.AttrAtomicAggregate, "length is not 0")
	}
	return Value{present: true, wire: raw}, nil
}

// readAggregator serves aggregator: the AS as Number, the router's address as
// Wire.
func readAggregator(v *View) (Value, error) {
	if err := v.requireFourOctet(); err != nil {
		return Value{}, err
	}
	raw, err := v.raw(attribute.AttrAggregator)
	if raw == nil {
		return Value{}, err
	}
	// RFC 7606 Section 7.7: "Its length is not 8 (when the 4-octet AS number
	// capability is both advertised to and received from the peer)."
	if len(raw) != 8 {
		return Value{}, malformed(attribute.AttrAggregator, "length is not 8")
	}
	return Value{present: true, wire: raw[4:8], number: uint64(binary.BigEndian.Uint32(raw)), numbered: true}, nil
}

// readCommunity serves community.
func readCommunity(v *View) (Value, error) {
	// RFC 7606 Section 7.8: "The Community attribute SHALL be considered
	// malformed if its length is not a non-zero multiple of 4."
	return v.multiple(attribute.AttrCommunity, 4)
}

// readExtendedCommunity serves extended-community.
func readExtendedCommunity(v *View) (Value, error) {
	// RFC 7606 Section 7.14: "The Extended Community attribute SHALL be
	// considered malformed if its length is not a non-zero multiple of 8."
	return v.multiple(attribute.AttrExtCommunity, 8)
}

// readLargeCommunity serves large-community.
func readLargeCommunity(v *View) (Value, error) {
	// RFC 8092 Section 6: "A BGP Large Communities attribute SHALL be
	// considered malformed if the length of the BGP Large Communities
	// Attribute value, expressed in octets, is not a non-zero multiple of 12."
	return v.multiple(attribute.AttrLargeCommunity, 12)
}

// readOriginatorID serves originator-id.
func readOriginatorID(v *View) (Value, error) {
	// RFC 7606 Section 7.9: "if received from an internal neighbor, it SHALL
	// be considered malformed if its length is not equal to 4."
	return v.fixed4(attribute.AttrOriginatorID)
}

// readClusterList serves cluster-list.
func readClusterList(v *View) (Value, error) {
	// RFC 7606 Section 7.10: "if received from an internal neighbor, it SHALL
	// be considered malformed if its length is not a non-zero multiple of 4."
	return v.multiple(attribute.AttrClusterList, 4)
}

// readAIGP serves aigp: the first metric TLV's value as Number, the whole
// attribute as Wire. An attribute with no metric TLV is present with no
// number.
func readAIGP(v *View) (Value, error) {
	raw, err := v.raw(attribute.AttrAIGP)
	if raw == nil {
		return Value{}, err
	}
	// attribute.AIGPMetricOffset walks the TLVs of RFC 7311 Section 3 and
	// refuses a TLV overrun or a metric TLV of the wrong length.
	off, err := attribute.AIGPMetricOffset(raw)
	if err != nil {
		return Value{}, errors.Join(malformed(attribute.AttrAIGP, "TLV sequence"), err)
	}
	x := Value{present: true, wire: raw}
	if off >= 0 {
		x.number, x.numbered = binary.BigEndian.Uint64(raw[off:]), true
	}
	return x, nil
}

// fixed4 reads an attribute whose value is one four-octet number.
func (v *View) fixed4(code attribute.AttributeCode) (Value, error) {
	raw, err := v.raw(code)
	if raw == nil {
		return Value{}, err
	}
	if len(raw) != 4 {
		return Value{}, malformed(code, "length is not 4")
	}
	return Value{present: true, wire: raw, number: uint64(binary.BigEndian.Uint32(raw)), numbered: true}, nil
}

// multiple reads an attribute whose value is a non-zero number of unit-octet
// entries.
func (v *View) multiple(code attribute.AttributeCode, unit int) (Value, error) {
	raw, err := v.raw(code)
	if raw == nil {
		return Value{}, err
	}
	if len(raw) == 0 {
		return Value{}, malformed(code, "length is zero")
	}
	if len(raw)%unit != 0 {
		return Value{}, malformed(code, "length is not a multiple of the entry size")
	}
	return Value{present: true, wire: raw}, nil
}

// nlriSections answers the body NLRI and the MP_REACH_NLRI attribute, decoded
// once per run for prefix, family and next-hop.
func (v *View) nlriSections() ([]byte, wireu.MPReachWire, error) {
	if v.nlri.run == v.run {
		return v.nlri.body, v.nlri.mpReach, v.nlri.err
	}
	v.decodes.countNLRI()
	body, err := v.update.NLRI()
	var mpReach wireu.MPReachWire
	if err == nil {
		mpReach, err = v.update.MPReach()
	}
	if err != nil {
		err = errors.Join(ErrMalformed, err)
		body, mpReach = nil, nil
	}
	v.nlri = shared{run: v.run, body: body, mpReach: mpReach, err: err}
	return body, mpReach, err
}

// reconstructedPath answers the AS path, built once per run however many
// members read it: as-path and origin-as share it. The parse is the
// attribute package's own, so the RFC 7606 Section 7.2 segment checks are
// the ones the ingress pass ran.
func (v *View) reconstructedPath() (*attribute.ASPath, bool, error) {
	if v.asPath.run == v.run {
		return v.asPath.path, v.asPath.present, v.asPath.err
	}
	path, present, err := v.parsePath()
	v.asPath = sharedPath{run: v.run, path: path, present: present, err: err}
	return path, present, err
}

func (v *View) parsePath() (*attribute.ASPath, bool, error) {
	if err := v.requireFourOctet(); err != nil {
		return nil, false, err
	}
	attrs, err := v.attrs()
	if err != nil {
		return nil, false, err
	}
	if attrs == nil {
		return nil, false, nil
	}
	parsed, err := attrs.Get(attribute.AttrASPath)
	if err != nil {
		return nil, false, errors.Join(malformed(attribute.AttrASPath, "segments"), err)
	}
	if parsed == nil {
		return nil, false, nil
	}
	v.decodes.countAttribute(attribute.AttrASPath)
	path, ok := parsed.(*attribute.ASPath)
	if !ok {
		// The attribute package answers AS_PATH with *ASPath by construction
		// (knownAttrParsers); another type is a Ze defect, and it fails the
		// route rather than reading as an empty path.
		return nil, false, errors.New("BUG: policyview: AS_PATH parsed to an unexpected type")
	}
	return path, true, nil
}

// requireFourOctet refuses an UPDATE still in the two-octet AS form. The
// ingest collapse (wireu.CollapseAS4Family) rewrites every received UPDATE to
// the four-octet form, merging AS4_PATH and AS4_AGGREGATOR under RFC 6793
// Section 4.2.3, and relabels its encoding context. A view over a two-octet
// UPDATE would need a second reconstruction beside that one, so it fails the
// route instead.
func (v *View) requireFourOctet() error {
	ctx, err := v.context()
	if err != nil {
		return err
	}
	if !ctx.ASN4() {
		return errTwoOctet
	}
	return nil
}

func (v *View) context() (*bgpctx.EncodingContext, error) {
	ctx := bgpctx.Registry.Get(v.update.SourceCtxID())
	if ctx == nil {
		return nil, errNoContext
	}
	return ctx, nil
}

// malformed names the attribute and the broken rule. It runs only on the
// failure path.
func malformed(code attribute.AttributeCode, rule string) error {
	return &malformedError{code: code, rule: rule}
}

type malformedError struct {
	code attribute.AttributeCode
	rule string
}

func (e *malformedError) Error() string {
	return e.code.String() + " is malformed on the wire: " + e.rule
}

func (e *malformedError) Unwrap() error { return ErrMalformed }
