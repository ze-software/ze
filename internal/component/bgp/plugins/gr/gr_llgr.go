// Design: docs/architecture/core-design.md — LLGR capability decode and format
// RFC: rfc/short/rfc9494.md — Long-Lived Graceful Restart
// Overview: gr.go — GR plugin entry point, event dispatch, and capability storage
// Related: gr_state.go — GR state machine (extended for LLGR in spec-llgr-2)

package gr

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// llgrFamily represents an AFI/SAFI entry in the LLGR capability.
// RFC 9494 Section 3: each tuple is 7 bytes: AFI(2) + SAFI(1) + Flags(1) + LLST(3).
type llgrFamily struct {
	AFI          uint16 `json:"afi"`
	SAFI         uint8  `json:"safi"`
	ForwardState bool   `json:"forward-state"`
	LLST         uint32 `json:"long-lived-stale-time"`
}

// llgrResult represents a decoded LLGR capability.
type llgrResult struct {
	Name     string       `json:"name"`
	Families []llgrFamily `json:"families,omitempty"`
}

// llgrPeerCap holds LLGR capability data extracted from a peer's OPEN message.
// Used by the state machine to determine LLST per family.
type llgrPeerCap struct {
	Families []llgrCapFamily
}

// llgrCapFamily represents one AFI/SAFI entry in a peer's LLGR capability.
type llgrCapFamily struct {
	Family       string // "ipv4/unicast", "ipv6/unicast", etc.
	ForwardState bool   // F-bit: peer preserved forwarding state during LLGR
	LLST         uint32 // Long-Lived Stale Time in seconds (0-16777215)
}

// decodeLLGR decodes LLGR capability wire bytes.
// RFC 9494 Section 3: Wire format is a sequence of 7-byte tuples:
//   - AFI (2 bytes) + SAFI (1 byte) + Flags (1 byte) + LLST (3 bytes)
//
// Unlike GR capability (code 64), there is no global header.
// Partial tuples (<7 bytes remaining) are silently ignored.
func decodeLLGR(data []byte) (*llgrResult, error) {
	// RFC 9494 Section 3: capability value is a sequence of 7-byte tuples.
	// A non-empty value that is not a multiple of 7 has trailing garbage,
	// but we still parse complete tuples and ignore the remainder.
	if len(data) > 0 && len(data) < 7 {
		return nil, fmt.Errorf("LLGR capability too short: need at least 7 bytes per tuple, got %d", len(data))
	}

	result := &llgrResult{
		Name: "long-lived-graceful-restart",
	}

	// RFC 9494 Section 3: parse 7-byte AFI/SAFI tuples
	remaining := data
	for len(remaining) >= 7 {
		afi := (uint16(remaining[0]) << 8) | uint16(remaining[1])
		safi := remaining[2]
		flags := remaining[3]
		// RFC 9494 Section 3: LLST is 24-bit unsigned integer (3 bytes big-endian)
		llst := (uint32(remaining[4]) << 16) | (uint32(remaining[5]) << 8) | uint32(remaining[6])

		result.Families = append(result.Families, llgrFamily{
			AFI:          afi,
			SAFI:         safi,
			ForwardState: (flags & 0x80) != 0, // F-bit is bit 0 (high bit of flags byte)
			LLST:         llst,
		})
		remaining = remaining[7:]
	}

	return result, nil
}

// llgrResultToPeerCap converts a decoded LLGR wire result to the state machine's
// capability representation, mapping AFI/SAFI numbers to family strings.
func llgrResultToPeerCap(r *llgrResult) *llgrPeerCap {
	peerCap := &llgrPeerCap{
		Families: make([]llgrCapFamily, 0, len(r.Families)),
	}
	for _, f := range r.Families {
		family := afiSAFIToFamily(f.AFI, f.SAFI)
		if family != "" {
			peerCap.Families = append(peerCap.Families, llgrCapFamily{
				Family:       family,
				ForwardState: f.ForwardState,
				LLST:         f.LLST,
			})
		}
	}
	return peerCap
}

// formatLLGRText formats LLGR capability as human-readable text.
func formatLLGRText(r *llgrResult) string {
	var sb strings.Builder
	sb.WriteString("long-lived-graceful-restart")
	for _, f := range r.Families {
		fmt.Fprintf(&sb, " afi=%d/safi=%d llst=%d", f.AFI, f.SAFI, f.LLST)
		if f.ForwardState {
			sb.WriteString("(F)")
		}
	}
	return sb.String()
}

