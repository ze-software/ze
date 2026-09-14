// Design: docs/architecture/core-design.md — software-version capability plugin
//
// Package bgp_softver implements a software-version capability plugin for ze.
// It advertises the software version of the BGP speaker (code 75).
//
// draft-abraitis-bgp-version-capability: BGP Software Version Capability.
package softver

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/plugins/softver/yang"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Logger is the package-level logger, disabled by default.
var Logger = slogutil.DiscardLogger()

// ConfigureLogger sets the package-level logger.
func ConfigureLogger(l *slog.Logger) {
	if l != nil {
		Logger = l
	}
}

// ZeVersion is the software version string advertised in the capability.
// Convention: "name/version" (e.g., "ExaBGP/4.2.22", "FRRouting/9.0").
const configRootBGP = "bgp"

const ZeVersion = "Ze/0.1.0"

// Software-version capability mode values that suppress advertisement.
const (
	modeDisable = "disable"
	modeRefuse  = "refuse"
)

// encodeValue returns the hex-encoded capability value (without code/length prefix).
// draft-abraitis-bgp-version-capability: version-length (1 octet) + version-string (UTF-8).
func encodeValue() string {
	version := ZeVersion
	if len(version) > 255 {
		version = version[:255]
	}

	data := make([]byte, 1+len(version))
	data[0] = byte(len(version))
	copy(data[1:], version)

	return hex.EncodeToString(data)
}

// RunSoftverPlugin runs the softver plugin using the SDK RPC protocol.
func RunSoftverPlugin(conn net.Conn) int {
	Logger.Debug("softver plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-softver", conn)
	defer func() { _ = p.Close() }()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		var caps []sdk.CapabilityDecl
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			caps = append(caps, extractSoftverCapabilities(section.Data)...)
		}
		p.SetCapabilities(caps)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{configRootBGP},
	})
	if err != nil {
		Logger.Error("softver plugin failed", "error", err)
		return 1
	}

	return 0
}

// extractSoftverCapabilities parses bgp config JSON and returns per-peer software-version capabilities.
// Handles both standalone peers (bgp.peer) and grouped peers (bgp.group.<name>.peer).
func extractSoftverCapabilities(jsonStr string) []sdk.CapabilityDecl {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		Logger.Warn("invalid JSON in bgp config")
		return nil
	}

	const softverCapCode = 75
	var caps []sdk.CapabilityDecl

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		// Check per-peer software-version capability first.
		peerHasExplicit := false
		peerEnabled := false
		if svRaw, exists := configjson.GetCapability(peerMap)["software-version"]; exists {
			peerHasExplicit = true
			var mode string
			switch sv := svRaw.(type) {
			case map[string]any:
				mode, _ = sv["mode"].(string)
			case string:
				mode = sv
			case nil:
				// bare presence -- treat as enable
			}
			peerEnabled = mode != modeDisable && mode != modeRefuse
		}

		// Check group-level software-version capability (fallback).
		groupEnabled := false
		if groupMap != nil {
			if svRaw, exists := configjson.GetCapability(groupMap)["software-version"]; exists {
				var mode string
				switch sv := svRaw.(type) {
				case map[string]any:
					mode, _ = sv["mode"].(string)
				case string:
					mode = sv
				}
				groupEnabled = mode != modeDisable && mode != modeRefuse
			}
		}

		// Per-peer wins; if no per-peer config, use group default.
		enabled := groupEnabled
		if peerHasExplicit {
			enabled = peerEnabled
		}

		if !enabled {
			if peerHasExplicit {
				Logger.Debug("software-version capability suppressed by mode", "peer", peerAddr)
			}
			return
		}

		caps = append(caps, sdk.CapabilityDecl{
			Code:     softverCapCode,
			Encoding: sdk.CapEncodingHex,
			Payload:  encodeValue(),
			Peers:    []string{configjson.CapabilitySelector(peerAddr, origin)},
		})
		Logger.Debug("software-version capability enabled", "peer", peerAddr)
	})

	return caps
}

// GetYANG returns the embedded YANG for the softver plugin.
func GetYANG() string {
	return yang.ZeSoftverYANG
}

