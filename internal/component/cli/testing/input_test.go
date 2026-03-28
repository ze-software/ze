package testing

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireKeyPress extracts a KeyPressMsg from a tea.Msg, failing if not possible.
func requireKeyPress(t *testing.T, msg tea.Msg) tea.KeyPressMsg {
	t.Helper()
	km, ok := msg.(tea.KeyPressMsg)
	require.True(t, ok, "expected tea.KeyPressMsg, got %T", msg)
	return km
}

// TestInputTypeToMessages verifies typed text becomes KeyPressMsg messages.
//
// VALIDATES: Each character in text input produces a KeyPressMsg with text.
// PREVENTS: Text not delivered to editor character by character.
func TestInputTypeToMessages(t *testing.T) {
	inp := Input{Kind: "type", Text: "abc"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 3)

	// Each character should be a KeyPressMsg with Text set
	for i, msg := range msgs {
		km := requireKeyPress(t, msg)
		assert.NotEmpty(t, km.Text, "message %d should have text", i)
	}

	// Check actual characters
	assert.Equal(t, "a", requireKeyPress(t, msgs[0]).Text)
	assert.Equal(t, "b", requireKeyPress(t, msgs[1]).Text)
	assert.Equal(t, "c", requireKeyPress(t, msgs[2]).Text)
}

// TestInputTypeWithSpaces verifies spaces are preserved.
//
// VALIDATES: Spaces in text input become KeyPressMsg messages.
// PREVENTS: Spaces lost during conversion.
func TestInputTypeWithSpaces(t *testing.T) {
	inp := Input{Kind: "type", Text: "edit bgp"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 8)

	// Check space character
	assert.Equal(t, " ", requireKeyPress(t, msgs[4]).Text)
}

// TestInputKeyTab verifies tab key conversion.
//
// VALIDATES: input=tab produces KeyPressMsg with KeyTab code.
// PREVENTS: Tab completion not triggering.
func TestInputKeyTab(t *testing.T) {
	inp := Input{Kind: "key", Key: "tab"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyPress(t, msgs[0])
	assert.Equal(t, tea.KeyTab, km.Code)
}

// TestInputKeyEnter verifies enter key conversion.
//
// VALIDATES: input=enter produces KeyPressMsg with KeyEnter code.
// PREVENTS: Commands not executing.
func TestInputKeyEnter(t *testing.T) {
	inp := Input{Kind: "key", Key: "enter"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyPress(t, msgs[0])
	assert.Equal(t, tea.KeyEnter, km.Code)
}

// TestInputKeyArrows verifies arrow key conversion.
//
// VALIDATES: Arrow keys produce correct KeyPressMsg codes.
// PREVENTS: Navigation not working in dropdown.
func TestInputKeyArrows(t *testing.T) {
	tests := []struct {
		key      string
		expected rune
	}{
		{"up", tea.KeyUp},
		{"down", tea.KeyDown},
		{"left", tea.KeyLeft},
		{"right", tea.KeyRight},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			inp := Input{Kind: "key", Key: tt.key}
			msgs, err := inp.ToMessages()
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			assert.Equal(t, tt.expected, requireKeyPress(t, msgs[0]).Code)
		})
	}
}

// TestInputKeyEscape verifies escape key conversion.
//
// VALIDATES: Both "esc" and "escape" produce KeyEscape code.
// PREVENTS: Escape not closing dropdown.
func TestInputKeyEscape(t *testing.T) {
	for _, key := range []string{"esc", "escape"} {
		t.Run(key, func(t *testing.T) {
			inp := Input{Kind: "key", Key: key}
			msgs, err := inp.ToMessages()
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			assert.Equal(t, tea.KeyEscape, requireKeyPress(t, msgs[0]).Code)
		})
	}
}

// TestInputKeySpecial verifies special key conversions.
//
// VALIDATES: Special keys (backspace, delete, etc.) convert correctly.
// PREVENTS: Text editing keys not working.
func TestInputKeySpecial(t *testing.T) {
	// Note: "space" is handled specially - it returns a KeyPressMsg with Text=" "
	// for textinput text insertion. See TestInputSpaceSpecial for that case.
	tests := []struct {
		key      string
		expected rune
	}{
		{"backspace", tea.KeyBackspace},
		{"delete", tea.KeyDelete},
		{"home", tea.KeyHome},
		{"end", tea.KeyEnd},
		{"pgup", tea.KeyPgUp},
		{"pgdn", tea.KeyPgDown},
		{"pgdown", tea.KeyPgDown},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			inp := Input{Kind: "key", Key: tt.key}
			msgs, err := inp.ToMessages()
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			assert.Equal(t, tt.expected, requireKeyPress(t, msgs[0]).Code)
		})
	}
}

