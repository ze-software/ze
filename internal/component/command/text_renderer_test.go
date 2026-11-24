package command

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// testRenderedKey is an envelope key only these tests register.
const testRenderedKey = "test-rendered-answer"

func init() {
	MustRegisterTextRenderer(testRenderedKey, func(answer json.RawMessage) (string, error) {
		var value struct {
			Word string `json:"word"`
		}
		if err := json.Unmarshal(answer, &value); err != nil {
			return "", err
		}
		if value.Word == "" {
			return "", errors.New("no word to draw")
		}
		return "drawn " + value.Word + "\n", nil
	})
}

// TestTextRendersRegisteredAnswer: the text rendering of an answer whose one
// envelope key has a registered renderer is that renderer's drawing, both as
// the default format and as an explicit `| text`, while `| json` keeps the
// structure.
//
// VALIDATES: a self-describing answer is drawn by the renderer its owner
// registered, never by the generic table.
// PREVENTS: a handler pre-rendering text because the CLI cannot draw its data.
func TestTextRendersRegisteredAnswer(t *testing.T) {
	answer := `{"` + testRenderedKey + `":{"word":"box"}}`
	for _, input := range []string{"show thing", "show thing | text"} {
		_, format, msg := ProcessPipesDefaultFormatChecked(input, "text")
		if msg != "" {
			t.Fatalf("%s: %s", input, msg)
		}
		if got := format(answer); got != "drawn box\n" {
			t.Errorf("%s: got %q, want the registered drawing", input, got)
		}
	}

	_, format, msg := ProcessPipesDefaultFormatChecked("show thing | json", "")
	if msg != "" {
		t.Fatal(msg)
	}
	if got := format(answer); !strings.Contains(got, `"word": "box"`) {
		t.Errorf("| json lost the structure: %q", got)
	}
}

// TestTextRendererRefusalIsAPipeError: a renderer that cannot draw the answer
// it was given makes the rendering a pipe error naming why.
//
// VALIDATES: the renderer's refusal reaches the reader.
// PREVENTS: an undrawable answer shown as an empty screen.
func TestTextRendererRefusalIsAPipeError(t *testing.T) {
	_, format, msg := ProcessPipesDefaultFormatChecked("show thing", "text")
	if msg != "" {
		t.Fatal(msg)
	}
	got := format(`{"` + testRenderedKey + `":{"word":""}}`)
	if !IsPipeError(got) || !strings.Contains(got, "no word to draw") {
		t.Errorf("got %q, want a pipe error naming the refusal", got)
	}
}

// TestTextRendererIgnoresUnregisteredAndCompositeAnswers: an answer with no
// registered key, or with a registered key beside another, renders as the
// generic text table.
//
// VALIDATES: only a single-key answer the owner registered is drawn.
// PREVENTS: a renderer swallowing fields it does not know.
func TestTextRendererIgnoresUnregisteredAndCompositeAnswers(t *testing.T) {
	_, format, msg := ProcessPipesDefaultFormatChecked("show thing", "text")
	if msg != "" {
		t.Fatal(msg)
	}
	for _, answer := range []string{
		`{"other":{"word":"box"}}`,
		`{"` + testRenderedKey + `":{"word":"box"},"other":1}`,
	} {
		if got := format(answer); strings.Contains(got, "drawn") {
			t.Errorf("%s: drawn by the renderer: %q", answer, got)
		}
	}
}

// TestMustRegisterTextRendererRefusesDuplicate: a second renderer for one key
// is a programming error, refused at registration.
//
// VALIDATES: one key, one owner.
// PREVENTS: two plugins silently fighting over one answer's drawing.
func TestMustRegisterTextRendererRefusesDuplicate(t *testing.T) {
	if err := RegisterTextRenderer(testRenderedKey, func(json.RawMessage) (string, error) { return "", nil }); err == nil {
		t.Fatal("duplicate registration accepted")
	}
	if err := RegisterTextRenderer("", func(json.RawMessage) (string, error) { return "", nil }); err == nil {
		t.Fatal("empty key accepted")
	}
	if err := RegisterTextRenderer("nil-renderer", nil); err == nil {
		t.Fatal("nil renderer accepted")
	}
}

const (
	testEnvelopeKeyA = "test-envelope-a"
	testEnvelopeKeyB = "test-envelope-b"
)

// TestEnvelopeTextRendererAndLineMatch proves multi-key envelope text
// renderers draw under default text, `| text`, `| text | match`, and bare
// `| match` on line-rendered surfaces, while keeping `| json` structured.
//
// VALIDATES: RegisterEnvelopeTextRenderer, applyRegisteredText multi-key
// matching, and validateDeclaredShape allowing line transforms after `| text`.
// PREVENTS: a two-key envelope such as `{config, implied}` falling back to a
// generic ASCII table on `| text` or failing `| text | match` on ShapeBlob/Doc.
func TestEnvelopeTextRendererAndLineMatch(t *testing.T) {
	if err := RegisterEnvelopeTextRenderer([]string{testEnvelopeKeyB, testEnvelopeKeyA}, func(answer json.RawMessage) (string, error) {
		var env struct {
			A string `json:"test-envelope-a"`
			B string `json:"test-envelope-b"`
		}
		if err := json.Unmarshal(answer, &env); err != nil {
			return "", err
		}
		return "line-a " + env.A + "\n# implied: " + env.B + "\n", nil
	}); err != nil {
		t.Fatalf("RegisterEnvelopeTextRenderer: %v", err)
	}

	if err := RegisterEnvelopeTextRenderer([]string{testEnvelopeKeyA, testEnvelopeKeyB}, func(json.RawMessage) (string, error) {
		return "", nil
	}); err == nil {
		t.Fatal("duplicate envelope key-set registration accepted")
	}

	raw := `{"` + testEnvelopeKeyA + `":"alpha","` + testEnvelopeKeyB + `":"beta"}`
	for _, input := range []string{"show env-pair", "show env-pair | text"} {
		_, format, msg := ProcessPipesDefaultFormatChecked(input, "text")
		if msg != "" {
			t.Fatalf("%s: %s", input, msg)
		}
		want := "line-a alpha\n# implied: beta\n"
		if got := format(raw); got != want {
			t.Errorf("%s: got %q, want %q", input, got, want)
		}
	}

	_, formatMatch, msg := ProcessPipesDefaultFormatChecked("show env-pair | match implied", "text")
	if msg != "" {
		t.Fatalf("show env-pair | match implied: %s", msg)
	}
	if got := formatMatch(raw); got != "# implied: beta\n" {
		t.Errorf("match on envelope text = %q, want %q", got, "# implied: beta\n")
	}

	_, formatJSON, msg := ProcessPipesDefaultFormatChecked("show env-pair | json", "")
	if msg != "" {
		t.Fatalf("show env-pair | json: %s", msg)
	}
	if got := formatJSON(raw); !strings.Contains(got, testEnvelopeKeyA) || !strings.Contains(got, testEnvelopeKeyB) {
		t.Errorf("| json lost envelope keys: %q", got)
	}

	const blobPath = "show test env blob"
	RegisterShape([]string{blobPath}, ShapeBlob)
	_, formatBlobMatch, msg := ProcessPipesDefaultFormatChecked(blobPath+" | text | match implied", "")
	if msg != "" {
		t.Fatalf("%s | text | match implied refused: %s", blobPath, msg)
	}
	if got := formatBlobMatch(raw); got != "# implied: beta\n" {
		t.Errorf("%s | text | match implied = %q, want %q", blobPath, got, "# implied: beta\n")
	}
}