// parseLLGRCapValue extracts LLGR capability hex from a capability map's
// "graceful-restart" entry. Returns "" if no LLGR config (no long-lived-stale-time).
// RFC 9494: LLGR capability code 71 encodes per-family LLST as 7-byte tuples.
func parseLLGRCapValue(capMap map[string]any, peerAddr string, families []string) string {
	if capMap == nil {
		return ""
	}
	grData, ok := capMap["graceful-restart"].(map[string]any)
	if !ok {
		return ""
	}

	// Check for long-lived-stale-time (LLST)
	llstRaw, ok := grData["long-lived-stale-time"]
	if !ok {
		return ""
	}

	var llst uint32
	switch v := llstRaw.(type) {
	case float64:
		llst = uint32(v)
	case string:
		if parsed, err := strconv.ParseUint(v, 10, 32); err == nil {
			llst = uint32(parsed)
		}
	}

	// RFC 9494: LLST is 24-bit (max 16777215)
	if llst > 16777215 {
		logger().Warn("long-lived-stale-time exceeds 24-bit max, clamping",
			"peer", peerAddr, "value", llst)
		llst = 16777215
	}

	// Build hex payload: 7 bytes per family tuple
	// Apply the same LLST to all negotiated families
	if len(families) == 0 {
		families = []string{"ipv4/unicast"}
	}

	var buf []byte
	for _, fam := range families {
		f, ok := nlri.ParseFamily(fam)
		if !ok {
			continue
		}
		// AFI (2 bytes) + SAFI (1 byte) + Flags (1 byte: F-bit=1) + LLST (3 bytes)
		buf = append(buf,
			byte(f.AFI>>8), byte(f.AFI),
			byte(f.SAFI),
			0x80, // F-bit set (forwarding state preserved)
			byte(llst>>16), byte(llst>>8), byte(llst),
		)
	}

	if len(buf) == 0 {
		return ""
	}

	return hex.EncodeToString(buf)
}

// extractLLGRCapabilities extracts LLGR capability declarations from BGP config.
// Returns CapabilityDecl entries for code 71 (one per peer that has LLGR configured).
func extractLLGRCapabilities(jsonStr string) []sdk.CapabilityDecl {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return nil
	}

	const llgrCapCode = 71
	var caps []sdk.CapabilityDecl

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any) {
		families := collectPeerFamilies(peerMap, groupMap)

		peerCapValue := parseLLGRCapValue(configjson.GetCapability(peerMap), peerAddr, families)

		var groupCapValue string
		if groupMap != nil {
			groupCapValue = parseLLGRCapValue(configjson.GetCapability(groupMap), peerAddr, families)
		}

		capValue := groupCapValue
		if peerCapValue != "" {
			capValue = peerCapValue
		}
		if capValue == "" {
			return
		}

		caps = append(caps, sdk.CapabilityDecl{
			Code:     llgrCapCode,
			Encoding: "hex",
			Payload:  capValue,
			Peers:    []string{peerAddr},
		})
		logger().Debug("llgr capability", "peer", peerAddr)
	})

	return caps
}

// collectPeerFamilies returns the address families configured for a peer.
// Checks peer-level "family" config, then group-level fallback.
// Returns ["ipv4/unicast"] as default if no families configured.
func collectPeerFamilies(peerMap, groupMap map[string]any) []string {
	if families := extractFamilies(peerMap); len(families) > 0 {
		return families
	}
	if groupMap != nil {
		if families := extractFamilies(groupMap); len(families) > 0 {
			return families
		}
	}
	return []string{"ipv4/unicast"}
}

// extractFamilies extracts family strings from a peer or group config map.
func extractFamilies(m map[string]any) []string {
	famRaw, ok := m["family"]
	if !ok {
		return nil
	}
	switch v := famRaw.(type) {
	case []any:
		var families []string
		for _, f := range v {
			if s, ok := f.(string); ok {
				families = append(families, s)
			}
		}
		return families
	case string:
		return []string{v}
	}
	return nil
}

// decodeLLGRMode handles "decode capability 71 <hex>" in decode mode.
func decodeLLGRMode(format, hexData string, writeJSON func([]byte), writeText func(string), writeUnknown func()) {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	result, decErr := decodeLLGR(data)
	if decErr != nil {
		writeUnknown()
		return
	}

	if format == decodeFormatText {
		writeText(formatLLGRText(result))
	} else {
		jsonBytes, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			writeUnknown()
			return
		}
		writeJSON(jsonBytes)
	}
}

// runCLIDecodeLLGR decodes LLGR capability hex from CLI arguments.
func runCLIDecodeLLGR(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeOut(stderr, fmt.Sprintf("error: invalid hex: %v\n", err))
		return 1
	}

	result, decErr := decodeLLGR(data)
	if decErr != nil {
		writeOut(stderr, fmt.Sprintf("error: %v\n", decErr))
		return 1
	}

	if textOutput {
		writeOut(stdout, formatLLGRText(result)+"\n")
	} else {
		jsonBytes, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			writeOut(stderr, fmt.Sprintf("error: JSON encoding: %v\n", jsonErr))
			return 1
		}
		writeOut(stdout, string(jsonBytes)+"\n")
	}
	return 0
}
