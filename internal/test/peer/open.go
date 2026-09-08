// Design: docs/architecture/testing/ci-format.md — the OPEN ze-peer sends
// Overview: peer.go — doOpenHandshake, which reads ze's OPEN and calls buildOpen
// Related: expect.go — the option= lines that declare each fact this file writes
// RFC: rfc/short/rfc6793.md — the two carriers of a speaker's AS in an OPEN
// RFC: rfc/short/rfc9234.md — the BGP Role capability and its pairing rule
// RFC: rfc/short/rfc7911.md — the ADD-PATH Send/Receive field
// RFC: rfc/short/rfc9072.md — the two-octet Optional Parameters Length
// RFC: rfc/short/rfc4271.md — the Hold Time field and its negotiation
// RFC: rfc/short/rfc4724.md — the Graceful Restart Restart Time and family tuples
// RFC: rfc/short/rfc9494.md — the Long-Lived Graceful Restart stale time
// RFC: rfc/short/rfc4760.md — the Multiprotocol capability that names the families
// RFC: rfc/short/draft-abraitis-idr-addpath-paths-limit.md — the Max Paths field
//
// ze-peer answers ze's OPEN with an OPEN of its own. The capability SET is
// mirrored from ze's, so a .ci that says nothing about capabilities still
// negotiates whatever ze offers. Every VALUE that describes the SENDER is
// resolved here instead, once, from the test's own configuration.
//
// A mirror is an assertion of sameness, so every field that describes the
// sender is wrong in a mirror by construction. The AS is checked by ze
// (validateOpenPeerAS, internal/component/bgp/reactor/session_open_as.go) and
// answered with NOTIFICATION 2/2. The role is checked (validateOpenRolePair,
// internal/component/bgp/plugins/role/validate.go) and answered with 2/11. The
// ADD-PATH direction is not checked: a mirrored direction intersects to
// AddPathNone in Negotiate (internal/core/bgp/capability/negotiated.go) and the
// session quietly negotiates ADD-PATH off. The FQDN is not checked at all.
//
// # Why this file walks capability TLVs instead of decoding them
//
// The spec chose capability.ParseFromOptionalParams for the read, and this file
// does not use it. Decoding and re-encoding is NOT octet-preserving:
// parsePathsLimit (internal/core/bgp/capability/capability.go) drops every entry
// whose limit is zero, and ParseFromOptionalParams drops every parameter whose
// type is not 2. A .ci asserting ze's reply to specific octets would then change
// verdict for a reason no test names. So the walk keeps each capability as raw
// octets, in the order it read them, and only the four codes whose VALUE
// describes the sender are decoded and rebuilt.
//
// # The RFC 9072 gap this file copies on purpose
//
// parseOpenParams detects the extended framing the way UnpackOpen
// (internal/component/bgp/message/open.go) detects it: on the Non-Ext OP Len
// octet being 255 AND the Non-Ext OP Type octet after it being 255. RFC 9072
// Section 3 puts the test on the TYPE alone, and UnpackOpen records that as a
// known gap under RFC9072-2-5, 2-6 and 3-1. The harness copies the gap so it
// frames exactly what ze frames. Correcting ze's detection therefore has to
// correct this function in the same change, or the harness misreads the first
// OPEN ze emits under the corrected rule.

package peer

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

const (
	// openBodyFixedLen is version(1) + My AS(2) + Hold Time(2) + BGP Identifier(4).
	// RFC 4271 Section 4.2 fixes every one of them.
	openBodyFixedLen = 9
	// paramTypeCapability is the RFC 5492 Section 4 Capabilities Optional
	// Parameter, the only parameter type that carries capability TLVs.
	paramTypeCapability = 2
	// openMsgMax is the largest OPEN, header included. RFC 4271 Section 4.1
	// states it, and RFC 8654 Section 4 keeps it: "The BGP Extended Message
	// Capability applies to all messages except for OPEN and KEEPALIVE messages."
	// So the bound holds whatever the session negotiates.
	openMsgMax = 4096
	// asTrans is the AS RFC 6793 Section 3 reserves for the My Autonomous System
	// field of a speaker whose real AS does not fit in two octets.
	asTrans = 23456
	// asMax is the largest AS number, RFC 6793 Section 3 having widened the field
	// to four octets.
	asMax = 4294967295
	// peerHostname is the name ze-peer answers with in the FQDN capability. The
	// mirror would answer with ze's, which is a claim about ze made in the test
	// peer's name.
	peerHostname = "ze-peer"
	// unknownCapabilityCode and unknownCapabilityValue are what
	// option=open:value=send-unknown-capability puts on the wire, to prove ze
	// ignores a capability it does not recognize (RFC 5492 Section 3).
	unknownCapabilityCode  = 66
	unknownCapabilityValue = "loremipsum"
	// peerSoftwareVersion is the version string ze-peer states in capability 75.
	// The mirror states ze's own build, which is a claim about ze made in the
	// test peer's name. It carries no version number after the name because the
	// harness has none: it is built from the checkout under test, so any number
	// here would be a second declaration of ze's version with nothing to keep
	// the two in step.
	peerSoftwareVersion = "ze-peer"
)

