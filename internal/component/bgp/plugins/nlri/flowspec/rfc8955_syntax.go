// Design: docs/architecture/wire/nlri-flowspec.md -- FlowSpec NLRI wire format
// Related: register.go -- registers flowSpecSyntaxRule for the four FlowSpec families
// RFC: rfc/short/rfc8955.md
// RFC: rfc/short/rfc8956.md

package flowspec

import (
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/family"
)

// flowSpecSyntaxRule returns the receive-path NLRI syntax rule for one FlowSpec
// family: every NLRI of an MP_REACH_NLRI or MP_UNREACH_NLRI section is decoded
// the way the plugin's own decoder reads it, before any of it reaches a plugin.
//
// Without it a malformed FlowSpec NLRI passed the receive path, the decoder
// refused it later, and the event's JSON writer fell back to the carrier's
// name: a script read the announce of a route `wire[ipv6/flow](49 bytes)`
// that the peer never sent.
func flowSpecSyntaxRule(fam family.Family) nlrisyntax.Rule {
	return nlrisyntax.Rule{
		Citation: "RFC 8955 Section 4.2",
		Judge: func(section []byte, addPath bool) nlrisyntax.Judgement {
			return judgeFlowSpecSection(fam, section, addPath)
		},
	}
}

// judgeFlowSpecSection answers for one FlowSpec NLRI section.
//
// RFC 8955 Section 4.2: "An NLRI value not encoded as specified here, including
// an NLRI that contains an unknown component type, is considered malformed and
// error handling according to Section 10 is performed." Section 10: "Error
// handling according to [RFC7606] and [RFC4760] applies to this specification."
// RFC 7606 Section 3 (j): "in order to use the approach of "treat-as-withdraw",
// the entire NLRI field and/or the MP_REACH_NLRI and MP_UNREACH_NLRI attributes
// need to be successfully parsed ... If this is not possible, the procedures of
// [RFC4271] and/or [RFC4760] continue to apply, meaning that the "session reset"
// approach (or the "AFI/SAFI disable" approach) MUST be followed." A FlowSpec
// NLRI is its own route key, so a malformed one names no route to withdraw, and
// Ze implements no AFI/SAFI disable: the verdict is session reset. ExaBGP 6.0
// raises Notify(3, 10) from the same decode (src/exabgp/bgp/message/update/nlri/
// flow.py), and BIRD calls bgp_parse_error for an invalid flow route
// (proto/bgp/packets.c).
//
// The section is framed by the family's splitter, so the walk is bounded by
// len(section). Allocates only on a malformed section.
func judgeFlowSpecSection(fam family.Family, section []byte, addPath bool) nlrisyntax.Judgement {
	parts, err := nlrisplit.Split(fam, section, addPath)
	if err != nil {
		return flowSpecMalformed(fam, "the section cannot be walked: "+err.Error())
	}
	for _, part := range parts {
		_, value, pathErr := nlri.SplitPathID(part, addPath)
		if pathErr != nil {
			return flowSpecMalformed(fam, pathErr.Error())
		}
		if parseErr := parseFlowSpecNLRI(fam, value); parseErr != nil {
			return flowSpecMalformed(fam, parseErr.Error())
		}
	}
	return nlrisyntax.WellFormed
}

// parseFlowSpecNLRI decodes one NLRI of fam with the parser decodeFlowSpecNLRI
// uses, so the receive path refuses exactly what the decoder cannot read.
func parseFlowSpecNLRI(fam family.Family, value []byte) error {
	if fam.SAFI == flowspec.SAFIFlowSpecVPN {
		_, err := flowspec.ParseFlowSpecVPN(fam, value)
		return err
	}
	_, err := flowspec.ParseFlowSpec(fam, value)
	return err
}

func flowSpecMalformed(fam family.Family, why string) nlrisyntax.Judgement {
	return nlrisyntax.Judgement{
		Action: nlrisyntax.ActionSessionReset,
		Reason: "RFC 8955 Section 4.2: malformed " + fam.String() + " NLRI: " + why,
	}
}
