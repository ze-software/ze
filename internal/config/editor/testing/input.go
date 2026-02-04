package testing

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Input kind constants.
const (
	inputKindType = "type"
	inputKindKey  = "key"
	inputKindCtrl = "ctrl"
)

// Input represents a runtime-typed input action for the editor.
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
// For "type" inputs, each character becomes a separate KeyRunes message.
// For "key" inputs, a single KeyMsg is returned.
// For "ctrl" inputs, a KeyMsg with Ctrl modifier is returned.
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

// toTypeMessages converts typed text to KeyRunes messages.
func (inp Input) toTypeMessages() []tea.Msg {
	msgs := make([]tea.Msg, 0, len(inp.Text))
	for _, r := range inp.Text {
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return msgs
}

// keyNameToType maps key names to tea.KeyType values.
var keyNameToType = map[string]tea.KeyType{
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
	"shift+tab": tea.KeyShiftTab,
}

// toKeyMessages converts a key name to a KeyMsg.
func (inp Input) toKeyMessages() ([]tea.Msg, error) {
	// Special case: "space" should insert a space character via KeyRunes,
	// not send KeySpace which doesn't insert text in textinput.
	if inp.Key == "space" {
		return []tea.Msg{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}}, nil
	}

	keyType, ok := keyNameToType[inp.Key]
	if !ok {
		return nil, fmt.Errorf("unknown key name: %s", inp.Key)
	}
	return []tea.Msg{tea.KeyMsg{Type: keyType}}, nil
}

// toCtrlMessages converts a ctrl+key to a KeyMsg.
func (inp Input) toCtrlMessages() ([]tea.Msg, error) {
	if len(inp.Key) != 1 {
		return nil, fmt.Errorf("ctrl key must be single character, got: %s", inp.Key)
	}

	// Ctrl+key is represented as the character with bit 6 cleared (ASCII 1-26).
	r := rune(inp.Key[0])

	// Convert to control code offset (1-26 for a-z)
	offset := ctrlKeyOffset(r)
	if offset < 0 {
		return nil, fmt.Errorf("ctrl key must be a-z, got: %s", inp.Key)
	}

	return []tea.Msg{tea.KeyMsg{Type: tea.KeyCtrlA + tea.KeyType(offset)}}, nil
}

// ctrlKeyOffset returns the control key offset (0-25) for a letter, or -1 if invalid.
func ctrlKeyOffset(r rune) int {
	if r >= 'a' && r <= 'z' {
		return int(r - 'a')
	}
	if r >= 'A' && r <= 'Z' {
		return int(r - 'A')
	}
	return -1
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
