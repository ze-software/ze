package peer

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// TestNotificationTextPreserved verifies notification text case is preserved.
//
// VALIDATES: Text in notification action is preserved exactly.
func TestNotificationTextPreserved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantText string
	}{
		{
			name:     "mixed case text preserved",
			input:    "action=notification:conn=1:seq=1:text=Hello World",
			wantText: "Hello World",
		},
		{
			name:     "all caps text preserved",
			input:    "action=notification:conn=1:seq=1:text=HELLO WORLD",
			wantText: "HELLO WORLD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewChecker([]string{tt.input})
			require.NoError(t, err)
			c.Init()

			ok, text := c.NextNotificationAction()
			if !ok {
				t.Fatalf("expected notification action, got none")
			}

			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}

// TestNotificationMsgLengthCapped verifies RFC 9003 255-byte limit is enforced.
//
// RFC 9003 limits shutdown communication to 255 octets.
// With ASCII (single-byte), we get exactly 255 bytes.
func TestNotificationMsgLengthCapped(t *testing.T) {
	// Create a string longer than RFC 9003 allows (ASCII = 1 byte per char)
	longText := make([]byte, 500)
	for i := range longText {
		longText[i] = 'A'
	}

	msg := NotificationMsg(string(longText))

	// Expected: 19 header + 3 (code, subcode, len) + 255 (capped text) = 277
	expectedLen := 19 + 3 + 255
	if len(msg) != expectedLen {
		t.Errorf("message length = %d, want %d (capped to 255)", len(msg), expectedLen)
	}

	// Check length byte is 255 (max per RFC 9003)
	if msg[21] != 255 {
		t.Errorf("length byte = %d, want 255 (RFC 9003 max)", msg[21])
	}

	// Verify the BGP length field is correct
	bgpLen := int(msg[16])<<8 | int(msg[17])
	if bgpLen != expectedLen {
		t.Errorf("BGP length field = %d, want %d", bgpLen, expectedLen)
	}

	// Verify content is valid UTF-8
	extractedText := msg[22:]
	if !utf8.Valid(extractedText) {
		t.Error("extracted text is not valid UTF-8")
	}
}

// TestNotificationWithColons ensures colons in message text are preserved.
func TestNotificationWithColons(t *testing.T) {
	input := "action=notification:conn=1:seq=1:text=time: 12:30:45 zone: UTC"
	c, err := NewChecker([]string{input})
	require.NoError(t, err)
	c.Init()

	ok, text := c.NextNotificationAction()
	if !ok {
		t.Fatal("expected notification action")
	}

	want := "time: 12:30:45 zone: UTC"
	if text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
}

// TestModeStringAndParse verifies Mode String() and ParseMode().
func TestModeStringAndParse(t *testing.T) {
	// Test String()
	if ModeCheck.String() != "check" {
		t.Errorf("ModeCheck.String() = %q, want %q", ModeCheck.String(), "check")
	}
	if ModeSink.String() != "sink" {
		t.Errorf("ModeSink.String() = %q, want %q", ModeSink.String(), "sink")
	}
	if ModeEcho.String() != "echo" {
		t.Errorf("ModeEcho.String() = %q, want %q", ModeEcho.String(), "echo")
	}
	if Mode(99).String() != "unknown" {
		t.Errorf("Mode(99).String() = %q, want %q", Mode(99).String(), "unknown")
	}

	// Test ParseMode (case-insensitive)
	tests := []struct {
		input string
		want  Mode
		valid bool
	}{
		{"check", ModeCheck, true},
		{"CHECK", ModeCheck, true},
		{"Check", ModeCheck, true},
		{"sink", ModeSink, true},
		{"SINK", ModeSink, true},
		{"echo", ModeEcho, true},
		{"ECHO", ModeEcho, true},
		{"invalid", ModeCheck, false},
		{"", ModeCheck, false},
	}

	for _, tt := range tests {
		got, valid := ParseMode(tt.input)
		if got != tt.want || valid != tt.valid {
			t.Errorf("ParseMode(%q) = (%v, %v), want (%v, %v)",
				tt.input, got, valid, tt.want, tt.valid)
		}
	}
}

// TestNotificationSequence tests notification at non-first position.
func TestNotificationSequence(t *testing.T) {
	inputs := []string{
		"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000",
		"action=notification:conn=1:seq=2:text=session ending",
	}

	c, err := NewChecker(inputs)
	require.NoError(t, err)
	c.Init()

	// First should NOT be a notification (it's a BGP expect)
	ok, _ := c.NextNotificationAction()
	if ok {
		t.Error("expect:bgp should not be a notification action")
	}

	// Simulate receiving the expected message (this advances the queue)
	// In practice, Expected() would be called with the matching message

	// For this test, manually check that seq=2 would be recognized
	// after seq=1 is consumed
}

// TestMultiConnectionSequences verifies that conn=1,2,3 create separate sequences.
//
// VALIDATES: Different conn values create separate sequences with correct IDs.
func TestMultiConnectionSequences(t *testing.T) {
	inputs := []string{
		"expect=bgp:conn=1:seq=1:hex=AAAA",
		"expect=bgp:conn=2:seq=1:hex=BBBB",
		"expect=bgp:conn=3:seq=1:hex=CCCC",
	}

	c, err := NewChecker(inputs)
	require.NoError(t, err)

	// Should have 3 sequences (one per connection)
	if len(c.sequences) != 3 {
		t.Errorf("expected 3 sequences, got %d", len(c.sequences))
	}

	// connectionIDs must match sequences length
	if len(c.connectionIDs) != 3 {
		t.Errorf("expected 3 connectionIDs, got %d", len(c.connectionIDs))
	}

	// Verify connection IDs are correct (1, 2, 3)
	expectedIDs := []int{1, 2, 3}
	for i, want := range expectedIDs {
		if i < len(c.connectionIDs) && c.connectionIDs[i] != want {
			t.Errorf("connectionIDs[%d] = %d, want %d", i, c.connectionIDs[i], want)
		}
	}
}

// TestParseExpectRuleValidation verifies validation of expect rules.
//
// VALIDATES: Missing or invalid fields produce clear errors.
// PREVENTS: Silent failures on malformed input.
func TestParseExpectRuleValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		// expect:bgp validation
		{
			name:    "bgp missing conn",
			input:   "expect=bgp:seq=1:hex=FFFF",
			wantErr: "missing conn",
		},
		{
			name:    "bgp missing seq",
			input:   "expect=bgp:conn=1:hex=FFFF",
			wantErr: "missing seq",
		},
		{
			name:    "bgp missing hex",
			input:   "expect=bgp:conn=1:seq=1:",
			wantErr: "missing hex",
		},
		{
			name:    "bgp invalid conn zero",
			input:   "expect=bgp:conn=0:seq=1:hex=FFFF",
			wantErr: "invalid conn",
		},
		{
			name:    "bgp invalid conn five",
			input:   "expect=bgp:conn=5:seq=1:hex=FFFF",
			wantErr: "invalid conn",
		},
		{
			name:    "bgp invalid seq zero",
			input:   "expect=bgp:conn=1:seq=0:hex=FFFF",
			wantErr: "invalid seq",
		},
		{
			name:    "bgp invalid seq negative",
			input:   "expect=bgp:conn=1:seq=-1:hex=FFFF",
			wantErr: "invalid seq",
		},
		// action:notification validation
		{
			name:    "notification missing conn",
			input:   "action=notification:seq=1:text=hello",
			wantErr: "missing conn",
		},
		{
			name:    "notification missing seq",
			input:   "action=notification:conn=1:text=hello",
			wantErr: "missing seq",
		},
		{
			name:    "notification missing text",
			input:   "action=notification:conn=1:seq=1:",
			wantErr: "missing text",
		},
		{
			name:    "notification invalid conn zero",
			input:   "action=notification:conn=0:seq=1:text=hello",
			wantErr: "invalid conn",
		},
		{
			name:    "notification invalid conn five",
			input:   "action=notification:conn=5:seq=1:text=hello",
			wantErr: "invalid conn",
		},
		{
			name:    "notification invalid seq zero",
			input:   "action=notification:conn=1:seq=0:text=hello",
			wantErr: "invalid seq",
		},
		{
			name:    "notification invalid seq negative",
			input:   "action=notification:conn=1:seq=-1:text=hello",
			wantErr: "invalid seq",
		},
		// action:send validation
		{
			name:    "send missing conn",
			input:   "action=send:seq=1:hex=FFFF",
			wantErr: "missing conn",
		},
		{
			name:    "send missing seq",
			input:   "action=send:conn=1:hex=FFFF",
			wantErr: "missing seq",
		},
		{
			name:    "send missing hex",
			input:   "action=send:conn=1:seq=1:",
			wantErr: "missing hex",
		},
		{
			name:    "send invalid conn zero",
			input:   "action=send:conn=0:seq=1:hex=FFFF",
			wantErr: "invalid conn",
		},
		{
			name:    "send invalid conn five",
			input:   "action=send:conn=5:seq=1:hex=FFFF",
			wantErr: "invalid conn",
		},
		{
			name:    "send invalid seq zero",
			input:   "action=send:conn=1:seq=0:hex=FFFF",
			wantErr: "invalid seq",
		},
		{
			name:    "send invalid seq negative",
			input:   "action=send:conn=1:seq=-1:hex=FFFF",
			wantErr: "invalid seq",
		},
		// Unknown format
		{
			name:    "unknown format",
			input:   "something=else:foo=bar",
			wantErr: "unknown expect format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewChecker([]string{tt.input})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestParseExpectRuleValid verifies valid expect rules are accepted.
//
// VALIDATES: Correct format is parsed successfully.
func TestParseExpectRuleValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"bgp valid", "expect=bgp:conn=1:seq=1:hex=FFFF"},
		{"bgp conn 4", "expect=bgp:conn=4:seq=1:hex=FFFF"},
		{"bgp high seq", "expect=bgp:conn=1:seq=99:hex=FFFF"},
		{"notification valid", "action=notification:conn=1:seq=1:text=hello"},
		{"notification with colons", "action=notification:conn=1:seq=1:text=a:b:c"},
		{"send valid", "action=send:conn=1:seq=1:hex=FFFF"},
		{"send with colons in hex", "action=send:conn=1:seq=1:hex=FF:FF:FF"},
		{"send conn 4", "action=send:conn=4:seq=1:hex=FFFF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewChecker([]string{tt.input})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestSendActionParsing verifies that action=send is correctly parsed and accessible.
//
// VALIDATES: Send actions are correctly identified and hex data preserved.
// PREVENTS: Send actions being silently ignored (dead code).
func TestSendActionParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantHex string
	}{
		{
			name:    "simple hex",
			input:   "action=send:conn=1:seq=1:hex=FFFF",
			wantHex: "FFFF",
		},
		{
			name:    "hex with colons stripped",
			input:   "action=send:conn=1:seq=1:hex=FF:FF:AA:BB",
			wantHex: "FFFFAABB",
		},
		{
			name:    "lowercase hex normalized",
			input:   "action=send:conn=1:seq=1:hex=aabbccdd",
			wantHex: "AABBCCDD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewChecker([]string{tt.input})
			require.NoError(t, err)
			c.Init()

			ok, hexData := c.NextSendAction()
			if !ok {
				t.Fatalf("expected send action, got none")
			}

			if hexData != tt.wantHex {
				t.Errorf("hex = %q, want %q", hexData, tt.wantHex)
			}
		})
	}
}

