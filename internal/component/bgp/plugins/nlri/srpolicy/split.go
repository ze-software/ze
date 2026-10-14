// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI splitting and receive syntax rule
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format; Sections 2.1, 4.2.1 and 5
// Related: types.go -- SRPolicy type and family constants
// Related: register.go -- splitter and syntax rule registration via init()

package srpolicy

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// SR Policy NLRI, RFC 9830 Section 2.1 (Figure 1), one entry of an MP NLRI section:
//
//	offset  size        field
//	[0..4)  4 octets    Path Identifier (RFC 7911, only when ADD-PATH is negotiated)
//	[p+0]   1 octet     NLRI Length, in bits
//	[p+1]   4 octets    Distinguisher
//	[p+5]   4 octets    Policy Color
//	[p+9]   4 or 16     Endpoint (IPv4 for AFI 1, IPv6 for AFI 2)
//
// p is 4 under ADD-PATH and 0 otherwise.
const (
	pathIDOctets = 4
	nlriBitsIPv4 = 96  // RFC 9830 Section 2.1: "When AFI = 1, the value MUST be 96"
	nlriBitsIPv6 = 192 // RFC 9830 Section 2.1: "when AFI = 2, the value MUST be 192"
)

// srPolicyNLRIBounds frames the entry that starts at off. lengthAt is the offset of its
// NLRI Length octet, past the path identifier when addPath is set, and end is one past its
// last octet. A non-nil error means the section cannot be walked from off onward.
//
// Every successful call returns end > off, so a loop advancing off to end is bounded by
// len(data).
func srPolicyNLRIBounds(data []byte, off int, addPath bool) (lengthAt, end int, err error) {
	lengthAt = off
	if addPath {
		lengthAt += pathIDOctets
		if lengthAt >= len(data) {
			return 0, 0, fmt.Errorf("srpolicy: path identifier at offset %d is cut short (have %d octets)", off, len(data)-off)
		}
	}
	lenBits := int(data[lengthAt])
	if lenBits == 0 {
		return 0, 0, fmt.Errorf("srpolicy: zero-length NLRI at offset %d", off)
	}
	if lenBits%8 != 0 {
		return 0, 0, fmt.Errorf("srpolicy: length %d bits is not byte-aligned at offset %d", lenBits, off)
	}
	end = lengthAt + 1 + lenBits/8
	if end > len(data) {
		return 0, 0, fmt.Errorf("srpolicy: NLRI at offset %d extends past data (need %d, have %d)", off, end-off, len(data)-off)
	}
	return lengthAt, end, nil
}

// SplitSRPolicy is the nlrisplit.Splitter for SR-Policy NLRIs (SAFI 73,
// RFC 9830).
//
// Each NLRI is framed as [path-id:4, under ADD-PATH][length-bits:1][body:length-bits/8].
// ADD-PATH is honored: a peer that negotiated it for the family (Ze's default add-path
// direction applies to every multiprotocol family) sends a path identifier before every
// NLRI, and each visited slice starts with it. The splitter judges framing only; the
// per-AFI body length is the registered syntax rule's (judgeSRPolicySection).
//
// It visits each NLRI in wire order, calls fn for each when fn is non-nil, and
// returns how many it visited. A false callback result stops after counting the
// current entry, with nil error and without inspecting the remaining bytes.
// A nil callback validates and counts the entire input. Slices alias `data`
// (zero-copy), so fn MUST copy what it retains. A malformed entry reached by the
// walk returns the count of the entries visited before it plus a non-nil error.
func SplitSRPolicy(data []byte, addPath bool, fn func(nlri []byte) bool) (int, error) {
	count := 0
	for off := 0; off < len(data); {
		_, end, err := srPolicyNLRIBounds(data, off, addPath)
		if err != nil {
			return count, err
		}
		count++
		if fn != nil {
			if !fn(data[off:end]) {
				return count, nil
			}
		}
		off = end
	}
	return count, nil
}