// The capability codes ze-peer resolves that internal/core/bgp/capability holds
// no constant for, because a plugin owns each one.
const (
	// capabilityCodeLLGR is Long-Lived Graceful Restart. RFC 9494 Section 3
	// assigns code 71, and the bgp-gr plugin builds ze's own value
	// (parseLLGRCapValue, internal/component/bgp/plugins/gr/gr_llgr.go).
	capabilityCodeLLGR capability.Code = 71
	// capabilityCodeSoftwareVersion is the software version capability.
	// draft-abraitis-bgp-version-capability assigns code 75, and the softver
	// plugin builds ze's own value (encodeValue,
	// internal/component/bgp/plugins/softver/softver.go).
	capabilityCodeSoftwareVersion capability.Code = 75
)

// The harness defaults, used for every fact a `.ci` states nothing about.
//
// Each one is a NUMBER the harness chose, never a number read out of ze's OPEN.
// That is the whole property: a file that declares nothing still asserts facts
// about ze-peer that ze-peer resolved, so no assertion in the suite can be
// satisfied by ze agreeing with itself.
const (
	// peerHoldTime is the Hold Time ze-peer proposes when no .ci declares one.
	//
	// RFC 4271 Section 4.2 makes the negotiated hold time the smaller of the two
	// advertised values, so the largest value the two-octet field can state is
	// the one value that can never be the smaller. Every existing test therefore
	// negotiates exactly what it negotiated under the mirror -- ze's own
	// configured hold time -- and only a .ci that declares a smaller one opts
	// into changing what ze holds.
	peerHoldTime = 65535
	// peerRestartTime is the Graceful Restart Restart Time ze-peer states when no
	// .ci declares one, in the 0..4095 RFC 4724 Section 3 gives the 12-bit field.
	// It is longer than any functional test runs, so a file that says nothing
	// about graceful restart never reaches the restart timer's expiry.
	peerRestartTime = 300
	// peerStaleTime is the Long-Lived Stale Time ze-peer states when no .ci
	// declares one, in the 0..16777215 RFC 9494 Section 3 gives the 24-bit field.
	peerStaleTime = 600
	// peerPathsLimit is the Max Paths ze-peer states for each family when no .ci
	// declares one. draft-abraitis-idr-addpath-paths-limit Section 4 makes the
	// receiver's limit a bound on what the sender may send, so the largest value
	// the two-octet field can state is the one that bounds nothing: a file that
	// says nothing about paths limits sees every path ze would otherwise send.
	peerPathsLimit = 65535
)

// openIdentity is every fact ze-peer's OPEN asserts about ze-peer.
//
// It is resolved before a single octet is written, so no fact can be written
// into one carrier and inherited into another. That shape, not a missed field,
// is what let one AS reach the My Autonomous System field while a second AS
// stayed in the four-octet AS capability.
type openIdentity struct {
	// as is the AS ze-peer claims, in 1..4294967295. It reaches BOTH carriers
	// RFC 6793 defines: the My Autonomous System field, narrowed to AS_TRANS
	// above 65535, and the Capability Value of capability 65.
	as uint32
	// routerID is the BGP Identifier ze-peer claims.
	routerID uint32
	// holdTime is the Hold Time ze-peer proposes, RFC 4271 Section 4.2.
	holdTime uint16
	// gracefulRestart is ze-peer's own code-64 value: the Restart Time, the
	// families whose forwarding state it preserves, and the F bit.
	gracefulRestart GracefulRestartDecl
	// llgr is ze-peer's own code-71 value: the Long-Lived Stale Time, the
	// families, and the F bit.
	llgr LLGRDecl
	// pathsLimit is ze-peer's own code-76 entries, one Max Paths per family.
	pathsLimit []PathsLimitDecl
}

