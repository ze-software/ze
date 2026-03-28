// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure

package testing

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Input kind constants.
const (
	inputKindType = "type"
	inputKindKey  = "key"
	inputKindCtrl = "ctrl"
)

// Input represents a runtime-typed input action for the cli.
// This is the type-safe version used for tea.Msg conversion.
type Input struct {
	Kind string // "type", "key", or "ctrl"
	Text string // Text to type (for Kind="type")
	Key  string // Key name (for Kind="key" or "ctrl")
}

// ToInput converts an InputAction (from parser) to an Input (for runtime use).
// This bridges the flexible map-based parser output to the type-safe runtime type.
func (ia InputAction) ToInput() Input {
	inp := Input{Kind: ia.Action}

	switch ia.Action {
	case inputKindType:
		inp.Text = ia.Values["text"]
	case inputKindKey:
		inp.Key = ia.Values["name"]
	case inputKindCtrl:
		inp.Key = ia.Values["key"]
	}

	return inp
}

// InputActionsToInputs converts a slice of InputAction to Input.
func InputActionsToInputs(actions []InputAction) []Input {
	inputs := make([]Input, len(actions))
	for i, a := range actions {
		inputs[i] = a.ToInput()
	}
	return inputs
}

// ToMessages converts an Input action to one or more tea.Msg values.
// For "type" inputs, each character becomes a separate KeyPressMsg.
// For "key" inputs, a single KeyPressMsg is returned.
// For "ctrl" inputs, a KeyPressMsg with ModCtrl is returned.
func (inp Input) ToMessages() ([]tea.Msg, error) {
	switch inp.Kind {
	case inputKindType:
		return inp.toTypeMessages(), nil
	case inputKindKey:
		return inp.toKeyMessages()
	case inputKindCtrl:
		return inp.toCtrlMessages()
	}
	return nil, fmt.Errorf("unknown input kind: %s", inp.Kind)
}

// toTypeMessages converts typed text to KeyPressMsg messages.
func (inp Input) toTypeMessages() []tea.Msg {
	msgs := make([]tea.Msg, 0, len(inp.Text))
	for _, r := range inp.Text {
		msgs = append(msgs, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return msgs
}

// keyNameToCode maps key names to tea key code runes.
var keyNameToCode = map[string]rune{
	"tab":       tea.KeyTab,
	"enter":     tea.KeyEnter,
	"esc":       tea.KeyEscape,
	"escape":    tea.KeyEscape,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
	"backspace": tea.KeyBackspace,
	"delete":    tea.KeyDelete,
	"home":      tea.KeyHome,
	"end":       tea.KeyEnd,
	"pgup":      tea.KeyPgUp,
	"pgdn":      tea.KeyPgDown,
	"pgdown":    tea.KeyPgDown,
	"space":     tea.KeySpace,
}

// toKeyMessages converts a key name to a KeyPressMsg.
func (inp Input) toKeyMessages() ([]tea.Msg, error) {
	// Special case: "space" should insert a space character,
	// not send KeySpace which doesn't insert text in textinput.
	if inp.Key == "space" {
		return []tea.Msg{tea.KeyPressMsg{Code: ' ', Text: " "}}, nil
	}

	// Special case: "shift+tab" uses modifier
	if inp.Key == "shift+tab" {
		return []tea.Msg{tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}}, nil
	}

	code, ok := keyNameToCode[inp.Key]
	if !ok {
		return nil, fmt.Errorf("unknown key name: %s", inp.Key)
	}
	return []tea.Msg{tea.KeyPressMsg{Code: code}}, nil
}

// toCtrlMessages converts a ctrl+key to a KeyPressMsg.
func (inp Input) toCtrlMessages() ([]tea.Msg, error) {
	if len(inp.Key) != 1 {
		return nil, fmt.Errorf("ctrl key must be single character, got: %s", inp.Key)
	}

	r := rune(inp.Key[0])
	if !isCtrlLetter(r) {
		return nil, fmt.Errorf("ctrl key must be a-z, got: %s", inp.Key)
	}

	// Normalize to lowercase for the Code field.
	code := r
	if code >= 'A' && code <= 'Z' {
		code = code - 'A' + 'a'
	}

	return []tea.Msg{tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}}, nil
}

// isCtrlLetter returns true if the rune is a letter (a-z or A-Z).
func isCtrlLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// InputsToMessages converts a slice of Input actions to tea.Msg values.
func InputsToMessages(inputs []Input) ([]tea.Msg, error) {
	var all []tea.Msg
	for i, inp := range inputs {
		msgs, err := inp.ToMessages()
		if err != nil {
			return nil, fmt.Errorf("input %d: %w", i, err)
		}
		all = append(all, msgs...)
	}
	return all, nil
}