// srPolicySyntaxRule returns the registered nlrisyntax rule for the AFI whose NLRI Length
// is nlriBits.
func srPolicySyntaxRule(nlriBits int) nlrisyntax.Rule {
	return nlrisyntax.Rule{
		Citation:       "RFC 9830 Section 5",
		RemovedMessage: "RFC 9830 Section 5: removed SR Policy NLRIs of the wrong length",
		Judge: func(section []byte, addPath bool) nlrisyntax.Judgement {
			return judgeSRPolicySection(section, addPath, nlriBits)
		},
		Retain: func(section []byte, addPath bool) ([]byte, int, bool) {
			return retainSRPolicySection(section, addPath, nlriBits)
		},
	}
}

// judgeSRPolicySection answers RFC 9830 Section 5 for one MP NLRI section of SAFI 73.
//
// RFC 9830 Section 4.2.1: "The SR Policy NLRI MUST include a distinguisher, Color, and
// Endpoint field that implies that the length of the NLRI MUST be either 12 or 24 octets
// (depending on the address family of the Endpoint)." Section 2.1 ties the length to the
// AFI, so nlriBits is the one length this section's AFI admits.
//
// RFC 9830 Section 5: "An SR Policy update that is determined not to be valid (and,
// therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by
// the "treat-as-withdraw" strategy." That is the verdict for a walkable section holding an
// NLRI of the wrong length. A section that cannot be walked takes the other branch:
// "Alternately, the router MUST perform "session reset" when the session is only being
// used for SR Policy or when a "AFI/SAFI disable" action is not possible." Ze implements no
// AFI/SAFI disable, so that is always session reset.
//
// The whole section is walked before answering, so an unwalkable tail outranks a
// wrong length earlier in it. Allocates only on a malformed section.
func judgeSRPolicySection(section []byte, addPath bool, nlriBits int) nlrisyntax.Judgement {
	wrongBits := -1
	for off := 0; off < len(section); {
		lengthAt, end, err := srPolicyNLRIBounds(section, off, addPath)
		if err != nil {
			return nlrisyntax.Judgement{
				Action: nlrisyntax.ActionSessionReset,
				Reason: "RFC 9830 Section 5: SR Policy NLRI section cannot be walked: " + err.Error(),
			}
		}
		if wrongBits < 0 && int(section[lengthAt]) != nlriBits {
			wrongBits = int(section[lengthAt])
		}
		off = end
	}
	if wrongBits < 0 {
		return nlrisyntax.WellFormed
	}
	var b textbuf.Buffer
	return nlrisyntax.Judgement{
		Action: nlrisyntax.ActionTreatAsWithdraw,
		Reason: b.Reset().Str("RFC 9830 Section 5: SR Policy NLRI length ").Int(int64(wrongBits)).
			Str(" bits, the family requires ").Int(int64(nlriBits)).String(),
	}
}

// retainSRPolicySection returns section with every NLRI of the wrong length removed, so no
// synthesized withdrawal and no relay carries an NLRI Ze has judged malformed. It returns
// section itself when nothing was removed, and framed false when the section cannot be
// walked. Both passes are bounded by len(section).
func retainSRPolicySection(section []byte, addPath bool, nlriBits int) (kept []byte, dropped int, framed bool) {
	keepBytes := 0
	for off := 0; off < len(section); {
		lengthAt, end, err := srPolicyNLRIBounds(section, off, addPath)
		if err != nil {
			return section, 0, false
		}
		if int(section[lengthAt]) == nlriBits {
			keepBytes += end - off
		} else {
			dropped++
		}
		off = end
	}
	if dropped == 0 {
		return section, 0, true
	}
	if keepBytes == 0 {
		return nil, dropped, true
	}
	out := make([]byte, 0, keepBytes)
	for off := 0; off < len(section); {
		lengthAt, end, err := srPolicyNLRIBounds(section, off, addPath)
		if err != nil {
			return section, 0, false
		}
		if int(section[lengthAt]) == nlriBits {
			out = append(out, section[off:end]...)
		}
		off = end
	}
	return out, dropped, true
}
