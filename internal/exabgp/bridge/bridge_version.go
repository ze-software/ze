// Design: docs/architecture/exabgp-bridge.md -- the API version a script speaks
// Related: bridge_encoder.go -- the encoder leaf the format also depends on
// Related: bridge_command.go -- Translator.Line, which reads `api version`

package bridge

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// APIVersion is the ExaBGP API a helper script is written for.
//
// ExaBGP 6.0 serves two APIs. A helper written for 5.x writes `announce route
// ...`, one written for 6.0 writes `peer * announce route ...`, and each is
// answered and fed in its own format (src/exabgp/reactor/api/dispatch/version.py
// at ExaBGP 685f772e4). The zero value names no version: a script whose version
// nobody set would otherwise read as whichever version zero happened to name.
type APIVersion uint8

const (
	// APIVersionUnspecified is the zero value. It names no version.
	APIVersionUnspecified APIVersion = iota
	// APIVersionAuto is ExaBGP's `auto` (0): undecided until the helper's
	// first deciding command, and as a setting, detect each helper's version.
	APIVersionAuto
	// APIV4 is the API of ExaBGP 4.x and 5.x.
	APIV4
	// APIV6 is the API of ExaBGP 6.0.
	APIV6
)

// apiVersionEnvKey is the env var the bridge reads when its fleet starts,
// through env.Get, so ExaBGP's `exabgp_api_version` reaches it too and the
// dotted spelling wins over it, as in ExaBGP. The config component registers
// it and writes it from `environment { exabgp { api { version ...; } } }`
// (config.ApplyEnvConfig), as it writes exabgp.api.ack.
const apiVersionEnvKey = "exabgp.api.version"

// ErrUnknownCommand is ExaBGP's UnknownCommand: a line the helper's dispatcher
// does not know. ExaBGP answers it `error` (API.process, `except
// UnknownCommand: ... answer_error_sync(service)`), where a line the bridge
// does not translate is refused without an answer.
var ErrUnknownCommand = errors.New("unknown command")

// v6Roots are ExaBGP's V6_ROOTS: "the first words of the v6 commands; no v4
// command starts with one of them".
var v6Roots = map[string]struct{}{"daemon": {}, "session": {}, "system": {}, "rib": {}, "peer": {}}

// v4SimpleWords are the keys of ExaBGP's V4_SIMPLE_TRANSLATIONS
// (src/exabgp/reactor/api/dispatch/v4.py): one v4 keyword that has a v6
// spelling, whatever follows it.
var v4SimpleWords = map[string]struct{}{
	"shutdown": {}, "reload": {}, "restart": {}, "status": {},
	"enable-ack": {}, "disable-ack": {}, "silence-ack": {}, "enable-sync": {}, "disable-sync": {},
	"reset": {}, "ping": {}, "bye": {},
	"help": {}, "version": {}, "crash": {}, "queue-status": {},
}

// String answers the version as ExaBGP writes it: `auto`, `4` or `6`
// (_api_version_name in src/exabgp/reactor/api/command/reactor.py).
func (v APIVersion) String() string {
	switch v {
	case APIVersionAuto:
		return "auto"
	case APIV4:
		return "4"
	case APIV6:
		return "6"
	case APIVersionUnspecified:
		return "unspecified"
	default:
		panic("BUG: invalid bridge API version")
	}
}

// ParseAPIVersion reads a version as ExaBGP's environment parser does, with its
// two refusals word for word:
//
//	"`auto` (0), each helper's version detected from its commands, or 4 or 6
//	for all of them." (api_version, src/exabgp/environment/parsing.py)
func ParseAPIVersion(word string) (APIVersion, error) {
	if strings.EqualFold(strings.TrimSpace(word), "auto") {
		return APIVersionAuto, nil
	}
	number, err := strconv.Atoi(strings.TrimSpace(word))
	if err != nil {
		return APIVersionUnspecified, errors.New("invalid API version: " + word)
	}
	switch number {
	case 0:
		return APIVersionAuto, nil
	case 4:
		return APIV4, nil
	case 6:
		return APIV6, nil
	}
	return APIVersionUnspecified, errors.New("API version must be auto, 4 or 6, got " + strconv.Itoa(number))
}

