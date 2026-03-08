// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec stdin/stdout protocol dispatch
// RFC: rfc/short/rfc5575.md
// Overview: plugin.go — plugin entry points, CLI, families
// Related: plugin_decode.go — wire-to-JSON decoding and formatting
// Related: plugin_encode_text.go — text-to-wire encoding

package flowspec

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
)

// Protocol constants for request/response handling.
const (
	cmdEncode       = "encode"
	cmdDecode       = "decode"
	objTypeNLRI     = "nlri"
	fmtJSON         = "json"
	fmtText         = "text"
	respDecodedUnk  = "decoded unknown"
	respEncodedErr  = "encoded error "
	respEncodedHex  = "encoded hex "
	respDecodedJSON = "decoded json "
	respDecodedText = "decoded text "
)

// protocolWrite writes a string to the output writer.
// Protocol writes are fire-and-forget; pipe failure causes exit.
func protocolWrite(output io.Writer, s string) {
	_, err := io.WriteString(output, s)
	_ = err // Protocol writes are fire-and-forget; pipe failure terminates the process
}

// RunFlowSpecDecode runs the plugin in decode/encode mode for ze bgp decode/encode.
// Handles both decode and encode requests on stdin, writes responses to stdout.
//
// Decode formats:
//   - "decode nlri <family> <hex>" -> JSON (default)
//   - "decode json nlri <family> <hex>" -> JSON (explicit)
//   - "decode text nlri <family> <hex>" -> human-readable text
//
// Encode formats:
//   - "encode nlri <family> <components...>" -> text input (default)
//   - "encode text nlri <family> <components...>" -> text input (explicit)
//   - "encode json nlri <family> <json>" -> JSON input
//
// Response: "encoded hex <hex>" or "encoded error <msg>".
func RunFlowSpecDecode(input io.Reader, output io.Writer) int {
	writeUnknown := func() { protocolWrite(output, "decoded unknown\n") }
	writeError := func(msg string) { protocolWrite(output, "encoded error "+msg+"\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			writeUnknown()
			continue
		}

		cmd := parts[0]
		objType := parts[1]

		// Handle format specifier for decode/encode
		// Decode: default=json, Encode: default=text
		format := fmtJSON
		if cmd == cmdEncode {
			format = fmtText // encode defaults to text input
		}

		if objType == fmtJSON || objType == fmtText {
			format = objType
			// Shift parts: cmd format nlri -> cmd nlri (with format stored)
			if len(parts) < 4 {
				if cmd == cmdDecode {
					writeUnknown()
				} else {
					writeError("missing arguments")
				}
				continue
			}
			objType = parts[2]
			parts = append([]string{cmd, objType}, parts[3:]...)
		}

		switch {
		case cmd == cmdDecode && objType == objTypeNLRI:
			handleDecodeNLRI(parts, format, output, writeUnknown)
		case cmd == cmdEncode && objType == objTypeNLRI:
			if format == fmtJSON {
				handleEncodeNLRIFromJSON(parts, output, writeError)
			} else {
				handleEncodeNLRI(parts, output, writeError)
			}
		case cmd == cmdDecode:
			writeUnknown()
		case cmd == cmdEncode:
			writeError("unsupported object type")
		}
	}
	return 0
}

// handleDecodeNLRI handles: decode nlri <family> <hex>.
// Format parameter determines output: "json" or "text".
func handleDecodeNLRI(parts []string, format string, output io.Writer, writeUnknown func()) {
	if len(parts) < 4 {
		writeUnknown()
		return
	}

	family := strings.ToLower(parts[2])
	hexData := parts[3]

	if !isValidFlowSpecFamily(family) {
		writeUnknown()
		return
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		writeUnknown()
		return
	}

	result := decodeFlowSpecNLRI(family, data)
	if result == nil {
		writeUnknown()
		return
	}

	if format == "text" {
		text := formatFlowSpecText(result)
		protocolWrite(output, "decoded text "+text+"\n")
		return
	}

	// Default: JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		writeUnknown()
		return
	}
	protocolWrite(output, "decoded json "+string(jsonBytes)+"\n")
}

// handleEncodeNLRIFromJSON handles: encode json nlri <family> <json>.
// JSON format matches decode output: {"destination":[["10.0.0.0/24/0"]],...}.
func handleEncodeNLRIFromJSON(parts []string, output io.Writer, writeError func(string)) {
	if len(parts) < 4 {
		writeError("missing family or JSON")
		return
	}

	family := strings.ToLower(parts[2])
	if !isValidFlowSpecFamily(family) {
		writeError("invalid family: " + family)
		return
	}

	// JSON is the remaining part (may have been split by Fields if it had spaces)
	jsonStr := strings.Join(parts[3:], " ")

	// Parse JSON
	var jsonMap map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &jsonMap); err != nil {
		writeError("invalid JSON: " + err.Error())
		return
	}

	// Convert JSON to text components
	textArgs, err := jsonToTextComponents(jsonMap)
	if err != nil {
		writeError(err.Error())
		return
	}

	// Parse family
	fam, ok := nlri.ParseFamily(family)
	if !ok {
		writeError("unknown family: " + family)
		return
	}

	// Encode using existing text encoder
	wireBytes, err := EncodeFlowSpecComponents(fam, textArgs)
	if err != nil {
		writeError(err.Error())
		return
	}

	protocolWrite(output, "encoded hex "+strings.ToUpper(hex.EncodeToString(wireBytes))+"\n")
}

// handleEncodeNLRI handles: encode nlri <family> <components...>
// Components: destination <prefix> | source <prefix> | protocol <num> | port <op><num> | ...
func handleEncodeNLRI(parts []string, output io.Writer, writeError func(string)) {
	if len(parts) < 4 {
		writeError("missing family or components")
		return
	}

	family := strings.ToLower(parts[2])
	if !isValidFlowSpecFamily(family) {
		writeError("invalid family: " + family)
		return
	}

	// Parse family using nlri.ParseFamily (still in nlri package)
	fam, ok := nlri.ParseFamily(family)
	if !ok {
		writeError("unknown family: " + family)
		return
	}

	// Parse components from remaining args
	args := parts[3:]
	wireBytes, err := EncodeFlowSpecComponents(fam, args)
	if err != nil {
		writeError(err.Error())
		return
	}

	protocolWrite(output, "encoded hex "+strings.ToUpper(hex.EncodeToString(wireBytes))+"\n")
}
