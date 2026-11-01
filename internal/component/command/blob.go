// Design: docs/architecture/api/commands.md — text and bytes answers
// Related: pipe_catalog.go — ShapeBlob, the shape a command answering a Blob declares
// Related: local_data.go — the local routes that print a Blob as it is
// Related: register_blob.go — the text rendering of a Blob
//
// blob.go holds the answer of a command whose answer IS text or bytes: a stored
// file, a formatted config, a page of documentation. Those bytes are what the
// operator asked for, so a local route prints them unchanged. A remote caller
// receives them inside a JSON answer, and a binary file is not valid JSON text,
// so on that route the answer names the encoding its content travels in.

package command

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Blob is the payload of an answer that is text or bytes read as they are.
// A command answering one declares ShapeBlob, so the row operators are refused
// by name before it runs. A nil or empty Blob is an answer of no bytes, such as
// an empty stored file; a handler with nothing to answer returns no payload at
// all (LocalRefusal).
type Blob []byte

// The two encodings a Blob's content travels in. Text that is valid UTF-8 is
// carried as a JSON string, so a reader of `| json` sees the text; anything
// else is carried as standard base64, because json.Marshal would replace each
// invalid byte with U+FFFD and the caller could not get the bytes back.
const (
	BlobEncodingText   = "utf-8"
	BlobEncodingBase64 = "base64"
)

// blobEnvelopeKey is the one key of a Blob's JSON answer. The text renderer is
// registered under it, so `| text` prints the content and not a table of it.
const blobEnvelopeKey = "blob"

// blobWire is a Blob's JSON form under blobEnvelopeKey.
type blobWire struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// MarshalJSON answers {"blob":{"encoding":...,"content":...}}, the form every
// route that carries an answer as JSON (the pipe chain, the daemon response)
// receives. DecodeBlob is the reverse.
func (b Blob) MarshalJSON() ([]byte, error) {
	wire := blobWire{Encoding: BlobEncodingText, Content: string(b)}
	if !utf8.Valid(b) {
		wire = blobWire{Encoding: BlobEncodingBase64, Content: base64.StdEncoding.EncodeToString(b)}
	}
	return json.Marshal(map[string]blobWire{blobEnvelopeKey: wire})
}

var errBlobNoEncoding = errors.New("the answer names no encoding for its content")

// DecodeBlob answers the bytes of a Blob from the value under its envelope key,
// or an error naming why it cannot: an encoding no Blob writes is refused, not
// read as text, so a caller never prints base64 as if it were the file.
func DecodeBlob(value json.RawMessage) (Blob, error) {
	var wire blobWire
	if err := json.Unmarshal(value, &wire); err != nil {
		return nil, err
	}
	switch wire.Encoding {
	case BlobEncodingText:
		return Blob(wire.Content), nil
	case BlobEncodingBase64:
		decoded, err := base64.StdEncoding.DecodeString(wire.Content)
		if err != nil {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("the base64 content does not decode: ").Err(err).String())
		}
		return Blob(decoded), nil
	case "":
		return nil, errBlobNoEncoding
	default:
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("the answer names encoding ").Str(wire.Encoding).
			Str(", and a blob is written only as ").Str(BlobEncodingText).Str(" or ").
			Str(BlobEncodingBase64).String())
	}
}

// renderBlobText is the text rendering of a Blob: its bytes, unchanged.
func renderBlobText(value json.RawMessage) (string, error) {
	blob, err := DecodeBlob(value)
	if err != nil {
		return "", err
	}
	return string(blob), nil
}
