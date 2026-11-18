// Design: docs/guide/graceful-restart.md -- Long-Lived Graceful Restart, per-family configuration
// RFC: rfc/short/rfc9494.md -- Section 5, procedures not enabled by default
// Overview: gr.go -- grPlugin, the OPEN and state handlers that call these
// Related: gr_llgr.go -- decodeLLGR, extractLLGRCapabilities (what Ze advertises)
// Related: gr_removal.go -- onPeerRemoved drops the record with the peer

package gr

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
)

// llgrCapabilityCode is the LLGR Capability code (RFC 9494 Section 3).
const llgrCapabilityCode = 71

// openParamCapabilities is the OPEN optional parameter type that carries capabilities
// (RFC 5492 Section 4).
const openParamCapabilities = 2

// grSentLLGR holds the original sent OPEN, including an empty family set.
// One record per actual owner bounds storage to the current connection.
type grSentLLGR struct {
	source   grSource
	families map[family.Family]bool
}

// connectionCurrentLocked fences state from a prior physical connection.
// The caller MUST hold gp.mu. Every actual session sends its OPEN first.
func (gp *grPlugin) connectionCurrentLocked(source grSource) bool {
	sent, ok := gp.sentLLGRFamilies[source.owner]
	if !ok {
		return false
	}
	return sent.source.connectionID == source.connectionID
}

// exchangedLLGRLocked returns the LLGR Capability source advertised, restricted to the
// AFI/SAFI that Ze declared in the LLGR Capability of the OPEN it sent on the same
// session. It returns nil when no family is left, which is the answer "the LLGR
// procedures do not apply to this peer". The caller MUST hold gp.mu.
//
// Ze declares code 71 for a family only when the operator configured
// long-lived-stale-time for that peer (extractLLGRCapabilities), so the families in
// Ze's own OPEN are the affirmative configuration RFC 9494 asks for, as the session
// actually carries it: a reload that changed the config does not change what this
// session exchanged.
func (gp *grPlugin) exchangedLLGRLocked(source grSource) *llgrPeerCap {
	received := gp.peerLLGRCaps[source.owner]
	if received == nil {
		return nil
	}
	if received.connectionID != source.connectionID {
		return nil
	}
	declared, ok := gp.sentLLGRFamilies[source.owner]
	if !ok {
		return nil
	}
	if declared.source.connectionID != source.connectionID {
		return nil
	}
	// RFC 9494 Section 5: "Implementations MUST NOT enable these procedures by default.
	// They MUST require affirmative configuration per AFI/SAFI in order to enable them."
	// A family the peer advertised but Ze did not declare is dropped here, so a peer
	// alone cannot turn on long-lived retention of its routes.
	var honored []llgrCapFamily
	for _, f := range received.Families {
		if declared.families[f.Family] {
			honored = append(honored, f)
		}
	}
	if len(honored) == 0 {
		return nil
	}
	return &llgrPeerCap{connectionID: source.connectionID, Families: honored}
}

// recordSentLLGR stores the families Ze declared in the LLGR Capability of the OPEN it
// sent to source. An OPEN with no LLGR Capability forgets the previous session's
// record, because only the latest OPEN describes the session in hand. Safe for
// concurrent use: it takes gp.mu.
func (gp *grPlugin) recordSentLLGR(source grSource, declared map[family.Family]bool) {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	if gp.sentLLGRFamilies == nil {
		gp.sentLLGRFamilies = make(map[uint64]grSentLLGR)
	}
	gp.sentLLGRFamilies[source.owner] = grSentLLGR{source: source, families: declared}
}

// handleStructuredSentOpen records the LLGR families of an OPEN Ze sent, read from its
// raw wire bytes. An OPEN that does not unpack records nothing for the session.
func (gp *grPlugin) handleStructuredSentOpen(source grSource, msg *bgptypes.RawMessage) {
	declared := make(map[family.Family]bool)
	if msg.RawBytes == nil {
		gp.recordSentLLGR(source, declared)
		return
	}
	open, err := message.UnpackOpen(msg.RawBytes)
	if err != nil {
		logger().Debug("gr: failed to unpack sent OPEN", "peer", source, "err", err)
		gp.recordSentLLGR(source, declared)
		return
	}

	params := open.OptionalParams
	offset := 0
	// Bounded by the parameter length, checked before each read.
	for offset+2 <= len(params) {
		paramType := params[offset]
		paramLen := int(params[offset+1])
		offset += 2
		if offset+paramLen > len(params) {
			break
		}
		if paramType == openParamCapabilities {
			addSentLLGRFamilies(source, params[offset:offset+paramLen], declared)
		}
		offset += paramLen
	}
	gp.recordSentLLGR(source, declared)
}

// addSentLLGRFamilies walks one capabilities parameter and adds to declared every
// family of each LLGR Capability it holds.
func addSentLLGRFamilies(source grSource, data []byte, declared map[family.Family]bool) {
	offset := 0
	// Bounded by the parameter length, checked before each read.
	for offset+2 <= len(data) {
		code := data[offset]
		capLen := int(data[offset+1])
		offset += 2
		if offset+capLen > len(data) {
			break
		}
		if code == llgrCapabilityCode {
			addLLGRFamilies(source, data[offset:offset+capLen], declared)
		}
		offset += capLen
	}
}

// handleSentOpenEvent records the LLGR families of an OPEN Ze sent, read from the JSON
// event's capability list.
func (gp *grPlugin) handleSentOpenEvent(source grSource, payload map[string]any) {
	declared := make(map[family.Family]bool)
	openObj, _ := payload["open"].(map[string]any)
	caps, _ := openObj["capabilities"].([]any)
	for _, capRaw := range caps {
		capObj, ok := capRaw.(map[string]any)
		if !ok {
			continue
		}
		code, _ := grCapabilityCode(capObj["code"])
		if int(code) != llgrCapabilityCode {
			continue
		}
		hexValue, _ := capObj["value"].(string)
		data, err := hex.DecodeString(hexValue)
		if err != nil {
			logger().Debug("gr: invalid sent cap 71 hex", "peer", source, "err", err)
			continue
		}
		addLLGRFamilies(source, data, declared)
	}
	gp.recordSentLLGR(source, declared)
}

// addLLGRFamilies decodes one LLGR Capability value and adds its families to declared.
func addLLGRFamilies(source grSource, value []byte, declared map[family.Family]bool) {
	result, err := decodeLLGR(value)
	if err != nil {
		logger().Debug("gr: failed to decode sent cap 71", "peer", source, "err", err)
		return
	}
	for _, f := range llgrResultToPeerCap(result).Families {
		declared[f.Family] = true
	}
}
