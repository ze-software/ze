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

// valueEncoding is the form of the Capability Value Ze sends. The zero value is
// no form, so a peer whose encoding was never resolved cannot be sent one by
// accident.
type valueEncoding uint8

const (
	valueEncodingUnspecified valueEncoding = iota
	// valueEncodingDraft is the form draft-abraitis-bgp-version-capability
	// Section 3 defines: the Capability Value is the version string alone.
	valueEncodingDraft
	// valueEncodingLegacy is the form FRR and ExaBGP send and expect: one
	// length octet, then that many octets of the version string. It is an
	// owner-approved deviation from the draft (D-13), kept because FRR 10.3.1's
	// bgp_capability_software_version reads the first value octet as a length
	// and sends a NOTIFICATION when it exceeds what follows.
	valueEncodingLegacy
)

// Values of the software-version encoding leaf (ze-softver.yang).
const (
	encodingNameDraft  = "draft"
	encodingNameLegacy = "legacy"
)

// capabilityValueOctetsMax is the largest Capability Value: the Capability
// Length that carries its size is one octet.
const capabilityValueOctetsMax = 255

// parseValueEncoding converts the encoding leaf to its typed form. An absent
// leaf ("") is the YANG default, draft (ze-softver.yang, `default "draft"`).
// The YANG leaf declares that default and this switch repeats it, so
// TestEncodingDefaultMatchesYANG compares the two.
// It is mapped here rather than refused because the config JSON does not
// always carry YANG defaults: the parser fills none, applyPeerSchemaDefaults
// (bgp/config/peers.go) fills only the containers under bgp/peer, and a
// group's container, the bare flex form (`software-version enable`) and bare
// presence all reach parseSoftverSetting with no encoding. Any other string is
// refused: the YANG enumeration admits only the two names, so one arriving
// here means the config did not pass through the schema.
func parseValueEncoding(name string) (valueEncoding, error) {
	switch name {
	case "", encodingNameDraft:
		return valueEncodingDraft, nil
	case encodingNameLegacy:
		return valueEncodingLegacy, nil
	}
	return valueEncodingUnspecified, fmt.Errorf("software-version encoding %q: want %s or %s", name, encodingNameDraft, encodingNameLegacy)
}

