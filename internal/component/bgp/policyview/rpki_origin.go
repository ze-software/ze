// Design: docs/architecture/core-design.md -- Policy Filter Chain, effective origin AS for RPKI validation
// RFC: rfc/short/rfc8893.md -- Sections 1 and 3: effective origin AS after egress configuration and policy
// RFC: rfc/short/rfc6811.md -- Section 2: Route Origin ASN derivation
// Related: readers.go -- readOriginAS, reconstructedPath
// Related: register.go -- FieldRPKIValidationState registration

package policyview

import (
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// EffectiveOriginAS returns the Route Origin ASN used for RPKI Route Origin
// Validation (RFC 6811 Section 2, RFC 8893 Sections 1 and 3).
//
// RFC 8893 Section 1: "Any AS_PATH modifications resulting in effective origin
// AS change MUST be taken into account."
// RFC 8893 Section 3: "When applied to egress policy, validation state MUST be
// determined using the effective origin AS of the route as it will (or would)
// be announced to the peer."
//
// On import, this returns the received route's RFC 6811 Section 2 origin AS.
// On export, it reads the post-policy AS_PATH (reconstructing 2-octet AS_PATH
// with AS4_PATH when needed) and accounts for egress configuration that
// transforms the announced AS_PATH: as-override (replacing PeerAS with LocalAS
// on eBGP) and RFC 4271 / RFC 7705 eBGP AS prepending (SecondaryAS innermost,
// LocalAS outermost) when the path is empty or ends in a confederation segment.
// Returns ribpolicy.OriginNone when the final segment is an AS_SET or another
// non-AS_SEQUENCE / non-confederation type.
func (v *View) EffectiveOriginAS() (uint32, bool, error) {
	if v.update == nil {
		return 0, false, ErrNoUpdate
	}
	path, err := v.effectiveASPath()
	if err != nil {
		return 0, false, err
	}
	if !v.session.IsExport() {
		return v.importOriginAS(path)
	}
	return v.exportEffectiveOriginAS(path)
}

func (v *View) effectiveASPath() (*attribute.ASPath, error) {
	ctx, err := v.context()
	if err != nil {
		return nil, err
	}
	if ctx.ASN4() {
		path, _, err := v.reconstructedPath()
		return path, err
	}
	// Two-octet destination body (e.g. exportFilterFromFacts on a 2-byte peer):
	// reconstruct the 4-octet path from AS_PATH + AS4_PATH per RFC 6793 Section 4.2.3.
	attrs, err := v.attrs()
	if err != nil {
		return nil, err
	}
	if attrs == nil {
		return nil, nil
	}
	rawAS, err := attrs.GetRaw(attribute.AttrASPath)
	if err != nil {
		return nil, err
	}
	if len(rawAS) == 0 {
		return nil, nil
	}
	path, err := attribute.ParseASPath(rawAS, false)
	if err != nil {
		return nil, malformed(attribute.AttrASPath, "segments")
	}
	rawAS4, err := attrs.GetRaw(attribute.AttrAS4Path)
	if err == nil && len(rawAS4) > 0 {
		if as4, err := attribute.ParseAS4Path(rawAS4); err == nil {
			path = attribute.MergeAS4Path(path, as4)
		}
	}
	return path, nil
}

func (v *View) importOriginAS(path *attribute.ASPath) (uint32, bool, error) {
	if path != nil && len(path.Segments) > 0 {
		last := path.Segments[len(path.Segments)-1]
		if last.Type == attribute.ASSequence {
			if len(last.ASNs) == 0 {
				return 0, false, malformed(attribute.AttrASPath, "empty segment")
			}
			return last.ASNs[len(last.ASNs)-1], true, nil
		}
		if last.Type != attribute.ASConfedSequence && last.Type != attribute.ASConfedSet {
			return ribpolicy.OriginNone, true, nil
		}
	}
	if v.session.PeerKnown && v.session.Peer.LocalAS != 0 {
		return v.session.Peer.LocalAS, true, nil
	}
	ctx, err := v.context()
	if err != nil {
		return 0, false, err
	}
	if ctx.LocalASN() == 0 {
		return 0, false, errLocalASNUnset
	}
	return ctx.LocalASN(), true, nil
}

func (v *View) exportEffectiveOriginAS(path *attribute.ASPath) (uint32, bool, error) {
	peer := v.session.Peer
	isEBGP := v.session.EBGP
	if v.session.PeerKnown {
		isEBGP = !peer.IsIBGP
	}
	if path != nil && len(path.Segments) > 0 {
		last := path.Segments[len(path.Segments)-1]
		if last.Type == attribute.ASSequence {
			if len(last.ASNs) == 0 {
				return 0, false, malformed(attribute.AttrASPath, "empty segment")
			}
			origin := last.ASNs[len(last.ASNs)-1]
			// RFC 8893 Section 1 & Section 3: as-override replaces PeerAS with
			// LocalAS on eBGP transmission, changing the effective origin AS
			// when the rightmost AS equals PeerAS.
			if isEBGP && peer.ASOverride && peer.PeerAS != 0 && peer.LocalAS != 0 && origin == peer.PeerAS {
				origin = peer.LocalAS
			}
			return origin, true, nil
		}
		if last.Type != attribute.ASConfedSequence && last.Type != attribute.ASConfedSet {
			return ribpolicy.OriginNone, true, nil
		}
	}
	// Empty AS_PATH (or confederation-only path): on eBGP (non-RS-client) with
	// RFC 7705 migration SecondaryAS, SecondaryAS is prepended innermost and is
	// therefore the rightmost (effective origin) AS on the wire.
	if isEBGP && !peer.RSClient && peer.SecondaryAS != 0 {
		return peer.SecondaryAS, true, nil
	}
	if peer.LocalAS != 0 {
		return peer.LocalAS, true, nil
	}
	ctx, err := v.context()
	if err != nil {
		return 0, false, err
	}
	if ctx.LocalASN() == 0 {
		return 0, false, errLocalASNUnset
	}
	return ctx.LocalASN(), true, nil
}

// readRPKIValidationState serves policy.FieldRPKIValidationState on the BGP
// route view so both `bgp import` and `bgp export` carry the field.
func readRPKIValidationState(v *View) (Value, error) {
	origin, ok, err := v.EffectiveOriginAS()
	if err != nil {
		return Value{}, err
	}
	if ok {
		if origin == ribpolicy.OriginNone {
			return Value{present: true}, nil
		}
		return Value{present: true, number: uint64(origin), numbered: true}, nil
	}
	return Value{}, nil
}
