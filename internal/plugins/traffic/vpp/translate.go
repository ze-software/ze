// Design: docs/architecture/traffic/fw-7-traffic-vpp.md -- Translation contract
//
// Pure translation functions mapping ze traffic.InterfaceQoS types to VPP
// binapi structures. These functions contain no I/O and no references to
// api.Channel; the backend composes their outputs into actual binary API
// calls. Keeping translation pure lets us unit-test the wire-level
// parameters without a running VPP.
//
// Current scope: HTB and TBF policers plus protocol classify (filter
// protocol). DSCP and mark filter translations remain rejected at verify
// until their VPP pipelines land; see verify.go.

package trafficvpp

import (
	"errors"
	"fmt"

	"go.fd.io/govpp/binapi/policer"
	"go.fd.io/govpp/binapi/policer_types"

	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// policerNamePrefix is the "ze/"-namespace all backend-managed policer names
// carry so startup orphan cleanup can tell Ze policers from foreign ones.
const policerNamePrefix = "ze/"

// policerName builds the VPP policer name from interface and class as
// "ze/<iface>/<class>". VPP limits names to 64 bytes; the verifier rejects any
// class whose produced name exceeds the limit, so this function does not
// truncate. Lives here (no build tag) so the cross-platform verifier and the
// linux-only backend share one definition.
func policerName(ifaceName, className string) string {
	var tb textbuf.Buffer
	return tb.Str(policerNamePrefix).Str(ifaceName).Byte('/').Str(className).String()
}

// --- Classify (filter protocol) translation ---------------------------------
//
// VPP's classify/policer-classify feature examines a fixed-width byte
// window of each packet. The traffic backend attaches a per-family classify
// table (via PolicerClassifySetInterface) whose sessions steer matching
// packets to the class's policer, so only the configured protocol is
// policed (unfiltered classes stay on the egress policer-output path).
//
// Wire offsets are the GROUND TRUTH emitted by VPP's own CLI for
// `classify table mask l3 ip4|ip6 proto` (captured on VPP v25.10-release):
// the feature skips the first 16-byte vector (skip_n_vectors=1, covering the
// L2 header region) and matches within the following 16-byte vector, where
//
//	IPv4 protocol    -> match-vector byte 7  (absolute 23 = eth 14 + ip4 proto 9)
//	IPv6 next-header -> match-vector byte 4  (absolute 20 = eth 14 + ip6 nexthdr 6)
//
// The first traffic implementation matched at L3-relative offset 9 with
// skip=0; that indexed the wrong bytes on the policer-classify arc and the
// session never hit ("matched wrong offset", the historical failure this
// pipeline's golden vectors pin against regression).

// classifyFamily selects the IP version for a classify table, which
// determines both the match-vector byte offset and the ip4-table / ip6-table
// slot the table is bound to in PolicerClassifySetInterface.
type classifyFamily uint8

const (
	classifyIPv4 classifyFamily = iota
	classifyIPv6
)

const (
	// classifySkipVectors is skip_n_vectors for the traffic classify tables.
	// The mask/match cover the L2-inclusive window at absolute offsets, so no
	// vectors are skipped (VPP's classify_add_del_session validates the match
	// length against skip+match vectors; a short skipped match is rejected
	// INVALID_VALUE, so we encode absolute offsets in a full-width mask).
	classifySkipVectors = 0 //nolint:unused // consumed by classifyAddDelTable calls in classify_linux.go (linux-tagged); the darwin lint pass excludes that file so it reads as unused.
	// classifyVectorLen is the width of one classify vector in bytes.
	classifyVectorLen = 16
	// classifyMaskLen is the mask/match width: two vectors, enough to reach
	// the IPv4 protocol / IPv6 next-header byte from the L2 frame start.
	classifyMaskLen = 2 * classifyVectorLen
	// ipv4ProtocolByte is the absolute offset of the IPv4 protocol byte from
	// the classify frame start: Ethernet(14) + IPv4 protocol(9) = 23. VPP's
	// own CLI emits the equivalent window (skip 1 vector, byte 7).
	ipv4ProtocolByte = 23
	// ipv6NextHeaderByte is the absolute offset of the IPv6 next-header byte:
	// Ethernet(14) + IPv6 next-header(6) = 20.
	ipv6NextHeaderByte = 20

	// --- DSCP classify offsets (police-by-dscp, USER decision 2026-07-10) ---
	//
	// `filter dscp` classifies the DiffServ Code Point (top 6 bits of the IPv4
	// TOS byte / IPv6 traffic-class field) at its absolute (L2-inclusive) frame
	// offset and steers matching packets to the class policer -- the SAME
	// pipeline as `filter protocol`, NOT a QoS remark. Offsets confirmed
	// accepted by real VPP v25.10 (skip=0, 2-vector mask).
	//
	// IPv4: TOS byte at Ethernet(14) + IPv4 TOS(1) = absolute byte 15. DSCP is
	// the top 6 bits -> mask 0xFC, match dscp<<2.
	ipv4TosByte  = 15
	ipv4DscpMask = 0xFC
	// IPv6: the 8-bit traffic-class field straddles two frame bytes because it
	// is not byte-aligned in the first IPv6 word (Version|TC|FlowLabel).
	// Ethernet(14) + IPv6 first word: byte 14 = Version(4b) | TC[7:4],
	// byte 15 = TC[3:0] | FlowLabel-high(4b). DSCP = TC[7:2], so it occupies
	// byte14's low nibble (TC[7:4] = DSCP[5:2]) and byte15's top two bits
	// (TC[3:2] = DSCP[1:0]).
	ipv6TrafficClassHiByte = 14
	ipv6TrafficClassLoByte = 15
	ipv6DscpHiMask         = 0x0F // byte14 low nibble  = DSCP[5:2]
	ipv6DscpLoMask         = 0xC0 // byte15 top 2 bits  = DSCP[1:0]
	// maxDSCPValue is the largest DiffServ Code Point (6-bit field). Mirrors
	// dscp.MaxValue; the verifier bounds `filter dscp` values to this.
	maxDSCPValue = 63
)

// protocolClassifyVectors builds the (mask, match) vectors for a classify
// session matching a single IP protocol / next-header value at its absolute
// (L2-inclusive) offset. The vectors are classifyMaskLen bytes wide so VPP's
// session match-length validation (skip+match vectors) is satisfied.
func protocolClassifyVectors(fam classifyFamily, proto uint8) (mask, match []byte) {
	mask = make([]byte, classifyMaskLen)
	match = make([]byte, classifyMaskLen)
	off := ipv4ProtocolByte
	if fam == classifyIPv6 {
		off = ipv6NextHeaderByte
	}
	mask[off] = 0xff
	match[off] = proto
	return mask, match
}

// dscpClassifyVectors builds the (mask, match) vectors for a classify session
// matching a single DiffServ Code Point at its absolute (L2-inclusive) offset.
// Like protocolClassifyVectors the vectors are classifyMaskLen bytes wide with
// skip=0. IPv4 matches one byte (TOS); IPv6 matches across two bytes because the
// traffic-class field is not byte-aligned in the first IPv6 word.
func dscpClassifyVectors(fam classifyFamily, dscp uint8) (mask, match []byte) {
	mask = make([]byte, classifyMaskLen)
	match = make([]byte, classifyMaskLen)
	if fam == classifyIPv6 {
		mask[ipv6TrafficClassHiByte] = ipv6DscpHiMask
		mask[ipv6TrafficClassLoByte] = ipv6DscpLoMask
		match[ipv6TrafficClassHiByte] = (dscp >> 2) & 0x0F
		match[ipv6TrafficClassLoByte] = (dscp & 0x03) << 6
		return mask, match
	}
	mask[ipv4TosByte] = ipv4DscpMask
	match[ipv4TosByte] = dscp << 2
	return mask, match
}

// filterClassifyVectors dispatches a TrafficFilter to the classify (mask,
// match) vectors that steer matching packets to the class policer. ok is false
// for filter types that carry no VPP-native steering match (e.g. mark, which is
// rejected at verify); such filters never reach the apply path. The verifier
// bounds the value ranges (protocol 0-255, dscp 0-63), so the uint8 conversions
// here cannot overflow for verified input.
//
//nolint:unparam // mask IS consumed by the caller in classify_linux.go (linux-tagged); on the darwin lint pass that caller is excluded and the darwin-visible caller (translate_test.go) discards it, so unparam wrongly sees mask as always-unused.
func filterClassifyVectors(fam classifyFamily, f traffic.TrafficFilter) (mask, match []byte, ok bool) {
	switch f.Type {
	case traffic.FilterProtocol:
		m, mt := protocolClassifyVectors(fam, uint8(f.Value))
		return m, mt, true
	case traffic.FilterDSCP:
		m, mt := dscpClassifyVectors(fam, uint8(f.Value))
		return m, mt, true
	case traffic.FilterMark:
		// Mark carries no VPP-native steering match (Linux SKB fwmark has no
		// faithful equivalent); rejected at verify, never reaches the apply
		// path. Returned not-ok so a verifier bypass fails loudly rather than
		// programming a bogus table.
		return nil, nil, false
	}
	return nil, nil, false
}

// classSteers reports whether a class carries any filter that steers matching
// traffic to its policer (protocol or dscp). Such a class uses the ingress
// policer-classify pipeline instead of the egress policer-output binding. Mark
// filters do not steer (rejected at verify) and do not count. Lives here (no
// build tag) so both the cross-platform verifier and the linux-only backend
// share one definition.
func classSteers(cls traffic.TrafficClass) bool {
	for _, f := range cls.Filters {
		if f.Type == traffic.FilterProtocol || f.Type == traffic.FilterDSCP {
			return true
		}
	}
	return false
}

var errRatetokbpsRateMustBe0 = errors.New("rateToKbps: rate must be > 0")

// kbpsPerBps is the divisor for bps -> kbps conversion.
const kbpsPerBps = 1000

// maxBpsForKbpsFit is the largest bps value that fits in uint32 kbps after
// rounding up. Precomputed so rateToKbps can reject on this bound BEFORE
// adding the rounding constant, which otherwise wraps around for bps very
// close to 2^64.
const maxBpsForKbpsFit = uint64(^uint32(0)) * kbpsPerBps

// rateToKbps converts bps to kbps, rounding UP. Returns an error if the
// input would overflow uint32 kbps (approximately 4.29 Tbps) or would wrap
// the uint64 arithmetic.
func rateToKbps(bps uint64) (uint32, error) {
	if bps == 0 {
		return 0, errRatetokbpsRateMustBe0
	}
	if bps > maxBpsForKbpsFit {
		return 0, fmt.Errorf("rateToKbps: %d bps exceeds uint32 kbps range", bps)
	}
	kbps := (bps + kbpsPerBps - 1) / kbpsPerBps
	return uint32(kbps), nil
}

// burstMilliseconds is the window size used to translate a policer rate
// into a committed burst value. 100ms at the configured rate absorbs brief
// traffic spikes without letting long-term rate exceed CIR. Lives in
// this platform-agnostic file (burstBytes is called from here) so the
// package compiles on non-Linux; backend_linux.go has //go:build linux
// and cannot host shared constants.
const burstMilliseconds = 100

// minBurstBytes is the floor applied to burstBytes so even a 1kbps
// policer can admit one full packet before the token bucket underruns.
// Standard Ethernet MTU is 1500 bytes; we round up to 2048 to leave
// headroom for VLAN / tunnel encapsulation without making the computed
// window dominate at realistic rates.
const minBurstBytes = 2048

// burstBytes returns a committed-burst value in bytes sized to absorb
// burstMilliseconds of traffic at the given rate. Derivation:
//
//	bytes = kbps * 1000 / 8 * (burstMilliseconds / 1000)
//	      = kbps * burstMilliseconds / 8
//
// At the default 100ms window this is roughly kbps * 12.5 bytes, close
// to the typical tc/HTB default. No overflow risk: kbps is uint32, the
// product kbps * 100 fits in uint64 comfortably.
//
// A floor of minBurstBytes prevents the policer from dropping every
// packet at very low rates (below ~160 kbps at 100ms window, the raw
// formula produces less than one MTU of burst).
func burstBytes(kbps uint32) uint64 {
	b := uint64(kbps) * burstMilliseconds / 8
	if b < minBurstBytes {
		return minBurstBytes
	}
	return b
}

// policerFromClass builds a PolicerAddDel message for one TrafficClass.
// For HTB: two-rate three-color policer (2R3C RFC 2698) with CIR=Rate,
// EIR=Ceil, color-blind, conform=transmit, exceed=transmit, violate=drop.
// For TBF: single-rate two-color policer (1R2C) with CIR=EIR=Rate,
// conform=transmit, exceed=drop, violate=drop.
// Other qdisc types are a translation bug (the verifier should have
// rejected them at config-verify time).
func policerFromClass(cls traffic.TrafficClass, qdiscType traffic.QdiscType) (policer.PolicerAddDel, error) {
	cir, err := rateToKbps(cls.Rate)
	if err != nil {
		return policer.PolicerAddDel{}, fmt.Errorf("class %q Rate: %w", cls.Name, err)
	}

	if qdiscType != traffic.QdiscHTB && qdiscType != traffic.QdiscTBF {
		return policer.PolicerAddDel{}, fmt.Errorf("class %q: qdisc %s not translatable to policer", cls.Name, qdiscType)
	}

	var eir uint32
	polType := policer_types.SSE2_QOS_POLICER_TYPE_API_1R2C
	exceedAction := policer_types.SSE2_QOS_ACTION_API_DROP

	if qdiscType == traffic.QdiscHTB {
		if cls.Ceil == 0 {
			// HTB class without explicit ceil uses rate as ceiling.
			eir = cir
		} else {
			eir, err = rateToKbps(cls.Ceil)
			if err != nil {
				return policer.PolicerAddDel{}, fmt.Errorf("class %q Ceil: %w", cls.Name, err)
			}
		}
		polType = policer_types.SSE2_QOS_POLICER_TYPE_API_2R3C_RFC_2698
		exceedAction = policer_types.SSE2_QOS_ACTION_API_TRANSMIT
	} else {
		// QdiscTBF: single-rate two-color, EIR mirrors CIR.
		eir = cir
	}

	// Name is a placeholder here: the backend overwrites it with the
	// composed "ze/<iface>/<class>" form before sending the request.
	// The verifier enforces the 64-byte VPP limit on that composed
	// name, so no truncation is needed in the translator.
	return policer.PolicerAddDel{
		IsAdd:      true,
		Name:       cls.Name,
		Cir:        cir,
		Eir:        eir,
		Cb:         burstBytes(cir),
		Eb:         burstBytes(eir),
		RateType:   policer_types.SSE2_QOS_RATE_API_KBPS,
		RoundType:  policer_types.SSE2_QOS_ROUND_API_TO_UP,
		Type:       polType,
		ColorAware: false,
		ConformAction: policer_types.Sse2QosAction{
			Type: policer_types.SSE2_QOS_ACTION_API_TRANSMIT,
		},
		ExceedAction: policer_types.Sse2QosAction{
			Type: exceedAction,
		},
		ViolateAction: policer_types.Sse2QosAction{
			Type: policer_types.SSE2_QOS_ACTION_API_DROP,
		},
	}, nil
}
