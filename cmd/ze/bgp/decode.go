// Design: docs/architecture/core-design.md — BGP CLI commands
// Detail: decode_open.go — OPEN message decoding
// Detail: decode_update.go — UPDATE message decoding
// Detail: decode_mp.go — MP_REACH/MP_UNREACH NLRI decoding
// Detail: decode_plugin.go — plugin invocation for decode
// Detail: decode_human.go — human-readable output formatting

package bgp

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// Message type constants.
const (
	msgTypeOpen   = "open"
	msgTypeUpdate = "update"
	msgTypeNLRI   = "nlri"
)

// cmdDecode handles the 'decode' subcommand.
// Decodes BGP messages from hex and outputs Ze-format JSON.
func cmdDecode(args []string) int {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)

	openMsg := fs.Bool("open", false, "decode as OPEN message")
	updateMsg := fs.Bool("update", false, "decode as UPDATE message")
	nlriFamily := fs.String("nlri", "", "decode as NLRI with family (e.g., 'ipv4/flow')")
	family := fs.String("f", "", "address family for UPDATE (e.g., 'ipv4/unicast', 'l2vpn/evpn')")
	outputJSON := fs.Bool("json", false, "output JSON instead of human-readable format")
	var plugins pluginFlags
	fs.Var(&plugins, "plugin", "plugin for capability/NLRI decoding (e.g., ze.hostname, flowspec)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze bgp decode",
			Summary: "Decode BGP message from hexadecimal and output Ze-format JSON",
			Usage:   []string{"ze bgp decode [options] <hex-payload>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--open", Desc: "Decode as OPEN message"},
					{Name: "--update", Desc: "Decode as UPDATE message"},
					{Name: "--nlri <family>", Desc: "Decode as NLRI with family (e.g., 'ipv4/flow')"},
					{Name: "-f <family>", Desc: "Address family for UPDATE (e.g., 'ipv4/unicast', 'l2vpn/evpn')"},
					{Name: "--json", Desc: "Output JSON instead of human-readable format"},
					{Name: "--plugin <name>", Desc: "Plugin for capability/NLRI decoding (e.g., ze.hostname, flowspec)"},
				}},
			},
			Examples: []string{
				"ze bgp decode --open FFFF...                          Decode OPEN message",
				"ze bgp decode --update FFFF...                        Decode UPDATE message",
				"ze bgp decode --plugin ze.hostname --open FFFF...     Decode with hostname plugin",
				"ze bgp decode --nlri l2vpn/evpn 02...                 Decode NLRI with family",
				"ze bgp decode --plugin flowspec --nlri ipv4/flow 07...  Decode NLRI via plugin",
			},
		}
		p.Write()
		fmt.Fprintf(os.Stderr, "\nThe hex payload can include colons or spaces which will be stripped.\n")
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing hex payload\n")
		fs.Usage()
		return 1
	}

	payload := fs.Arg(0)

	// Determine message type from flags
	var msgType string
	switch {
	case *openMsg:
		msgType = msgTypeOpen
	case *updateMsg:
		msgType = msgTypeUpdate
	case *nlriFamily != "":
		msgType = msgTypeNLRI
	}

	// Use nlriFamily for NLRI mode, fall back to -f flag
	familyStr := *family
	if *nlriFamily != "" {
		familyStr = *nlriFamily
	}

	output, err := decodeHexPacket(payload, msgType, familyStr, *outputJSON)
	if err != nil {
		if *outputJSON {
			// Return valid JSON error
			errJSON := map[string]any{
				"error":  err.Error(),
				"parsed": false,
			}
			data, _ := json.Marshal(errJSON)
			fmt.Println(string(data))
		} else {
			// Human-readable error
			fmt.Println("Error:", err.Error())
		}
		return 1
	}

	fmt.Println(output)
	return 0
}

// decodeHexPacket decodes a hex BGP packet and returns formatted output.
// If outputJSON is true, returns JSON; otherwise returns human-readable format.
func decodeHexPacket(hexStr, msgType, family string, outputJSON bool) (string, error) {
	// Normalize hex input - remove colons, spaces, uppercase
	hexStr = strings.ReplaceAll(hexStr, ":", "")
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ToUpper(hexStr)

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	// Detect format: if FF*16 marker present, it's a full message
	// Otherwise assume UPDATE body
	hasHeader := hasValidMarker(data)

	if msgType == "" {
		if hasHeader {
			msgType = detectMessageType(data)
		} else {
			msgType = msgTypeUpdate // Default to UPDATE body
		}
	}

	// For NLRI-only mode, don't wrap in envelope
	if msgType == msgTypeNLRI {
		return decodeNLRIOnly(data, family, outputJSON)
	}

	// Build output based on message type
	var result map[string]any
	switch msgType {
	case msgTypeOpen:
		result, err = decodeOpenMessage(data, hasHeader)
	case msgTypeUpdate:
		result, err = decodeUpdateMessage(data, family, hasHeader)
	default: // Unsupported message type
		return "", fmt.Errorf("unsupported message type: %s", msgType)
	}

	if err != nil {
		return "", err
	}

	// Human-readable output
	if !outputJSON {
		switch msgType {
		case msgTypeOpen:
			return formatOpenHuman(result), nil
		case msgTypeUpdate:
			return formatUpdateHuman(result), nil
		}
	}

	// Ze format: {"type": "bgp", "bgp": {"type": "<event>", "peer": {...}, "<event>": {...}}}.
	envelope := makeZeEnvelope(msgType)
	bgp, _ := envelope["bgp"].(map[string]any)

	// Merge event-specific content into bgp.<event> section
	if eventContent, ok := result[msgType].(map[string]any); ok {
		bgp[msgType] = eventContent
	} else {
		// Fallback: use result directly as event content
		bgp[msgType] = result
	}

	jsonData, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}

	return string(jsonData), nil
}

// detectMessageType reads the BGP message type from the header.
func detectMessageType(data []byte) string {
	if len(data) < message.HeaderLen {
		return msgTypeUpdate
	}
	switch data[18] {
	case 1:
		return msgTypeOpen
	case 2:
		return msgTypeUpdate
	default:
		return msgTypeUpdate
	}
}

// makeZeEnvelope creates the Ze ze-bgp JSON envelope structure.
// Ze format: {"type": "bgp", "bgp": {"peer": {...}, "message": {..., "type": "<event>"}, "<event>": {...}}}.
// The message type can be determined either from message.type or by checking which key exists (open/update).
func makeZeEnvelope(msgType string) map[string]any {
	return map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"peer": map[string]any{
				"address": "127.0.0.1",
				"remote":  map[string]any{"as": 65533},
			},
			"message": map[string]any{
				"id":        0,
				"direction": "received",
				"type":      msgType,
			},
		},
	}
}

// hasValidMarker checks if data has the BGP marker (16 0xFF bytes).
func hasValidMarker(data []byte) bool {
	if len(data) < 16 {
		return false
	}
	for i := range 16 {
		if data[i] != 0xFF {
			return false
		}
	}
	return true
}