// APIVersionFromEnv answers the version every script starts with: the
// `exabgp.api.version` setting, auto when unset. A value ExaBGP would refuse is
// refused here, so the fleet does not start on a setting it cannot honour.
func APIVersionFromEnv() (APIVersion, error) {
	raw := env.Get(apiVersionEnvKey)
	if strings.TrimSpace(raw) == "" {
		return APIVersionAuto, nil
	}
	version, err := ParseAPIVersion(raw)
	if err != nil {
		return APIVersionUnspecified, errors.New(apiVersionEnvKey + ": " + err.Error() +
			"; set environment { exabgp { api { version auto|4|6; } } }")
	}
	return version, nil
}

// CommandAPIVersion is ExaBGP's command_api_version: the API version one
// command is written for, or APIVersionAuto when it does not tell.
//
//	"A v4 command is one the v4 dispatcher knows: `neighbor ...`, or one with a
//	v6 translation. A command neither dispatcher knows decides nothing, so a
//	typing mistake in a helper's first command does not hold the helper to the
//	wrong version." (src/exabgp/reactor/api/dispatch/version.py)
//
// The v4 half is translate_v4_to_v6 returning a translation, branch by branch.
// `group` is in neither vocabulary, so it decides nothing.
func CommandAPIVersion(line string) APIVersion {
	words := strings.Fields(line)
	if len(words) == 0 {
		return APIVersionAuto
	}
	if strings.HasPrefix(words[0], "#") {
		return APIVersionAuto
	}
	if _, ok := v6Roots[words[0]]; ok {
		return APIV6
	}
	if words[0] == "neighbor" {
		return APIV4
	}
	if hasV6Translation(words) {
		return APIV4
	}
	return APIVersionAuto
}

// hasV6Translation answers whether translate_v4_to_v6 returns a translation for
// the words, one branch of it per case.
func hasV6Translation(words []string) bool {
	if _, ok := v4SimpleWords[words[0]]; ok {
		return true
	}
	switch words[0] {
	case "teardown", "announce", "withdraw":
		return true
	case "api":
		return len(words) >= 2 && words[1] == "version"
	case "show":
		if len(words) >= 3 && words[1] == "adj-rib" {
			return words[2] == "in" || words[2] == "out"
		}
		return len(words) >= 2 && words[1] == "neighbor"
	case "flush":
		return len(words) >= 3 && words[1] == "adj-rib" && words[2] == "out"
	case "clear":
		return len(words) >= 3 && words[1] == "adj-rib" && (words[2] == "in" || words[2] == "out")
	case "create", "delete":
		return len(words) >= 2 && words[1] == "neighbor"
	}
	return false
}