// RunDecodeMode runs the plugin in decode mode for ze bgp decode.
func RunDecodeMode(input io.Reader, output io.Writer) int {
	writeResponse := func(s string) {
		_, _ = io.WriteString(output, s)
	}
	writeUnknown := func() { writeResponse("decoded unknown\n") }
	writeJSON := func(j []byte) { writeResponse("decoded json " + string(j) + "\n") }
	writeText := func(t string) { writeResponse("decoded text " + t + "\n") }

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "decode" {
			writeUnknown()
			continue
		}

		format := "json"
		capIdx := 1
		if parts[1] == "json" || parts[1] == "text" {
			format = parts[1]
			capIdx = 2
			if len(parts) < 5 {
				writeUnknown()
				continue
			}
		}

		if parts[capIdx] != "capability" {
			writeUnknown()
			continue
		}

		if parts[capIdx+1] != "75" {
			writeUnknown()
			continue
		}

		hexData := parts[capIdx+2]
		data, err := hex.DecodeString(hexData)
		if err != nil {
			writeUnknown()
			continue
		}

		version, ok := decodeSoftwareVersion(data)
		if !ok {
			// Section 3's encoding error: a zero Capability Length, a value
			// shorter than its own length octet, or invalid UTF-8. Each is
			// reported as unknown, which is how this path ignores it.
			writeUnknown()
			continue
		}

		if format == "text" {
			writeText(fmt.Sprintf("%-20s %s", "software-version", version))
		} else {
			result := map[string]any{
				"name":    "software-version",
				"version": version,
			}
			jsonBytes, _ := json.Marshal(result)
			writeJSON(jsonBytes)
		}
	}
	// bufio.Scanner reports a read failure and an over-long line through Err(),
	// never through Scan(). Without this the caller sees a clean, complete decode.
	if err := scanner.Err(); err != nil {
		var eb textbuf.Buffer
		writeResponse(eb.Str("decoded error ").Err(err).Byte('\n').String())
		return 1
	}
	return 0
}

// decodeSoftwareVersion decodes software-version capability wire bytes and
// reports whether the value was well formed. A false ok is the draft's
// "encoding error": the caller ignores the capability and shows no version.
//
// draft-abraitis-bgp-version-capability Section 3: "The Capability Length for
// the Software Version Capability MUST be greater than zero.  A value of zero
// SHALL be treated as an encoding error and the Capability MUST be ignored."
// data holds the Capability Value, so a Capability Length of zero arrives here
// as an empty slice. A value whose own length octet is zero is refused on the
// same ground: the capability then carries no version at all, which is the
// state Section 3 requires a receiver to ignore rather than to display.
//
// draft-abraitis-bgp-version-capability Section 3: "The Version field MUST be
// encoded using UTF-8.  A receiving BGP speaker MUST NOT interpret invalid
// UTF-8 sequences." Go's string conversion never fails, so an unchecked
// conversion would hand invalid bytes to the renderer and interpret them. The
// validity test is what stops that, and it runs before any caller sees a value.
func decodeSoftwareVersion(data []byte) (string, bool) {
	if len(data) < 1 {
		return "", false
	}
	vLen := int(data[0])
	if vLen == 0 || len(data) < 1+vLen {
		return "", false
	}
	value := data[1 : 1+vLen]
	if !utf8.Valid(value) {
		return "", false
	}
	return string(value), true
}

// RunCLIDecode decodes hex capability data directly from CLI arguments.
func RunCLIDecode(hexData string, textOutput bool, stdout, stderr io.Writer) int {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: invalid hex: %v\n", err) //nolint:errcheck // output
		return 1
	}

	version, ok := decodeSoftwareVersion(data)
	if !ok {
		// The capability is ignored, so no version is shown. Section 3 gives
		// the receiver one answer for a zero Capability Length and for invalid
		// UTF-8 alike, and printing an empty value would interpret both.
		var eb textbuf.Buffer
		_, _ = io.WriteString(stderr, eb.Str("error: software-version capability ignored: encoding error").Byte('\n').String())
		return 1
	}

	if textOutput {
		_, _ = fmt.Fprintf(stdout, "%-20s %s\n", "software-version", version) //nolint:errcheck // output
	} else {
		result := map[string]any{
			"code":  75,
			"name":  "software-version",
			"value": version,
		}
		jsonBytes, _ := json.Marshal(result)
		_, _ = fmt.Fprintln(stdout, string(jsonBytes)) //nolint:errcheck // output
	}
	return 0
}
