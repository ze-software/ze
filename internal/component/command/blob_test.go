package command

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// TestBlobJSONNamesItsEncoding proves a Blob's JSON form says how its content
// travels, and that DecodeBlob gives the bytes back.
//
// VALIDATES: UTF-8 text travels as a JSON string under "utf-8"; bytes that are
// not valid UTF-8 travel as base64; both decode to the original bytes.
// PREVENTS: a binary file reaching a remote caller with every invalid byte
// replaced by U+FFFD, which is what json.Marshal does to a Go string.
func TestBlobJSONNamesItsEncoding(t *testing.T) {
	cases := []struct {
		name     string
		blob     Blob
		encoding string
		content  string
	}{
		{"text", Blob("line one\nline two\n"), BlobEncodingText, "line one\nline two\n"},
		{"binary", Blob{0xff, 0x00, 'a'}, BlobEncodingBase64, "/wBh"},
		{"empty", Blob{}, BlobEncodingText, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.blob)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var envelope map[string]blobWire
			if err := json.Unmarshal(encoded, &envelope); err != nil {
				t.Fatalf("unmarshal %s: %v", encoded, err)
			}
			wire, found := envelope[blobEnvelopeKey]
			if !found || len(envelope) != 1 {
				t.Fatalf("JSON %s: want one %q key", encoded, blobEnvelopeKey)
			}
			if wire.Encoding != tc.encoding || wire.Content != tc.content {
				t.Errorf("wire = %+v, want encoding %q content %q", wire, tc.encoding, tc.content)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatalf("unmarshal raw: %v", err)
			}
			decoded, err := DecodeBlob(raw[blobEnvelopeKey])
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if string(decoded) != string(tc.blob) {
				t.Errorf("decoded %q, want %q", decoded, tc.blob)
			}
		})
	}
}

// TestDecodeBlobRefusesAnUnknownEncoding proves an encoding no Blob writes is
// refused by name rather than read as text.
func TestDecodeBlobRefusesAnUnknownEncoding(t *testing.T) {
	if _, err := DecodeBlob(json.RawMessage(`{"encoding":"hex","content":"ff"}`)); err == nil ||
		!strings.Contains(err.Error(), "hex") {
		t.Errorf("unknown encoding: err = %v, want a refusal naming hex", err)
	}
	if _, err := DecodeBlob(json.RawMessage(`{"content":"x"}`)); err == nil {
		t.Error("missing encoding: want a refusal")
	}
	if _, err := DecodeBlob(json.RawMessage(`{"encoding":"base64","content":"!!"}`)); err == nil {
		t.Error("bad base64: want a refusal")
	}
}

// TestParseAnswerShapeReadsBlob proves the wire spelling round-trips, so a
// plugin can declare a blob answer.
func TestParseAnswerShapeReadsBlob(t *testing.T) {
	shape, ok := ParseAnswerShape(ShapeBlob.String())
	if !ok || shape != ShapeBlob {
		t.Errorf("ParseAnswerShape(%q) = %v, %v; want ShapeBlob, true", ShapeBlob.String(), shape, ok)
	}
}

// TestBlobShapeRefusesRowOperatorsByName proves a command declaring ShapeBlob
// refuses every data operator before it runs, with a message that names the
// operator, says why, and names the operators that do apply.
func TestBlobShapeRefusesRowOperatorsByName(t *testing.T) {
	const path = "show test blob refusal"
	RegisterShape([]string{path}, ShapeBlob)

	for _, op := range pipeCatalog {
		if op.Class != ClassData {
			continue
		}
		input := path + " | " + op.Name
		if op.Arg != ArgNone && op.Arg != ArgOptional {
			input += " 1"
		}
		command, ops := parsePipeChain(input)
		msg := validateDeclaredShape(command, ops)
		if op.NeedsAddressField {
			// resolve and origin are refused first because no field of a
			// blob holds an address, which is the more precise reason.
			if !strings.Contains(msg, op.Name+" cannot apply here") {
				t.Errorf("%s over a blob: refusal %q, want it named", op.Name, msg)
			}
			continue
		}
		if !strings.Contains(msg, op.Name+" cannot apply here") ||
			!strings.Contains(msg, "text or bytes") ||
			!strings.Contains(msg, "| text") {
			t.Errorf("%s over a blob: refusal %q, want it named, explained and fixed", op.Name, msg)
		}
	}
	for _, name := range []string{"json", "text", "raw", "yaml"} {
		command, ops := parsePipeChain(path + " | " + name)
		if msg := validateDeclaredShape(command, ops); msg != "" {
			t.Errorf("| %s over a blob refused: %q; a global operator acts on any answer", name, msg)
		}
	}
}

// TestBlobRendersUnchangedOnEveryLocalRoute proves a Blob answer reaches the
// operator as its bytes on both local routes, whatever the configured format.
//
// VALIDATES: `ze <verb>` (RenderLocalAnswer) prints the bytes with no newline
// added; `ze cli -c` (ServeLocal) answers the text under a session format of
// json and of table; `| json` answers the encoded envelope.
// PREVENTS: a stored file shown as a table of its envelope, or a binary file
// gaining a trailing newline on the route a shell redirect reads.
func TestBlobRendersUnchangedOnEveryLocalRoute(t *testing.T) {
	const path = "show test blob render"
	const text = "line one\nline two\n"
	RegisterShape([]string{path}, ShapeBlob)
	if err := RegisterLocalData(path, func(ValidatedArgs) LocalAnswer {
		return LocalAnswer{Payload: Blob(text)}
	}, registry.Meta{}, RenderLocalAnswer); err != nil {
		t.Fatalf("register: %v", err)
	}
	withArgDefSource(t, func(string) ([]ArgDef, error) { return nil, nil })

	binary := Blob{0xff, 'a'}
	var code int
	stdout, stderr := captureStdio(t, func() { code = RenderLocalAnswer(path, binary) })
	if code != 0 || stdout != string(binary) || stderr != "" {
		t.Errorf("RenderLocalAnswer: code %d stdout %q stderr %q; want 0, %q, nothing", code, stdout, stderr, binary)
	}

	for _, format := range []string{"json", "table", ""} {
		answer, code, served := ServeLocal(path, format)
		if !served || code != 0 || answer != text {
			t.Errorf("ServeLocal with session format %q = %q, %d, %v; want the text", format, answer, code, served)
		}
	}

	answer, _, _ := ServeLocal(path+" | json", "")
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answer), &envelope); err != nil {
		t.Fatalf("| json answer %q is not JSON: %v", answer, err)
	}
	decoded, err := DecodeBlob(envelope[blobEnvelopeKey])
	if err != nil || string(decoded) != text {
		t.Errorf("| json decoded = %q, %v; want the text", decoded, err)
	}
}