// Admits refuses a command the helper's version does not hold, as ExaBGP's
// dispatch_for does:
//
//	"A helper is held to its version: a v4 helper writing a v6 command gets
//	the same UnknownCommand as a v6 helper writing a v4 one. Before the version
//	is known, the v4 dispatcher is used, as it accepts both forms."
//
// A v6 helper's v4 command is refused because the v6 dispatcher has no v4 root.
// `group` decides nothing, so every version admits it: "`group` is the v6
// dispatcher's whatever the version".
func (v APIVersion) Admits(line string) error {
	found := CommandAPIVersion(line)
	if found == APIVersionAuto {
		return nil
	}
	if v != APIV4 && v != APIV6 {
		return nil
	}
	if found == v {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("the helper is held to API ").Str(v.String()).Str(" and this is an API ").Str(found.String()).
		Str(" command; ExaBGP holds a helper to the API of its first deciding command until it restarts: ").
		Str(strconv.Quote(strings.TrimSpace(line)))
	return errors.Join(ErrUnknownCommand, errors.New(tb.String()))
}

// Format is the format one script's events are written in: what its API
// version and its `encoder` select together.
type Format uint8

const (
	// FormatUnspecified is the zero value. It names no format.
	FormatUnspecified Format = iota
	// FormatV4Text is ExaBGP 5.x's text, Response.V4.Text.
	FormatV4Text
	// FormatV4JSON is ExaBGP 5.x's JSON, envelope 4.0.1, Response.V4.JSON.
	FormatV4JSON
	// FormatV6JSON is ExaBGP 6.0's JSON, envelope 6.0.0, Response.JSON.
	FormatV6JSON
)

// FormatCount is how many formats a fan-out can render one event in.
const FormatCount = 3

// JSONVersionV4 is the envelope of a v4 JSON line, ExaBGP's json_v4: "Legacy
// API v4 JSON version string (frozen)" (src/exabgp/version.py).
const JSONVersionV4 = "4.0.1"

// FormatFor answers the format a helper reads, as ExaBGP's
// Processes._select_encoder, _adopt_v4 and _adopt_v6 choose it:
//
//	"An undecided helper is written to as its `encoder` asks, v4 text or v6
//	JSON, until its version is known." and "API v6 is JSON only, encoder text
//	is ignored".
func FormatFor(version APIVersion, encoder Encoder) Format {
	switch version {
	case APIV6:
		return FormatV6JSON
	case APIV4:
		if encoder == EncoderJSON {
			return FormatV4JSON
		}
		return FormatV4Text
	case APIVersionAuto, APIVersionUnspecified:
		if encoder == EncoderJSON {
			return FormatV6JSON
		}
		return FormatV4Text
	default:
		panic("BUG: invalid bridge API version")
	}
}

// Slot indexes a per-format render cache of FormatCount entries.
func (f Format) Slot() int {
	switch f {
	case FormatV4Text:
		return 0
	case FormatV4JSON:
		return 1
	case FormatV6JSON:
		return 2
	case FormatUnspecified:
		panic("BUG: bridge format unspecified")
	default:
		panic("BUG: invalid bridge format")
	}
}

// String names the format for a log line.
func (f Format) String() string {
	switch f {
	case FormatV4Text:
		return "v4 text"
	case FormatV4JSON:
		return "v4 json"
	case FormatV6JSON:
		return "v6 json"
	case FormatUnspecified:
		return "unspecified"
	default:
		panic("BUG: invalid bridge format")
	}
}

// APIVersionRequest is one `api version [<value>]` line: ExaBGP's
// api_version_cmd, answered by the bridge because the setting and each
// helper's version live in the bridge.
type APIVersionRequest struct {
	// Words are what followed `version`, unchanged: ExaBGP parses the first
	// of them, so a trailing `json` on a bare `api version` is the value.
	Words []string
	// JSON says the line ended in `json`, which asks a v4 or undecided helper
	// for a JSON answer (API._answers_in_json).
	JSON bool
}

// apiVersionRequest reads `api version ...` and the v6 `system api version
// ...`, and answers false for any other line.
func apiVersionRequest(line string) (APIVersionRequest, bool) {
	words := strings.Fields(line)
	if len(words) >= 1 && words[0] == "system" {
		words = words[1:]
	}
	if len(words) < 2 || words[0] != "api" || words[1] != "version" {
		return APIVersionRequest{}, false
	}
	return APIVersionRequest{Words: words[2:], JSON: words[len(words)-1] == "json"}, true
}

// Answer performs the request and answers the line it writes and whether that
// is a failure, which is then followed by `error` rather than `done`. A set
// moves *setting, which ExaBGP applies to "helpers started afterwards".
//
// The answers are api_version_cmd's and _show_api_version's, byte for byte; a
// JSON answer is json.dumps of the same dict, so it carries the spacing and
// key order Python writes. The caller MUST hold whatever guards *setting.
func (r APIVersionRequest) Answer(helper APIVersion, setting *APIVersion) ([]byte, bool) {
	asJSON := helper == APIV6 || r.JSON
	var tb textbuf.Buffer
	if len(r.Words) == 0 {
		if asJSON {
			tb.Str(`{"api_version": `).Str(jsonString(setting.String())).
				Str(`, "helper_api_version": `).Str(jsonString(helper.String())).Str("}\n")
			return tb.Bytes(), false
		}
		tb.Str("API version: ").Str(setting.String()).Str(" (this helper: ").Str(helper.String()).Str(")\n")
		return tb.Bytes(), false
	}
	version, err := ParseAPIVersion(r.Words[0])
	if err != nil {
		if asJSON {
			tb.Str(`{"error": `).Str(jsonString(err.Error())).Str("}\n")
			return tb.Bytes(), true
		}
		tb.Str("error: ").Str(sanitizeErrorMessage(err.Error())).Byte('\n')
		return tb.Bytes(), true
	}
	*setting = version
	if asJSON {
		tb.Str(`{"status": "API version set", "version": `).Str(jsonString(version.String())).
			Str(`, "note": "effective for helpers started afterwards"}`).Byte('\n')
		return tb.Bytes(), false
	}
	tb.Str("API version set to ").Str(version.String()).Str(" (effective for helpers started afterwards)\n")
	return tb.Bytes(), false
}

// jsonString quotes s as a JSON string. The value can carry a word the helper
// wrote, so it is marshalled rather than concatenated.
func jsonString(s string) string {
	quoted, err := json.Marshal(sanitizeErrorMessage(s))
	if err != nil {
		panic("BUG: a Go string always marshals: " + err.Error())
	}
	return string(quoted)
}
