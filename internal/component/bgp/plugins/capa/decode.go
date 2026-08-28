// Design: docs/architecture/core-design.md -- core BGP capability decode
// Related: register.go -- plugin registration

package capa

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The JSON record keys a decoded capability answers with. A misspelling here
// drops the field from the record rather than failing, so the reader sees a
// capability with no name.
const (
	jsonKeyName  = "name"
	jsonKeyValue = "value"
)

var capNames = map[uint8]string{
	1:  "multiprotocol",
	5:  "extended-nexthop",
	6:  "extended-message",
	65: "asn4",
	69: "add-path",
	76: "paths-limit",
}

func runDecodeMode(input io.Reader, output io.Writer) int {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 || parts[0] != "decode" || parts[1] != "capability" {
			writeResponse(output, "decoded unknown\n")
			continue
		}

		codeVal, parseErr := strconv.ParseUint(parts[2], 10, 8)
		if parseErr != nil {
			writeResponse(output, "decoded unknown\n")
			continue
		}
		code := uint8(codeVal)

		var hexData string
		if len(parts) > 3 {
			hexData = parts[3]
		}

		result := decodeCapability(code, hexData)
		if result == nil {
			writeResponse(output, "decoded unknown\n")
			continue
		}

		jsonBytes, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			writeResponse(output, "decoded unknown\n")
			continue
		}
		var tb textbuf.Buffer
		writeResponse(output, tb.Str("decoded json ").Str(string(jsonBytes)).Byte('\n').String())
	}
	// bufio.Scanner reports a read failure and an over-long line through Err(),
	// never through Scan(). Without this the caller sees a clean, complete decode.
	if err := scanner.Err(); err != nil {
		var eb textbuf.Buffer
		writeResponse(output, eb.Str("decoded error ").Err(err).Byte('\n').String())
		return 1
	}
	return 0
}

func decodeCapability(code uint8, hexData string) map[string]any {
	name, ok := capNames[code]
	if !ok {
		return nil
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		return map[string]any{jsonKeyName: name}
	}

	switch code {
	case 1:
		return decodeMultiprotocol(name, data)
	case 5:
		return decodeExtendedNextHop(name, data)
	case 6:
		return map[string]any{jsonKeyName: name}
	case 65:
		return decodeASN4(name, data)
	case 69:
		return decodeAddPath(name, data)
	case 76:
		return decodePathsLimit(name, data)
	}
	return map[string]any{jsonKeyName: name}
}

func decodeMultiprotocol(name string, data []byte) map[string]any {
	if len(data) < 4 {
		return map[string]any{jsonKeyName: name}
	}
	afi := capability.AFI(binary.BigEndian.Uint16(data[0:2]))
	safi := capability.SAFI(data[3])
	var tb textbuf.Buffer
	return map[string]any{
		jsonKeyName:  name,
		jsonKeyValue: tb.Str(afi.String()).Byte('/').Str(safi.String()).String(),
	}
}

func decodeASN4(name string, data []byte) map[string]any {
	if len(data) < 4 {
		return map[string]any{jsonKeyName: name}
	}
	asn := binary.BigEndian.Uint32(data)
	return map[string]any{
		jsonKeyName:  name,
		jsonKeyValue: textbuf.StringUint(uint64(asn)),
	}
}

func decodeAddPath(name string, data []byte) map[string]any {
	if len(data)%4 != 0 {
		return map[string]any{jsonKeyName: name}
	}
	var tb textbuf.Buffer
	families := make([]string, len(data)/4)
	for i := range families {
		off := i * 4
		afi := capability.AFI(binary.BigEndian.Uint16(data[off:]))
		safi := capability.SAFI(data[off+2])
		families[i] = tb.Reset().Str(afi.String()).Byte('/').Str(safi.String()).String()
	}
	return map[string]any{jsonKeyName: name, jsonKeyValue: families}
}

func decodePathsLimit(name string, data []byte) map[string]any {
	if len(data)%5 != 0 {
		return map[string]any{jsonKeyName: name}
	}
	var tb textbuf.Buffer
	entries := make([]string, 0, len(data)/5)
	for i := 0; i+5 <= len(data); i += 5 {
		afi := capability.AFI(binary.BigEndian.Uint16(data[i:]))
		safi := capability.SAFI(data[i+2])
		limit := binary.BigEndian.Uint16(data[i+3:])
		entries = append(entries, tb.Reset().Str(afi.String()).Byte('/').Str(safi.String()).Byte(' ').Uint16(limit).String())
	}
	return map[string]any{jsonKeyName: name, jsonKeyValue: entries}
}

func decodeExtendedNextHop(name string, data []byte) map[string]any {
	if len(data)%6 != 0 {
		return map[string]any{jsonKeyName: name}
	}
	var tb textbuf.Buffer
	families := make([]string, len(data)/6)
	for i := range families {
		off := i * 6
		nlriAFI := capability.AFI(binary.BigEndian.Uint16(data[off:]))
		nlriSAFI := capability.SAFI(binary.BigEndian.Uint16(data[off+2:]))
		nhAFI := capability.AFI(binary.BigEndian.Uint16(data[off+4:]))
		families[i] = tb.Reset().Str(nlriAFI.String()).Byte('/').Str(nlriSAFI.String()).Str("->").Str(nhAFI.String()).String()
	}
	return map[string]any{jsonKeyName: name, jsonKeyValue: families}
}

func writeResponse(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil {
		return
	}
}