// newOpenIdentity is the only place a sender-fact takes its harness default.
//
// Every fact has one here, so no caller can leave one at the Go zero value and
// no zero can be read two ways: a Restart Time of 0 means RFC 4724 Section 3's
// "the peer will not preserve forwarding state", and it must reach the wire only
// because a .ci asked for it (ai/rules/principles.md).
//
// The family lists stay empty here on purpose. They are the one fact that cannot
// be resolved before ze's OPEN is read, because ze-peer's own families are the
// mirrored capability SET, and resolveFamilies fills them.
func newOpenIdentity(as, routerID uint32) openIdentity {
	return openIdentity{
		as:              as,
		routerID:        routerID,
		holdTime:        peerHoldTime,
		gracefulRestart: GracefulRestartDecl{RestartTime: peerRestartTime, ForwardState: true},
		llgr:            LLGRDecl{StaleTime: peerStaleTime, ForwardState: true},
	}
}

// resolveFamilies fills every family list a `.ci` left unstated with the
// families ze-peer's own OPEN advertises.
//
// The default is ze-peer's OWN capability SET, which is a mirror of ze's by
// design and is therefore what ze-peer itself asserts. It is not ze's code-64
// family list: that one carries no families at all (parseGRCapValue,
// internal/component/bgp/plugins/gr/gr.go), so inheriting it would leave every
// receiving path guarded on an empty set.
func (id *openIdentity) resolveFamilies(advertised []family.Family) {
	if len(id.gracefulRestart.Families) == 0 {
		id.gracefulRestart.Families = advertised
	}
	if len(id.llgr.Families) == 0 {
		id.llgr.Families = advertised
	}
	if len(id.pathsLimit) > 0 {
		return
	}
	for _, fam := range advertised {
		id.pathsLimit = append(id.pathsLimit, PathsLimitDecl{Family: fam, Limit: peerPathsLimit})
	}
}

// advertisedFamilies is the family set ze-peer's OPEN carries, read from the
// Multiprotocol capabilities it mirrors.
//
// RFC 4760 Section 8 gives capability 1 a four-octet value of AFI(2),
// Reserved(1) and SAFI(1). A speaker that sends none is an IPv4 unicast speaker,
// which RFC 4271 defines with no capability at all, so that is what the empty
// read means rather than "this speaker has no families".
func advertisedFamilies(read openParams) []family.Family {
	var families []family.Family
	for _, param := range read.params {
		for _, tlv := range param.caps {
			if capability.Code(tlv[0]) != capability.CodeMultiprotocol || len(tlv) != 6 {
				continue
			}
			families = append(families, family.Family{
				AFI:  family.AFI(binary.BigEndian.Uint16(tlv[2:])),
				SAFI: family.SAFI(tlv[5]),
			})
		}
	}
	if len(families) > 0 {
		return families
	}
	return []family.Family{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}}
}

// openParam is one Optional Parameter of ze's OPEN, kept in the order it was read.
//
// A capability parameter is split into whole TLVs so the builder can replace the
// ones that describe the sender and leave every other octet alone. Any other
// parameter type is kept verbatim: ze-peer has no opinion about it and re-encoding
// what it did not decode is how octets drift.
type openParam struct {
	kind byte
	caps [][]byte
	raw  []byte
}

// openParams is ze's Optional Parameters block, decoded.
type openParams struct {
	// extended reports the RFC 9072 Section 2 framing, whose Parameter Length is
	// two octets. It cannot be derived from the octets: a one-octet and a
	// two-octet length are the same bytes read two ways.
	extended bool
	params   []openParam
}

// resolveOpenAS answers the AS ze-peer opens with on a connection whose two
// endpoint addresses are local and remote.
//
// A keyed declaration (option=asn:peer=<ip>) wins over an unkeyed one, because a
// single ze-peer process can serve several of ze's peers at once and each one is
// configured for its own AS: one process with option=conn_map accepts a batch of
// connections and answers each with its own OPEN. Either endpoint may carry the
// key. ze dials ze-peer at the address the peer's `connection { remote { ip } }`
// names, which ze-peer reads as the connection's LOCAL address, and ze-peer dials
// ze at the address `connection { local { ip } }` names, which it reads as the
// REMOTE address.
//
// It returns 0 and false when the .ci declared no AS at all. The caller then
// mirrors ze's own AS, which is what every .ci got before an AS could be
// declared, and is correct exactly when the session is iBGP.
//
// "Nothing was declared" is answered by its own boolean and never by the AS
// being zero. Reading absence out of the value would make the answer depend on
// every entry point refusing AS 0 forever, which is a guard nothing names and
// no test covers (ai/rules/principles.md).
func (c *Config) resolveOpenAS(local, remote netip.Addr) (uint32, bool) {
	var fallback uint32
	haveFallback := false
	for _, binding := range c.OpenAS {
		if !binding.Addr.IsValid() {
			fallback, haveFallback = binding.AS, true
			continue
		}
		if binding.Addr == local || binding.Addr == remote {
			return binding.AS, true
		}
	}
	return fallback, haveFallback
}

