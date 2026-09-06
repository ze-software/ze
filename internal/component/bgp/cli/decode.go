// Design: docs/architecture/core-design.md — BGP CLI commands
// Detail: decode_open.go — OPEN message decoding
// Detail: decode_update.go — UPDATE message decoding
// Detail: decode_mp.go — MP_REACH/MP_UNREACH NLRI decoding
// Detail: decode_plugin.go — plugin invocation for decode
// Detail: decode_human.go — human-readable output formatting

package cli

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

// Message type constants.
const (
	msgTypeOpen         = "open"
	msgTypeUpdate       = "update"
	msgTypeNLRI         = "nlri"
	msgTypeNotification = "notification"
	msgTypeKeepalive    = "keepalive"
)

// Keys of the decode output map. jsonKeyNLRI is the key that carries the route
// list inside an operation; msgTypeNLRI above is the name of the decode mode,
// so the two spellings stay separate names.
const (
	jsonKeyAction = "action"
	jsonKeyNLRI   = "nlri"
	jsonKeyParsed = "parsed"
	jsonKeyRaw    = "raw"
)

// capNameUnknown is the capability name used when the code has no decoder.
const capNameUnknown = "unknown"

// cmdDecode handles the 'decode' subcommand.
// Decodes BGP messages from hex and outputs Ze-format JSON.
func cmdDecode(args []string) int {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)

	openMsg := fs.Bool("open", false, "decode as OPEN message")
	updateMsg := fs.Bool("update", false, "decode as UPDATE message")
	notifMsg := fs.Bool("notification", false, "decode as NOTIFICATION message")
	keepaliveMsg := fs.Bool("keepalive", false, "decode as KEEPALIVE message")
	nlriFamily := fs.String("nlri", "", "decode as NLRI with family (e.g., 'ipv4/flow')")
	fam := fs.String("f", "", "address family for UPDATE (e.g., 'ipv4/unicast', 'l2vpn/evpn')")
	fs.StringVar(fam, "family", "", "address family for UPDATE (long form of -f)")
	outputJSON := fs.Bool("json", false, "output JSON instead of human-readable format")
	var plugins pluginFlags
	fs.Var(&plugins, "plugin", "plugin for capability/NLRI decoding (e.g., ze.hostname, flowspec)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze bgp decode",
			Summary: "Decode BGP message from hexadecimal and output Ze-format JSON",
			Usage: []string{
				"ze bgp decode [options] <hex-payload>",
				"ze bgp decode [options] -                Read one hex message for each line of standard input",
				"ze bgp decode [options] pcap <file>      Decode every BGP message in a capture",
				"ze bgp decode [options] pcap -           Read the capture from standard input",
			},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "--open", Desc: "Decode as OPEN message"},
					{Name: "--update", Desc: "Decode as UPDATE message"},
					{Name: "--notification", Desc: "Decode as NOTIFICATION message"},
					{Name: "--keepalive", Desc: "Decode as KEEPALIVE message"},
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
				"ze bgp decode pcap session.pcap                       Decode a tcpdump or ze capture",
				"tcpdump -w - port 179 | ze bgp decode pcap -           Decode a capture from a pipe",
			},
		}
		p.WriteErr()
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

	// Determine message type from flags
	var msgType string
	switch {
	case *openMsg:
		msgType = msgTypeOpen
	case *updateMsg:
		msgType = msgTypeUpdate
	case *notifMsg:
		msgType = msgTypeNotification
	case *keepaliveMsg:
		msgType = msgTypeKeepalive
	case *nlriFamily != "":
		msgType = msgTypeNLRI
	}

	// Use nlriFamily for NLRI mode, fall back to -f flag
	familyStr := *fam
	if *nlriFamily != "" {
		familyStr = *nlriFamily
	}

	// Three input forms, and the keyword decides between them before any value
	// is read (`ai/rules/cli.md`). A capture is named after `pcap`, hexadecimal
	// lines arrive on standard input under the "-" token, and a bare word is
	// the single hex payload this command has always taken.
	if fs.Arg(0) == inputKeywordPcap {
		return decodePcapInput(fs.Args()[1:], msgType, familyStr, *outputJSON)
	}
	if cliio.IsStdin(fs.Arg(0)) {
		if fs.NArg() > 1 {
			fmt.Fprintf(os.Stderr, "error: %q reads every message from standard input, so it takes no further argument, and %s given\n",
				cliio.StdinToken, countWord(fs.NArg()-1))
			return 1
		}
		return decodeHexStdin(msgType, familyStr, *outputJSON)
	}

	output, err := decodeHexPacket(fs.Arg(0), msgType, familyStr, *outputJSON)
	if err != nil {
		if *outputJSON {
			// Return valid JSON error
			errJSON := map[string]any{
				"error":       err.Error(),
				jsonKeyParsed: false,
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
	case msgTypeNotification:
		result, err = decodeNotificationMessage(data, hasHeader)
	case msgTypeKeepalive:
		result, err = decodeKeepaliveMessage(data, hasHeader)
	default:
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
		case msgTypeNotification:
			return formatNotificationHuman(result), nil
		case msgTypeKeepalive:
			return "KEEPALIVE", nil
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
	case 3:
		return msgTypeNotification
	case 4:
		return msgTypeKeepalive
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