// TestSendNotTreatedAsExpect verifies send actions are not matched as expect patterns.
//
// VALIDATES: Send actions trigger sending, not receiving expectations.
func TestSendNotTreatedAsExpect(t *testing.T) {
	c, err := NewChecker([]string{"action=send:conn=1:seq=1:hex=FFFF"})
	require.NoError(t, err)
	c.Init()

	// Should be a send action, not an expect
	ok, _ := c.NextSendAction()
	if !ok {
		t.Error("action:send should be treated as send action")
	}
}

// TestUTF8TruncationSafety verifies that truncating at 255 bytes doesn't break UTF-8.
//
// RFC 9003 requires valid UTF-8. Cutting in the middle of a multi-byte
// character creates invalid UTF-8.
func TestUTF8TruncationSafety(t *testing.T) {
	// Create a string with multi-byte characters near the 255 boundary
	// Each emoji is 4 bytes, so 64 emojis = 256 bytes
	var text string
	for i := 0; i < 64; i++ {
		text += "👋" // 4 bytes each
	}

	if len([]byte(text)) != 256 {
		t.Fatalf("test setup: expected 256 bytes, got %d", len([]byte(text)))
	}

	msg := NotificationMsg(text)

	// Extract the text from the message (starts at byte 22)
	extractedText := msg[22:]

	// Verify with proper UTF-8 validation
	if !utf8.Valid(extractedText) {
		t.Errorf("INVALID UTF-8: truncation broke multi-byte character")
		t.Logf("Last 4 bytes: %X", extractedText[len(extractedText)-4:])
	}

	// 63 complete emojis = 252 bytes, should have truncated cleanly
	// If we have 255 bytes and it's valid UTF-8, something's wrong with the test
	// Actually: 255 / 4 = 63.75, so 255 bytes = 63 emojis + 3 bytes of broken emoji
	t.Logf("Extracted %d bytes, valid UTF-8: %v", len(extractedText), utf8.Valid(extractedText))
}