// validateOpenDeclarations refuses a combination of option= lines the builder
// cannot honor.
//
// Each one is a PAIR whose halves are legal apart and contradictory together, so
// neither line's own parser can see it. It runs twice, where the .ci is read and
// again in New, because the two halves can also arrive one from the command line
// and one from the file.
func (c *Config) validateOpenDeclarations() error {
	if err := c.validateOwnedCodesStatedOnce(); err != nil {
		return err
	}

	// TWO questions, and they are not the same one. Naming both is the point:
	// asking only the first is what let this guard fail open twice.
	//
	//   carriesTheAS -- does a line the .ci wrote CARRY the AS? Only a
	//   four-octet capability 65 does (statedAS).
	//
	//   suppressesOwnASN4 -- will ze-peer's OWN capability 65 stay off the wire?
	//   A drop does that, and so does ANY add-capability for code 65, because
	//   reconcileParams skips the builder's TLV for a code the .ci stated
	//   whatever its length. That is the precondition the AS_TRANS refusal
	//   below turns on.
	carriesTheAS, suppressesOwnASN4 := 0, false
	for _, override := range c.CapabilityOverrides {
		if capability.Code(override.Code) != capability.CodeASN4 {
			continue
		}
		// Both arms suppress: reconcileParams reads `stated` for any Add and
		// `dropped` for any drop, and skips the builder's own TLV for either.
		suppressesOwnASN4 = true
		if _, declares := statedAS(override); declares {
			carriesTheAS++
		}
	}

	// Two stated capability 65s put two ASNs on the wire, and the first
	// four-octet one silently becomes the AS the header field narrows from.
	// option=asn already refuses two unkeyed lines for this reason, and this is
	// the declaration that outranks it.
	if carriesTheAS > 1 {
		return fmt.Errorf(
			"the peer block states %d add-capability:code=65 lines carrying a four-octet AS, so its "+
				"OPEN would declare %d ASNs and ze would read whichever came first; state one",
			carriesTheAS, carriesTheAS)
	}
	// A stated capability 65 outranks option=asn, so a disagreement between them
	// is one AS declared twice with two answers, and the higher-precedence one
	// wins in silence. It is refused on exactly the argument the two-stated-65
	// refusal above rests on: any two declarations of the AS that disagree are a
	// question the block asked twice.
	if statedAS, isStated := statedOpenAS(c); isStated {
		for _, binding := range c.OpenAS {
			if binding.AS == statedAS {
				continue
			}
			return fmt.Errorf(
				"the peer block states add-capability:code=65 carrying AS %d and option=asn:value=%d, "+
					"so it declares the AS twice with two answers and the capability would win in "+
					"silence; state one",
				statedAS, binding.AS)
		}
	}

	// The precondition is that ze-peer's own capability 65 will not be emitted,
	// and the exemption is that something the .ci wrote carries the AS instead.
	// Reading either question off the other is how this failed open twice: first
	// by counting capability-65 LINES as AS declarations, then by asking only
	// whether one was DROPPED. A stated capability 65 of any length removes the
	// carrier exactly as a drop does.
	if !suppressesOwnASN4 || carriesTheAS > 0 {
		return nil
	}

	// RFC 6793 Section 3 puts AS_TRANS in the My Autonomous System field for a
	// speaker with no two-octet AS, so above 65535 capability 65 is the ONLY
	// carrier the real AS has. Dropping it leaves the peer claiming AS_TRANS,
	// which is a fact the harness cannot represent going out in silence.
	for _, binding := range c.OpenAS {
		if binding.AS <= 65535 {
			continue
		}
		// "drops or states", because both remove the carrier and a reader who
		// wrote only the add-capability line must recognize their own file here.
		// Naming the drop alone sent them looking for a line they never wrote.
		return fmt.Errorf(
			"the peer block asks for AS %d, and it drops or states capability 65 itself, so ze-peer "+
				"emits no four-octet AS capability of its own. RFC 6793 Section 3 makes that "+
				"capability the only carrier of an AS above 65535: the OPEN would claim AS_TRANS "+
				"(%d) and nothing would carry %d. Carry %d in the capability you state, or lower the AS",
			binding.AS, asTrans, binding.AS, binding.AS)
	}
	return nil
}