// TestInputShiftTab verifies shift+tab uses modifier.
//
// VALIDATES: shift+tab produces KeyTab with ModShift.
// PREVENTS: Shift+Tab not working for reverse tab completion.
func TestInputShiftTab(t *testing.T) {
	inp := Input{Kind: "key", Key: "shift+tab"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	km := requireKeyPress(t, msgs[0])
	assert.Equal(t, tea.KeyTab, km.Code)
	assert.True(t, km.Mod.Contains(tea.ModShift), "shift+tab should have ModShift")
}

// TestInputSpaceSpecial verifies space is converted to text for textinput.
//
// VALIDATES: Space input inserts a space character via Text field.
// PREVENTS: Space not inserting text in textinput.
func TestInputSpaceSpecial(t *testing.T) {
	inp := Input{Kind: "key", Key: "space"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	km := requireKeyPress(t, msgs[0])
	assert.Equal(t, " ", km.Text, "space should produce Text=\" \"")
}

// TestInputKeyUnknown verifies error on unknown key.
//
// VALIDATES: Unknown key names produce error.
// PREVENTS: Silent failure on typos.
func TestInputKeyUnknown(t *testing.T) {
	inp := Input{Kind: "key", Key: "unknown"}
	_, err := inp.ToMessages()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key")
}

// TestInputCtrlC verifies ctrl+c conversion.
//
// VALIDATES: Ctrl+C produces KeyPressMsg with ModCtrl.
// PREVENTS: Ctrl combinations not working.
func TestInputCtrlC(t *testing.T) {
	inp := Input{Kind: "ctrl", Key: "c"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyPress(t, msgs[0])
	assert.Equal(t, 'c', km.Code)
	assert.True(t, km.Mod.Contains(tea.ModCtrl), "ctrl+c should have ModCtrl")
}

// TestInputCtrlU verifies ctrl+u conversion.
//
// VALIDATES: Ctrl+U produces KeyPressMsg with ModCtrl for clearing line.
// PREVENTS: Line clear not working.
func TestInputCtrlU(t *testing.T) {
	inp := Input{Kind: "ctrl", Key: "u"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyPress(t, msgs[0])
	assert.Equal(t, 'u', km.Code)
	assert.True(t, km.Mod.Contains(tea.ModCtrl), "ctrl+u should have ModCtrl")
}

// TestInputCtrlInvalid verifies error on invalid ctrl key.
//
// VALIDATES: Non-letter ctrl keys produce error.
// PREVENTS: Invalid ctrl combinations.
func TestInputCtrlInvalid(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"multi_char", "ab"},
		{"digit", "1"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inp := Input{Kind: "ctrl", Key: tt.key}
			_, err := inp.ToMessages()
			require.Error(t, err)
		})
	}
}

// TestInputUnknownKind verifies error on unknown input kind.
//
// VALIDATES: Unknown input kinds produce error.
// PREVENTS: Silent failure on invalid test files.
func TestInputUnknownKind(t *testing.T) {
	inp := Input{Kind: "invalid", Key: "x"}
	_, err := inp.ToMessages()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown input kind")
}

// TestInputsToMessages verifies batch conversion.
//
// VALIDATES: Multiple inputs convert to message sequence.
// PREVENTS: Input sequence broken.
func TestInputsToMessages(t *testing.T) {
	inputs := []Input{
		{Kind: "type", Text: "hi"},
		{Kind: "key", Key: "enter"},
	}

	msgs, err := InputsToMessages(inputs)
	require.NoError(t, err)
	// "hi" = 2 messages, enter = 1 message
	require.Len(t, msgs, 3)

	// First two are text characters
	assert.NotEmpty(t, requireKeyPress(t, msgs[0]).Text)
	assert.NotEmpty(t, requireKeyPress(t, msgs[1]).Text)
	// Last is enter
	assert.Equal(t, tea.KeyEnter, requireKeyPress(t, msgs[2]).Code)
}
