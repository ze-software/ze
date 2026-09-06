// Design: docs/architecture/exabgp-bridge.md -- the encoder a script's events are written in
// Overview: bridge_event.go -- the JSON encoder
// Related: bridge_event_text.go -- the text encoder

package bridge

import (
	"errors"
	"fmt"
)

// Encoder names the format one script's events are written in. ExaBGP declares
// it per process, `process <name> { run ...; encoder text|json; }`, so two
// scripts under one bridge can ask for different formats
// (src/exabgp/configuration/process/__init__.py, the `encoder` Leaf).
//
// The zero value is EncoderUnspecified and is never a format: a script whose
// encoder nobody set would otherwise be written in whichever format the
// constant zero happened to name, which is the silently-wrong answer
// `ai/rules/principles.md` bans. Every construction site states the format.
type Encoder uint8

const (
	// EncoderUnspecified is the zero value. It names no format.
	EncoderUnspecified Encoder = iota
	// EncoderJSON writes each event as one ExaBGP JSON object.
	EncoderJSON
	// EncoderText writes each event as ExaBGP's one-event-per-line text.
	EncoderText
)

// Encoder spellings, as ExaBGP's config and ze's YANG both write them.
const (
	encoderNameJSON = "json"
	encoderNameText = "text"
)

// ErrEncoderUnknown is answered for a word that names neither format.
var ErrEncoderUnknown = errors.New("the ExaBGP bridge knows no such encoder")

// String answers the config spelling of the encoder.
func (e Encoder) String() string {
	switch e {
	case EncoderJSON:
		return encoderNameJSON
	case EncoderText:
		return encoderNameText
	default:
		return "unspecified"
	}
}

// ParseEncoder reads the `encoder` leaf. It REFUSES a word it does not know
// rather than answering a format, because a config asking for a third encoder
// wants neither of these two.
func ParseEncoder(value string) (Encoder, error) {
	switch value {
	case encoderNameJSON:
		return EncoderJSON, nil
	case encoderNameText:
		return EncoderText, nil
	default:
		return EncoderUnspecified, fmt.Errorf("%w: %q (text|json)", ErrEncoderUnknown, value)
	}
}