// validateOwnedCodesStatedOnce refuses a peer block that declares one
// capability twice: once as a typed option=open value, and once as raw octets in
// an add-capability or as a drop-capability.
//
// The two lines are legal apart and contradictory together, which is why neither
// line's own parser can see it. reconcileParams gives the stated octets
// precedence, so the typed line would be read, validated, and then dropped in
// silence -- the .ci author reads their restart time in the file and never on
// the wire.
func (c *Config) validateOwnedCodesStatedOnce() error {
	declared := map[byte]string{}
	if c.GracefulRestart != nil {
		declared[byte(capability.CodeGracefulRestart)] = optOpenGracefulRestart
	}
	if c.LLGR != nil {
		declared[byte(capabilityCodeLLGR)] = optOpenLLGR
	}
	if len(c.PathsLimit) > 0 {
		declared[byte(capability.CodePathsLimit)] = optOpenPathsLimit
	}

	for _, override := range c.CapabilityOverrides {
		option, isDeclared := declared[override.Code]
		if !isDeclared {
			continue
		}
		stated := "add-capability"
		if !override.Add {
			stated = "drop-capability"
		}
		return fmt.Errorf(
			"the peer block states option=open:value=%s and option=open:value=%s:code=%d, so it "+
				"declares capability %d twice and the stated octets would win in silence; state one",
			option, stated, override.Code, override.Code)
	}
	return nil
}

// parseOpenParams decodes the Optional Parameters of an OPEN body.
//
// RFC 9072 Section 2: when the Non-Ext OP Len octet is 255 and the Non-Ext OP
// Type octet after it is 255, the real length is the two octets that follow and
// every Parameter Length inside is two octets rather than one.
func parseOpenParams(body []byte) (openParams, error) {
	if len(body) < openBodyFixedLen+1 {
		return openParams{}, fmt.Errorf("the OPEN body is %d octets, want at least %d", len(body), openBodyFixedLen+1)
	}

	at := openBodyFixedLen + 1
	total := int(body[openBodyFixedLen])
	out := openParams{}

	if body[openBodyFixedLen] == message.ExtendedParamMarker && len(body) > at && body[at] == message.ExtendedParamMarker {
		if len(body) < openBodyFixedLen+4 {
			return openParams{}, fmt.Errorf("the OPEN body is %d octets, want at least %d for the RFC 9072 framing", len(body), openBodyFixedLen+4)
		}
		out.extended = true
		total = int(binary.BigEndian.Uint16(body[openBodyFixedLen+2:]))
		at = openBodyFixedLen + 4
	}

	end := at + total
	if end > len(body) {
		return openParams{}, fmt.Errorf("the Optional Parameters claim %d octets, the OPEN body holds %d", total, len(body)-at)
	}

	headerLen := 2
	if out.extended {
		headerLen = 3
	}

	for at < end {
		if at+headerLen > end {
			return openParams{}, fmt.Errorf("an Optional Parameter header at offset %d runs past the block", at)
		}
		param := openParam{kind: body[at]}
		paramLen := int(body[at+1])
		if out.extended {
			paramLen = int(binary.BigEndian.Uint16(body[at+1:]))
		}
		at += headerLen
		if at+paramLen > end {
			return openParams{}, fmt.Errorf("an Optional Parameter claims %d octets, the block holds %d", paramLen, end-at)
		}
		value := body[at : at+paramLen]
		at += paramLen

		if param.kind != paramTypeCapability {
			param.raw = value
			out.params = append(out.params, param)
			continue
		}

		caps, err := splitCapabilityTLVs(value)
		if err != nil {
			return openParams{}, err
		}
		param.caps = caps
		out.params = append(out.params, param)
	}

	return out, nil
}

// splitCapabilityTLVs cuts a Capabilities Optional Parameter into whole
// capability TLVs, header included, in the order they appear.
//
// RFC 5492 Section 4: the parameter value is one or more triples of Capability
// Code (1), Capability Length (1) and Capability Value.
func splitCapabilityTLVs(value []byte) ([][]byte, error) {
	var caps [][]byte
	at := 0
	for at < len(value) {
		if at+2 > len(value) {
			return nil, fmt.Errorf("capability header at offset %d runs past the parameter", at)
		}
		capLen := int(value[at+1])
		if at+2+capLen > len(value) {
			return nil, fmt.Errorf("capability %d claims %d octets, the parameter holds %d", value[at], capLen, len(value)-at-2)
		}
		caps = append(caps, value[at:at+2+capLen])
		at += 2 + capLen
	}
	return caps, nil
}

