// Design: docs/architecture/update-building.md — final outbound attribute admission
// Related: session_write.go — every final UPDATE body passes this guard after policy
// Related: ../message/rfc7607.go — the shared AS-zero field walkers
// RFC: rfc/short/rfc7607.md — Section 2 forbids originating or propagating AS zero

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const asZeroOutboundFix = "remove AS zero from the route source, plugin or export policy and send the corrected route again; " +
	"run ze show errors to find the affected peer and attribute"

// transmittedASZeroAttribute finds an AS-zero field in the final attribute
// block, including a repeated occurrence. The caller MUST hold writeMu, which
// protects negotiation. Internal AS-zero sentinels and NLRI are never examined.
//
// RFC 7607 Section 2: "A BGP speaker MUST NOT originate or propagate a route with an AS
// number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or
// AS4_AGGREGATOR attributes."
func (s *Session) transmittedASZeroAttribute(body []byte) (uint8, bool) {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		// Opaque diagnostics keep their existing framing/retirement contract;
		// untrusted offsets cannot identify a route attribute to rewrite.
		return 0, false
	}
	asn4 := false
	if s.negotiated != nil {
		asn4 = s.negotiated.ASN4
	}
	attrs := attribute.NewAttrIterator(sections.Attrs(body))
	for {
		code, _, value, ok := attrs.Next()
		if !ok {
			return 0, false
		}
		// RFC 7607 Section 2.
		if message.AttributeHasASZero(uint8(code), value, asn4) {
			return uint8(code), true
		}
	}
}

// writeASZeroAsWithdrawal withholds every announcement carrying the forbidden
// AS, withdrawing an older version instead of leaving it advertised. This is
// outbound admission, not received-attribute error handling: RFC 7606 and RFC
// 6793 still assign their distinct receive-side discard/withdraw actions.
//
// Original withdrawals and every MP family survive through the existing
// withdrawal synthesizer, with negotiated ADD-PATH framing. It drops all
// AS-bearing attributes, so re-entry into writeUpdateBody is bounded to one
// level. Raw diagnostic writes retain their raw ownership semantics; ordinary
// writes retain the final ownership filter and its duplicate suppression.
// The caller MUST hold writeMu.
//
// RFC 7607 Section 2: "A BGP speaker MUST NOT originate or propagate a route with an AS
// number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or
// AS4_AGGREGATOR attributes."
func (s *Session) writeASZeroAsWithdrawal(body []byte, code uint8, raw bool) error {
	if emit, suppressed := s.asZeroSendLog.allow(s.clock.Now().UnixNano()); emit {
		sessionLogger().Warn("route withdrawn instead of advertised: RFC 7607 Section 2 forbids AS zero",
			"peer", s.settings.Address, "attribute", code,
			"suppressed-since-last", suppressed, "fix", asZeroOutboundFix,
			"code", diagnostic.CodeDoctorBGPASZeroOutbound)
		// As with a write-budget event, this bounded report-bus append waits
		// on no peer or plugin while writeMu is held.
		report.RaiseErrorFix(reportSourceBGP, diagnostic.CodeDoctorBGPASZeroOutbound,
			s.settings.Address.String(), "BGP route withheld because an AS attribute contains zero; any older advertised version is withdrawn",
			asZeroOutboundFix, map[string]any{"attribute": code, "suppressed-since-last": suppressed})
	}
	ctx := bgpctx.Registry.Get(s.sendCtxID)
	var projection *fwdProvenance
	if item := s.sentForward; item != nil {
		var err error
		projection, err = projectFwdWithdrawals(item, body, ctx)
		if err != nil {
			return err
		}
		original, originalBody := item.provenance, item.writeBody
		item.provenance, item.writeBody = projection, 0
		defer func() {
			item.provenance, item.writeBody = original, originalBody
			projection.release()
		}()
	}
	splitter := message.GetSplitter()
	defer message.PutSplitter(splitter)
	emit := func(chunk *message.Update) error {
		if projection != nil {
			if err := projection.bindWithdrawalChunk(chunk, ctx); err != nil {
				return err
			}
		}
		n, err := chunk.CheckedWriteTo(s.writeBuf.Buffer(), 0, nil)
		if err != nil {
			return err
		}
		_, err = s.writeUpdateBody(s.writeBuf.Buffer()[message.HeaderLen:n], raw)
		return err
	}
	for _, withdrawal := range message.SynthesizeWithdrawFamilies(body, nil, s.mpAddPathSendWire) {
		update, err := message.UnpackUpdate(withdrawal)
		if err != nil {
			return err
		}
		// Receive-side synthesis may combine legacy withdrawals with one MP
		// family. Outbound RFC 7606 Section 5.1 permits only one NLRI-bearing
		// field per message, so split before returning to the final writer.
		if err := splitter.SplitCompliant(update, len(s.writeBuf.Buffer()), s.mpAddPathSend, emit); err != nil {
			return err
		}
	}
	return nil
}
