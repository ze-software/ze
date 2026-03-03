package testing

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireKeyMsg extracts a KeyMsg from a tea.Msg, failing if not possible.
func requireKeyMsg(t *testing.T, msg tea.Msg) tea.KeyMsg {
	t.Helper()
	km, ok := msg.(tea.KeyMsg)
	require.True(t, ok, "expected tea.KeyMsg, got %T", msg)
	return km
}

// TestInputTypeToMessages verifies typed text becomes KeyRunes messages.
//
// VALIDATES: Each character in text input produces a KeyRunes message.
// PREVENTS: Text not delivered to editor character by character.
func TestInputTypeToMessages(t *testing.T) {
	inp := Input{Kind: "type", Text: "abc"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 3)

	// Each character should be a KeyRunes message
	for i, msg := range msgs {
		km := requireKeyMsg(t, msg)
		assert.Equal(t, tea.KeyRunes, km.Type, "message %d", i)
		assert.Len(t, km.Runes, 1, "message %d", i)
	}

	// Check actual characters
	assert.Equal(t, 'a', requireKeyMsg(t, msgs[0]).Runes[0])
	assert.Equal(t, 'b', requireKeyMsg(t, msgs[1]).Runes[0])
	assert.Equal(t, 'c', requireKeyMsg(t, msgs[2]).Runes[0])
}

// TestInputTypeWithSpaces verifies spaces are preserved.
//
// VALIDATES: Spaces in text input become KeyRunes messages.
// PREVENTS: Spaces lost during conversion.
func TestInputTypeWithSpaces(t *testing.T) {
	inp := Input{Kind: "type", Text: "edit bgp"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 8)

	// Check space character
	assert.Equal(t, ' ', requireKeyMsg(t, msgs[4]).Runes[0])
}

// TestInputKeyTab verifies tab key conversion.
//
// VALIDATES: input=tab produces KeyTab message.
// PREVENTS: Tab completion not triggering.
func TestInputKeyTab(t *testing.T) {
	inp := Input{Kind: "key", Key: "tab"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyMsg(t, msgs[0])
	assert.Equal(t, tea.KeyTab, km.Type)
}

// TestInputKeyEnter verifies enter key conversion.
//
// VALIDATES: input=enter produces KeyEnter message.
// PREVENTS: Commands not executing.
func TestInputKeyEnter(t *testing.T) {
	inp := Input{Kind: "key", Key: "enter"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyMsg(t, msgs[0])
	assert.Equal(t, tea.KeyEnter, km.Type)
}

// TestInputKeyArrows verifies arrow key conversion.
//
// VALIDATES: Arrow keys produce correct KeyMsg types.
// PREVENTS: Navigation not working in dropdown.
func TestInputKeyArrows(t *testing.T) {
	tests := []struct {
		key      string
		expected tea.KeyType
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
			assert.Equal(t, tt.expected, requireKeyMsg(t, msgs[0]).Type)
		})
	}
}

// TestInputKeyEscape verifies escape key conversion.
//
// VALIDATES: Both "esc" and "escape" produce KeyEscape.
// PREVENTS: Escape not closing dropdown.
func TestInputKeyEscape(t *testing.T) {
	for _, key := range []string{"esc", "escape"} {
		t.Run(key, func(t *testing.T) {
			inp := Input{Kind: "key", Key: key}
			msgs, err := inp.ToMessages()
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			assert.Equal(t, tea.KeyEscape, requireKeyMsg(t, msgs[0]).Type)
		})
	}
}

// TestInputKeySpecial verifies special key conversions.
//
// VALIDATES: Special keys (backspace, delete, etc.) convert correctly.
// PREVENTS: Text editing keys not working.
func TestInputKeySpecial(t *testing.T) {
	// Note: "space" is handled specially - it returns KeyRunes{' '} for textinput
	// compatibility, not KeySpace. See TestInputSpaceSpecial for that case.
	tests := []struct {
		key      string
		expected tea.KeyType
	}{
		{"backspace", tea.KeyBackspace},
		{"delete", tea.KeyDelete},
		{"home", tea.KeyHome},
		{"end", tea.KeyEnd},
		{"pgup", tea.KeyPgUp},
		{"pgdn", tea.KeyPgDown},
		{"pgdown", tea.KeyPgDown},
		{"shift+tab", tea.KeyShiftTab},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			inp := Input{Kind: "key", Key: tt.key}
			msgs, err := inp.ToMessages()
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			assert.Equal(t, tt.expected, requireKeyMsg(t, msgs[0]).Type)
		})
	}
}

// TestInputSpaceSpecial verifies space is converted to KeyRunes for textinput.
//
// VALIDATES: Space input inserts a space character (not KeySpace event).
// PREVENTS: Space not inserting text in textinput.
func TestInputSpaceSpecial(t *testing.T) {
	inp := Input{Kind: "key", Key: "space"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	keyMsg := requireKeyMsg(t, msgs[0])
	assert.Equal(t, tea.KeyRunes, keyMsg.Type, "space should be KeyRunes, not KeySpace")
	assert.Equal(t, []rune{' '}, keyMsg.Runes, "space should insert a space character")
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
// VALIDATES: Ctrl+C produces correct KeyMsg.
// PREVENTS: Ctrl combinations not working.
func TestInputCtrlC(t *testing.T) {
	inp := Input{Kind: "ctrl", Key: "c"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyMsg(t, msgs[0])
	// Ctrl+C is KeyCtrlC (KeyCtrlA + 2)
	assert.Equal(t, tea.KeyCtrlC, km.Type)
}

// TestInputCtrlU verifies ctrl+u conversion.
//
// VALIDATES: Ctrl+U produces correct KeyMsg for clearing line.
// PREVENTS: Line clear not working.
func TestInputCtrlU(t *testing.T) {
	inp := Input{Kind: "ctrl", Key: "u"}
	msgs, err := inp.ToMessages()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	km := requireKeyMsg(t, msgs[0])
	// Ctrl+U is KeyCtrlU (KeyCtrlA + 20)
	assert.Equal(t, tea.KeyCtrlU, km.Type)
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

	// First two are type
	assert.Equal(t, tea.KeyRunes, requireKeyMsg(t, msgs[0]).Type)
	assert.Equal(t, tea.KeyRunes, requireKeyMsg(t, msgs[1]).Type)
	// Last is enter
	assert.Equal(t, tea.KeyEnter, requireKeyMsg(t, msgs[2]).Type)
}