// buildOpen writes the OPEN ze-peer sends in answer to ze's.
//
// zeBody is ze's OPEN body. The capability SET comes from it; every value that
// describes the SENDER comes from id and cfg.
func buildOpen(zeBody []byte, id openIdentity, cfg *Config) ([]byte, error) {
	read, err := parseOpenParams(zeBody)
	if err != nil {
		return nil, fmt.Errorf("read ze's OPEN: %w", err)
	}

	id.resolveFamilies(advertisedFamilies(read))

	owned, err := ownedCapabilities(read, id)
	if err != nil {
		return nil, err
	}
	if err := refuseUnofferedDeclarations(owned, cfg); err != nil {
		return nil, err
	}

	params := reconcileParams(read.params, owned, cfg)
	params = append(params, addedParams(cfg)...)

	return encodeOpen(zeBody, id, params)
}

// reconcileParams rebuilds ze's parameter list with every asserted value owned
// and every drop-capability applied.
//
// A code an add-capability names is left out here: the .ci's own capability
// REPLACES the builder's, so the harness never adds a second copy of a code the
// test already stated. That is what keeps the four Role tests, and every other
// .ci that drops a code and adds it back, exactly as they were.
func reconcileParams(read []openParam, owned map[byte]ownedCapability, cfg *Config) []openParam {
	dropped := make(map[byte]bool, len(cfg.CapabilityOverrides))
	stated := make(map[byte]bool, len(cfg.CapabilityOverrides))
	for _, override := range cfg.CapabilityOverrides {
		if override.Add {
			stated[override.Code] = true
			continue
		}
		dropped[override.Code] = true
	}

	var out []openParam
	emitted := make(map[byte]bool, len(owned))
	for _, param := range read {
		if param.kind != paramTypeCapability {
			out = append(out, param)
			continue
		}
		kept := openParam{kind: param.kind}
		for _, tlv := range param.caps {
			code := tlv[0]
			if dropped[code] {
				continue
			}
			if stated[code] {
				continue
			}
			if own, ours := owned[code]; ours {
				kept.caps = append(kept.caps, own.tlv)
				emitted[code] = true
				continue
			}
			kept.caps = append(kept.caps, tlv)
		}
		if len(kept.caps) == 0 {
			continue
		}
		out = append(out, kept)
	}

	// A capability ze never offered, that ze-peer must nevertheless assert,
	// becomes its own parameter. Only the four-octet AS reaches this today: above
	// 65535 the My Autonomous System field cannot carry the AS at all.
	for code, own := range owned {
		if !own.add || emitted[code] || dropped[code] || stated[code] {
			continue
		}
		out = append(out, openParam{kind: paramTypeCapability, caps: [][]byte{own.tlv}})
	}

	return out
}

// addedParams turns each add-capability, and send-unknown-capability, into its
// own Capabilities Optional Parameter, which is where the mirroring harness put
// them and what every .ci asserting ze's answer was written against.
func addedParams(cfg *Config) []openParam {
	var out []openParam
	for _, override := range cfg.CapabilityOverrides {
		if !override.Add {
			continue
		}
		tlv := make([]byte, 2+len(override.Value))
		tlv[0] = override.Code
		tlv[1] = byte(len(override.Value)) //nolint:gosec // parseOptionConfig refuses a value above 255 octets
		copy(tlv[2:], override.Value)
		out = append(out, openParam{kind: paramTypeCapability, caps: [][]byte{tlv}})
	}

	if !cfg.SendUnknownCapability {
		return out
	}
	// RFC 5492 Section 3: a receiver MUST ignore a capability it does not
	// recognize, and code 66 is unassigned.
	tlv := make([]byte, 2+len(unknownCapabilityValue))
	tlv[0] = unknownCapabilityCode
	tlv[1] = byte(len(unknownCapabilityValue))
	copy(tlv[2:], unknownCapabilityValue)
	return append(out, openParam{kind: paramTypeCapability, caps: [][]byte{tlv}})
}

