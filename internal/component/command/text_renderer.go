// Design: docs/architecture/api/commands.md — CLI pipe operators
// Overview: pipe.go — pipe operator framework (text is one format)
//
// text_renderer.go holds the text renderings an answer's owner registers. A
// command answers data; most data reads well as the generic text table, and
// some (an AS topology) reads only as a drawing. The owner of such an answer
// registers how to draw it under the answer's envelope key, so the payload
// stays structured for `| json`, `| yaml` and every program, and the text
// rendering is the drawing.
package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TextRenderer draws the text rendering of one self-describing answer: either a
// single-key JSON object (`{"graph":{...}}`) where it receives the value under
// that key, or a multi-key envelope (`{"config":{...},"implied":{...}}`) where
// it receives the whole JSON object with any pipe metadata removed. It answers
// the text, or an error saying why it cannot draw it. It MUST NOT answer an
// empty drawing for a value it did not understand.
type TextRenderer func(answer json.RawMessage) (string, error)

// textRenderers holds the renderers by single envelope key or exact sorted key
// set. Registration happens from init(); lookups happen on every text
// rendering. Safe for concurrent use.
var textRenderers = struct {
	sync.RWMutex
	byKey    map[string]TextRenderer
	byKeySet map[string]TextRenderer
}{
	byKey:    map[string]TextRenderer{},
	byKeySet: map[string]TextRenderer{},
}

var (
	errTextRendererNoKey      = errors.New("text renderer registered with no envelope key")
	errTextRendererNoFunction = errors.New("text renderer registered with no function")
)

// RegisterTextRenderer registers render as the text rendering of every answer
// whose only envelope key is key. A key has one owner, so a second
// registration is refused.
func RegisterTextRenderer(key string, render TextRenderer) error {
	if key == "" {
		return errTextRendererNoKey
	}
	if render == nil {
		return errTextRendererNoFunction
	}
	textRenderers.Lock()
	defer textRenderers.Unlock()
	if _, taken := textRenderers.byKey[key]; taken {
		return fmt.Errorf("text renderer for envelope key %q is already registered", key)
	}
	textRenderers.byKey[key] = render
	return nil
}

// MustRegisterTextRenderer is RegisterTextRenderer for init(): a refused
// registration is a programming error, so the process stops.
func MustRegisterTextRenderer(key string, render TextRenderer) {
	if err := RegisterTextRenderer(key, render); err != nil {
		panic("BUG: command.MustRegisterTextRenderer: " + err.Error())
	}
}

// RegisterEnvelopeTextRenderer registers render as the text rendering of every
// answer whose non-metadata top-level JSON keys are the exact set keys. A key
// set has one owner, so a second registration is refused.
func RegisterEnvelopeTextRenderer(keys []string, render TextRenderer) error {
	if len(keys) == 0 {
		return errTextRendererNoKey
	}
	if render == nil {
		return errTextRendererNoFunction
	}
	if len(keys) == 1 {
		return RegisterTextRenderer(keys[0], render)
	}
	sig, label, err := envelopeKeySignature(keys)
	if err != nil {
		return err
	}
	textRenderers.Lock()
	defer textRenderers.Unlock()
	if _, taken := textRenderers.byKeySet[sig]; taken {
		return fmt.Errorf("text renderer for envelope keys %q is already registered", label)
	}
	textRenderers.byKeySet[sig] = render
	return nil
}

// MustRegisterEnvelopeTextRenderer is RegisterEnvelopeTextRenderer for init():
// a refused registration is a programming error, so the process stops.
func MustRegisterEnvelopeTextRenderer(keys []string, render TextRenderer) {
	if err := RegisterEnvelopeTextRenderer(keys, render); err != nil {
		panic("BUG: command.MustRegisterEnvelopeTextRenderer: " + err.Error())
	}
}

func envelopeKeySignature(keys []string) (sig, label string, err error) {
	sorted := slices.Clone(keys)
	slices.Sort(sorted)
	for i, k := range sorted {
		if k == "" {
			return "", "", errTextRendererNoKey
		}
		if i > 0 && sorted[i-1] == k {
			return "", "", fmt.Errorf("duplicate envelope key %q", k)
		}
	}
	return textbuf.Join(sorted, "\x00"), textbuf.Join(sorted, ","), nil
}

// applyRegisteredText draws input with the renderer registered for its
// envelope key or exact top-level key set. found is false when input is not an
// object matching any registered key or key set; the caller then renders the
// generic text table. The pipe metadata key a chain may have injected beside
// the answer is not part of the answer and is ignored.
func applyRegisteredText(input string) (text string, found bool, errMsg string) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &envelope); err != nil {
		return "", false, ""
	}
	delete(envelope, pipeMetaKey)
	if len(envelope) == 0 {
		return "", false, ""
	}
	if len(envelope) == 1 {
		for key, answer := range envelope {
			textRenderers.RLock()
			render, registered := textRenderers.byKey[key]
			textRenderers.RUnlock()
			if !registered {
				return "", false, ""
			}
			drawn, err := render(answer)
			if err != nil {
				var tb textbuf.Buffer
				return "", true, tb.Str(key).Str(": ").Str(err.Error()).String()
			}
			return drawn, true, ""
		}
		panic("BUG: applyRegisteredText: a one-key map yielded no key")
	}

	keys := make([]string, 0, len(envelope))
	for k := range envelope {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	sig := textbuf.Join(keys, "\x00")

	textRenderers.RLock()
	render, registered := textRenderers.byKeySet[sig]
	textRenderers.RUnlock()
	if !registered {
		return "", false, ""
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		var tb textbuf.Buffer
		return "", true, tb.Str(textbuf.Join(keys, ",")).Str(": ").Str(err.Error()).String()
	}
	drawn, err := render(raw)
	if err != nil {
		var tb textbuf.Buffer
		return "", true, tb.Str(textbuf.Join(keys, ",")).Str(": ").Str(err.Error()).String()
	}
	return drawn, true, ""
}
