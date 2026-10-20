// Design: docs/architecture/exabgp-bridge.md -- the API version a script speaks
// Related: bridge_version.go -- CommandAPIVersion, Admits and FormatFor, which this holds a script to
// Related: bridge.go -- Bridge, the standalone runner, which holds one for its script

package bridge

import (
	"log/slog"
	"sync/atomic"
)

// ScriptVersion is the API version one running ExaBGP script is held to, and
// the format its events are written in. Every runner holds one per script: the
// plugin runner (bridgerun) one for each process block, the standalone runner
// (Bridge) one for the script it was started with. Holding the state here,
// rather than in one runner, is what keeps the runners from answering the same
// script two ways.
//
// Safe for concurrent use: an event goroutine reads Format while the batch
// goroutine moves the version. Only the batch goroutine moves it, through
// Detect, and the runner calls Reset at each fork.
type ScriptVersion struct {
	name    string
	log     *slog.Logger
	encoder Encoder

	// current is the APIVersion the script is held to, auto while undecided.
	current atomic.Uint32

	// setting is `exabgp.api.version`, which each fork starts with and
	// `api version <v>` moves. It is shared by every script of one runner, as
	// ExaBGP's environment setting is shared by every helper.
	setting *atomic.Uint32
}

// NewScriptVersion answers the version state of the script called name, whose
// `encoder` leaf is encoder, starting from setting. The runner MUST call Reset
// at each fork, after it has stored the setting it read.
func NewScriptVersion(name string, log *slog.Logger, encoder Encoder, setting *atomic.Uint32) *ScriptVersion {
	v := &ScriptVersion{name: name, log: log, encoder: encoder, setting: setting}
	v.current.Store(setting.Load())
	return v
}

// Encoder answers the `encoder` leaf the script was declared with.
func (v *ScriptVersion) Encoder() Encoder {
	return v.encoder
}

// Current answers the API version the script is held to, or APIVersionAuto
// while it is undecided.
func (v *ScriptVersion) Current() APIVersion {
	return APIVersion(v.current.Load())
}

// Format answers the format the script's events are written in now.
func (v *ScriptVersion) Format() Format {
	return FormatFor(v.Current(), v.encoder)
}

// Reset gives a freshly forked child the setting, which ExaBGP's
// Processes._select_encoder does at each start: "With exabgp.api.version set,
// every operator helper is held to that version; left to auto, a helper is
// undecided until its first command says which API it was written for". A
// respawned child starts undecided again, because the version is held "for as
// long as it runs".
func (v *ScriptVersion) Reset() {
	setting := APIVersion(v.setting.Load())
	v.current.Store(uint32(setting))
	if setting != APIVersionAuto {
		v.adopt(setting)
	}
}

// Detect is ExaBGP's Processes.detect_api_version: "The first command whose
// form belongs to one version holds the helper to it for as long as it runs.
// Undecided commands leave it undecided." It answers the version the script is
// held to after the line. Only the batch goroutine calls it, so the load and
// the store need no compare-and-swap.
func (v *ScriptVersion) Detect(line string) APIVersion {
	current := v.Current()
	if current != APIVersionAuto {
		return current
	}
	found := CommandAPIVersion(line)
	if found == APIVersionAuto {
		return current
	}
	v.current.Store(uint32(found))
	v.log.Debug("exabgp-bridge script API version detected", "script", v.name, "version", found.String())
	v.adopt(found)
	return found
}

// adopt logs what ExaBGP's _adopt_v4 and _adopt_v6 log when a helper is held
// to a version: the v4 API is deprecated, and a v6 helper whose encoder asks
// for text reads JSON regardless.
func (v *ScriptVersion) adopt(version APIVersion) {
	if version == APIV4 {
		v.log.Warn("exabgp-bridge script uses the deprecated ExaBGP API 4; ExaBGP 6.0 helpers write peer <selector> ...",
			"script", v.name)
		return
	}
	if version == APIV6 && v.encoder == EncoderText {
		v.log.Warn("exabgp-bridge script uses ExaBGP API 6, which is JSON only; its encoder text is ignored",
			"script", v.name)
	}
}

// Answer performs one `api version` line against the script's version and the
// setting, and answers the reply line and whether it is a failure. A set
// reaches scripts started afterwards, a respawn included, as ExaBGP's does: "a
// change applies to helpers started afterwards".
func (v *ScriptVersion) Answer(request APIVersionRequest) ([]byte, bool) {
	setting := APIVersion(v.setting.Load())
	reply, failed := request.Answer(v.Current(), &setting)
	v.setting.Store(uint32(setting))
	return reply, failed
}

// Line answers the event as the line a script reading format is written,
// newline included, or nil when that format writes nothing for it: ExaBGP's
// text encoder writes no line for `negotiated`, `fsm` or `signal`. An event
// with no text form, or one that does not marshal, is logged on log and
// answers nil, so the script is not told rather than told something wrong.
func (e Event) Line(format Format, log *slog.Logger) []byte {
	switch format {
	case FormatV4Text:
		var scratch [512]byte
		out, form := e.AppendText(scratch[:0])
		switch form {
		case TextWritten:
			return out
		case TextUnknown:
			log.Warn("exabgp-bridge: no text form for this event; the script is not told about it",
				"event", e.Kind)
			return nil
		case TextNone:
			// ExaBGP writes no text line for this kind either.
			return nil
		default:
			panic("BUG: invalid bridge text form")
		}
	case FormatV4JSON, FormatV6JSON:
		encoded, err := MarshalExabgpJSON(e.ExabgpJSON(format))
		if err != nil {
			log.Warn("exabgp-bridge: marshal external JSON failed", "error", err)
			return nil
		}
		return append(encoded, '\n')
	case FormatUnspecified:
		panic("BUG: bridge format unspecified")
	default:
		panic("BUG: invalid bridge format")
	}
}