// encodeOpen writes the whole OPEN: the header, the fixed body fields resolved
// from id, and the parameters.
//
// RFC 9072 Section 2 framing is used when the parameters exceed the 255 octets a
// one-octet length can state, and refused above the 65535 a two-octet one can.
// Truncating is the failure this replaces: the harness wrote the low octet of
// the real length and sent a message whose first parameter no reader could frame.
func encodeOpen(zeBody []byte, id openIdentity, params []openParam) ([]byte, error) {
	extended := false
	paramsLen := 0
	for _, param := range params {
		paramsLen += 2 + paramValueLen(param)
	}
	if paramsLen > 255 {
		extended = true
		paramsLen = 0
		for _, param := range params {
			paramsLen += 3 + paramValueLen(param)
		}
	}
	envelopeLen := 1
	if extended {
		envelopeLen = 4
	}
	total := HeaderLen + openBodyFixedLen + envelopeLen + paramsLen
	// The MESSAGE is what the two-octet Length field states, so the message is
	// what the bound is on. Bounding the parameters alone leaves 65504 to 65535
	// octets of parameters passing the check and the header length wrapping,
	// which sends a frame no reader can cut at the right place.
	//
	// RFC 4271 Section 4.1: "The value of the Length field MUST always be at
	// least 19 and no greater than 4096". RFC 8654 Section 6 raises that number
	// to 65535 "for all messages except for OPEN and KEEPALIVE messages", so an
	// OPEN stays at 4096 however the Extended Message capability negotiates.
	if total > openMsgMax {
		return nil, fmt.Errorf("the reconciled OPEN is %d octets, RFC 4271 Section 4.1 states at most %d", total, openMsgMax)
	}
	open := make([]byte, total)

	copy(open, Marker)
	binary.BigEndian.PutUint16(open[16:], uint16(total)) //nolint:gosec // bounded by openMsgMax above
	open[18] = MsgOPEN

	body := open[HeaderLen:]
	body[0] = zeBody[0] // BGP version, mirrored: ze-peer speaks the version ze offered.
	binary.BigEndian.PutUint16(body[1:], myASField(id.as))
	// RFC 4271 Section 4.2 makes the Hold Time the SENDER's proposal, and the
	// negotiation takes the smaller of the two. Mirroring it made the two values
	// equal, so session_negotiate's min-selection had nothing to choose between
	// and could not be reached from any .ci.
	binary.BigEndian.PutUint16(body[3:], id.holdTime)
	binary.BigEndian.PutUint32(body[5:], id.routerID)

	at := openBodyFixedLen
	if extended {
		body[at] = message.ExtendedParamMarker
		body[at+1] = message.ExtendedParamMarker
		binary.BigEndian.PutUint16(body[at+2:], uint16(paramsLen)) //nolint:gosec // bounded by optParamsMax above
		at += 4
	} else {
		body[at] = byte(paramsLen)
		at++
	}

	for _, param := range params {
		valueLen := paramValueLen(param)
		body[at] = param.kind
		if extended {
			binary.BigEndian.PutUint16(body[at+1:], uint16(valueLen)) //nolint:gosec // bounded by optParamsMax above
			at += 3
		} else {
			body[at+1] = byte(valueLen)
			at += 2
		}
		if param.kind != paramTypeCapability {
			copy(body[at:], param.raw)
			at += valueLen
			continue
		}
		for _, tlv := range param.caps {
			copy(body[at:], tlv)
			at += len(tlv)
		}
	}

	return open, nil
}

// myASField is the two-octet My Autonomous System field for an AS.
//
// RFC 6793 Section 3: AS_TRANS is placed there "if and only if the speaker does
// not have a (globally unique) two-octet AS number". Both OPEN builders in this
// package narrow through this one function, so neither can state an AS the other
// would state differently.
func myASField(as uint32) uint16 {
	if as > 65535 {
		return asTrans
	}
	return uint16(as) //nolint:gosec // the branch bounds it at 65535
}

// paramValueLen is the octet count of one parameter's value.
func paramValueLen(param openParam) int {
	if param.kind != paramTypeCapability {
		return len(param.raw)
	}
	total := 0
	for _, tlv := range param.caps {
		total += len(tlv)
	}
	return total
}

// openIdentity resolves every fact ze-peer's OPEN asserts about ze-peer, from
// the test's own configuration, before the OPEN is built.
func (p *Peer) openIdentity(zeBody []byte, conn net.Conn) openIdentity {
	id := newOpenIdentity(p.openAS(zeBody, conn), peerRouterID(zeBody))
	// Each declaration REPLACES the harness default whole. A .ci that states a
	// Restart Time and no family is stating the time alone, and resolveFamilies
	// fills the list from ze-peer's own advertised families once ze's OPEN is
	// read.
	if p.config.HoldTime != nil {
		id.holdTime = *p.config.HoldTime
	}
	if p.config.GracefulRestart != nil {
		id.gracefulRestart = *p.config.GracefulRestart
	}
	if p.config.LLGR != nil {
		id.llgr = *p.config.LLGR
	}
	if len(p.config.PathsLimit) > 0 {
		id.pathsLimit = p.config.PathsLimit
	}
	// A capability 65 the .ci wrote out itself IS the AS declaration, and it wins
	// over every other. It names the octets that go on the wire, and RFC 6793
	// Section 4.1 makes those octets the ones a receiver reads, so letting the
	// header field come from anywhere else puts two ASNs in one OPEN -- the exact
	// disagreement this builder exists to make impossible.
	if as, stated := statedOpenAS(p.config); stated {
		id.as = as
	}
	// Explicit BGP Identifier override (option=open:value=router-id:id=...), for
	// tests that need an identifier the derived default can never produce: 0.0.0.0
	// (RFC 6286 Section 2.2), or ze's own over an iBGP session.
	if p.config.RouterID != nil {
		id.routerID = *p.config.RouterID
	}
	return id
}