// encodeValue returns the hex-encoded Capability Value in the form the peer's
// encoding selects, without the code and Capability Length octets the OPEN
// encoder adds around it. encoding MUST be draft or legacy; the unspecified
// zero value is a Ze defect, and parseValueEncoding never returns it without an
// error.
//
// draft-abraitis-bgp-version-capability Section 3: "The Capability Value field
// is the software version encoded as a UTF-8 [RFC3629] string.  It is
// unstructured data and can be formatted in any way that the implementor
// decides.  The string is not null-terminated." In the draft form the value is
// the string itself: the Capability Length already carries its size, so the
// string is cut at 255 octets. The legacy form spends one of those octets on
// its own length, so its string is cut at 254.
func encodeValue(encoding valueEncoding) string {
	version := []byte(ZeVersion)
	switch encoding {
	case valueEncodingDraft:
		if len(version) > capabilityValueOctetsMax {
			version = version[:capabilityValueOctetsMax]
		}
		return hex.EncodeToString(version)
	case valueEncodingLegacy:
		if len(version) > capabilityValueOctetsMax-1 {
			version = version[:capabilityValueOctetsMax-1]
		}
		value := make([]byte, 1+len(version))
		value[0] = byte(len(version))
		copy(value[1:], version)
		return hex.EncodeToString(value)
	case valueEncodingUnspecified:
	}
	panic("BUG: software-version encodeValue called with an unresolved encoding")
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
			sectionCaps, err := extractSoftverCapabilities(section.Data)
			if err != nil {
				return err
			}
			caps = append(caps, sectionCaps...)
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

// softverSetting is the software-version container that governs one peer: the
// peer's own when it has one, else its group's.
type softverSetting struct {
	enabled  bool
	encoding valueEncoding
}

// parseSoftverSetting reads one software-version container. The container
// arrives as a map carrying mode and encoding, as the bare string of the older
// flex form (`software-version enable`), or as nil for bare presence, which is
// enable.
func parseSoftverSetting(raw any) (softverSetting, error) {
	var mode, encodingName string
	switch container := raw.(type) {
	case map[string]any:
		mode, _ = container["mode"].(string)
		encodingName, _ = container["encoding"].(string)
	case string:
		mode = container
	case nil:
	}
	encoding, err := parseValueEncoding(encodingName)
	if err != nil {
		return softverSetting{}, err
	}
	return softverSetting{enabled: mode != modeDisable && mode != modeRefuse, encoding: encoding}, nil
}

// extractSoftverCapabilities parses bgp config JSON and returns per-peer
// software-version capabilities. Handles both standalone peers (bgp.peer) and
// grouped peers (bgp.group.<name>.peer). A container on the peer replaces the
// group's whole, so mode and encoding both come from the one that governs.
func extractSoftverCapabilities(jsonStr string) ([]sdk.CapabilityDecl, error) {
	bgpSubtree, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		Logger.Warn("invalid JSON in bgp config")
		return nil, nil
	}

	const softverCapCode = 75
	var caps []sdk.CapabilityDecl
	var firstErr error

	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		raw, exists := configjson.GetCapability(peerMap)["software-version"]
		if !exists && groupMap != nil {
			raw, exists = configjson.GetCapability(groupMap)["software-version"]
		}
		if !exists {
			return
		}

		setting, err := parseSoftverSetting(raw)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("peer %s: %w", peerAddr, err)
			}
			return
		}
		if !setting.enabled {
			Logger.Debug("software-version capability suppressed by mode", "peer", peerAddr)
			return
		}

		caps = append(caps, sdk.CapabilityDecl{
			Code:     softverCapCode,
			Encoding: sdk.CapEncodingHex,
			Payload:  encodeValue(setting.encoding),
			Peers:    []string{configjson.CapabilitySelector(peerAddr, origin)},
		})
		Logger.Debug("software-version capability enabled", "peer", peerAddr)
	})

	if firstErr != nil {
		return nil, firstErr
	}
	return caps, nil
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
			// Section 3's encoding error: a zero Capability Length, or a value
			// that is not valid UTF-8. Each is reported as unknown, which is
			// how this path ignores it.
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
// Two forms reach Ze. The draft's is the version string alone. The legacy form
// FRR and ExaBGP send prefixes one length octet (valueEncodingLegacy). The value
// is read as legacy when its first octet equals the number of octets after it
// and those octets are valid UTF-8, and as the draft's bare string otherwise.
//
// The two forms overlap in one place, and this function picks legacy there. A
// bare string whose first octet equals its own length minus one reads as a
// length octet followed by the rest: a 34-octet version starting with "!"
// (0x21, 33) loses that "!". Every printable first octet is 0x20 or more, so
// the overlap needs a bare version of 33 octets or longer whose first character
// happens to encode its remaining length. Reading such a value as the draft
// form instead would misread every FRR and ExaBGP speaker, which send the
// legacy form on every session.
//
// A legacy value with octets after the declared version is read as the draft
// form, so its length octet shows as the version's first character. No known
// encoder sends that shape: FRR's (bgpd/bgp_open.c, bgp_open_capability)
// writes the length octet and the version, and a Capability Length of one more
// than the version. The draft declares no inner length, so a value whose first
// octet does not count what follows is a bare string by the draft's text.
// FRR 10.5.3's bgp_capability_software_version decides the same way: it reads
// the first octet as a length only when that octet plus one equals the
// Capability Length.
//
// draft-abraitis-bgp-version-capability Section 3: "The Capability Value field
// is the software version encoded as a UTF-8 [RFC3629] string." In the draft
// form data, the Capability Value, is the whole version.
//
// draft-abraitis-bgp-version-capability Section 3: "The Capability Length for
// the Software Version Capability MUST be greater than zero.  A value of zero
// SHALL be treated as an encoding error and the Capability MUST be ignored."
// A Capability Length of zero arrives here as an empty slice. A legacy value
// whose length octet is zero declares an empty version, which leaves nothing
// to show either, so it is refused the same way.
//
// draft-abraitis-bgp-version-capability Section 3: "The Version field MUST be
// encoded using UTF-8.  A receiving BGP speaker MUST NOT interpret invalid
// UTF-8 sequences." Go's string conversion never fails, so an unchecked
// conversion would hand invalid bytes to the renderer and interpret them. The
// validity test is what stops that, and it runs before any caller sees a value.
func decodeSoftwareVersion(data []byte) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	if int(data[0]) == len(data)-1 {
		if legacy := data[1:]; utf8.Valid(legacy) {
			if len(legacy) == 0 {
				return "", false
			}
			return string(legacy), true
		}
	}
	if !utf8.Valid(data) {
		return "", false
	}
	return string(data), true
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