// statedOpenAS reads the AS out of an add-capability the .ci wrote for code 65.
//
// Only the four-octet value RFC 6793 Section 3 defines is an AS declaration. A
// value of any other length is a malformed capability the test is driving on
// purpose, so it is sent as written and the My Autonomous System field keeps the
// AS resolved from the rest of the configuration. That is the one case where the
// two carriers legitimately differ, and it differs because the .ci asked it to.
func statedOpenAS(cfg *Config) (uint32, bool) {
	for _, override := range cfg.CapabilityOverrides {
		if as, declares := statedAS(override); declares {
			return as, true
		}
	}
	return 0, false
}

// statedAS answers the AS one capability override declares, and whether it
// declares one at all.
//
// This is the ONLY test for "does this line declare the AS", and every caller
// asks it here. A second spelling is what let the AS_TRANS refusal be disarmed:
// the validator counted every add-capability for code 65, this counted only a
// four-octet one, and a two-octet line therefore exempted a .ci from a check
// while carrying nothing that satisfied it.
//
// Note what it is NOT. "Did the .ci write a capability 65 itself" is a different
// question, asked by reconcileParams to decide whether to emit the builder's
// own, and a malformed capability the test is driving on purpose answers yes to
// that one and no to this one.
func statedAS(override CapabilityOverride) (uint32, bool) {
	if !override.Add || capability.Code(override.Code) != capability.CodeASN4 {
		return 0, false
	}
	// RFC 6793 Section 3 gives the Capability Value four octets. A value of any
	// other length is a malformed capability, not an AS.
	if len(override.Value) != 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(override.Value), true
}

// peerRouterID derives ze-peer's BGP Identifier from ze's, by incrementing the
// last octet. It stays distinct from ze's and stays derived from ze's, so no .ci
// that asserts a specific identifier changes.
func peerRouterID(zeBody []byte) uint32 {
	if len(zeBody) < openBodyFixedLen {
		return 0
	}
	zeID := binary.BigEndian.Uint32(zeBody[5:9])
	return zeID&0xFFFFFF00 | uint32((zeBody[8]+1)&0xFF)
}

// openAS answers the AS ze-peer opens with on this connection.
//
// With no option=asn anywhere in the peer block it mirrors ze's own AS, which is
// what every .ci got before an AS could be declared and is correct exactly when
// the session is iBGP. The mirror reads ze's AS through the RFC 6793 Section 4.1
// precedence rule, so the value it copies is the one ze itself asserts.
func (p *Peer) openAS(zeBody []byte, conn net.Conn) uint32 {
	var local, remote netip.Addr
	if conn != nil {
		local = addrOf(conn.LocalAddr())
		remote = addrOf(conn.RemoteAddr())
	}
	if as, declared := p.config.resolveOpenAS(local, remote); declared {
		return as
	}
	return zeAdvertisedAS(zeBody)
}

// addrOf reads the IP out of a net.Addr, or returns the zero Addr when it
// carries none.
func addrOf(addr net.Addr) netip.Addr {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return netip.Addr{}
	}
	out, ok := netip.AddrFromSlice(tcp.IP)
	if !ok {
		return netip.Addr{}
	}
	return out.Unmap()
}

// zeAdvertisedAS reads the AS ze asserts in the OPEN it sent.
//
// RFC 6793 Section 4.1: "When a NEW BGP speaker processes an OPEN message from
// another NEW BGP speaker, it MUST use the AS number encoded in the Capability
// Value field of the 'support for four-octet AS number capability' in lieu of
// the 'My Autonomous System' field of the OPEN message." openAdvertisedAS
// (internal/component/bgp/reactor/peer.go) applies the same rule to ze-peer's
// answer, so reading ze's AS any other way would mirror a value ze does not use.
func zeAdvertisedAS(zeBody []byte) uint32 {
	read, err := parseOpenParams(zeBody)
	if err == nil {
		for _, param := range read.params {
			for _, tlv := range param.caps {
				if capability.Code(tlv[0]) != capability.CodeASN4 || len(tlv) != 6 {
					continue
				}
				if as := binary.BigEndian.Uint32(tlv[2:]); as > 0 {
					return as
				}
			}
		}
	}
	if len(zeBody) < 3 {
		return 0
	}
	return uint32(binary.BigEndian.Uint16(zeBody[1:3]))
}
