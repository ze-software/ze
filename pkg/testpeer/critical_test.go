package testpeer

import (
	"testing"
	"unicode/utf8"
)

// TestCaseSensitivityBug exposes case sensitivity issue in groupMessages.
//
// BUG: If .ci file has "A1:NOTIFICATION:..." (uppercase), the text case is NOT preserved.
func TestCaseSensitivityBug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantText string
	}{
		{
			name:     "lowercase notification preserves case",
			input:    "A1:notification:Hello World",
			wantText: "Hello World",
		},
		{
			name:     "UPPERCASE NOTIFICATION should preserve case",
			input:    "A1:NOTIFICATION:Hello World",
			wantText: "Hello World", // BUG: Currently returns "hello world"
		},
		{
			name:     "Mixed case Notification should preserve case",
			input:    "A1:Notification:Hello World",
			wantText: "Hello World", // BUG: Currently returns "hello world"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker([]string{tt.input})
			c.Init()

			ok, text := c.NextNotificationAction()
			if !ok {
				t.Fatalf("expected notification action, got none")
			}

			if text != tt.wantText {
				t.Errorf("text = %q, want %q (case not preserved!)", text, tt.wantText)
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
	input := "A1:notification:time: 12:30:45 zone: UTC"
	c := NewChecker([]string{input})
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

// TestNotificationSequence tests notification at non-first position.
func TestNotificationSequence(t *testing.T) {
	inputs := []string{
		"A1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000", // First: raw expectation
		"A2:notification:session ending",                        // Second: notification action
	}

	c := NewChecker(inputs)
	c.Init()

	// First should NOT be a notification
	ok, _ := c.NextNotificationAction()
	if ok {
		t.Error("A1:raw should not be a notification action")
	}

	// Simulate receiving the expected message (this advances the queue)
	// In practice, Expected() would be called with the matching message

	// For this test, manually check that A2 would be recognized
	// after A1 is consumed
}

// TestMultiConnectionSequences verifies that connection letters (A, B, C) create separate sequences.
//
// BUG: After lowercasing raw rules, connection letter detection fails because
// we check for uppercase 'A'-'Z' but the prefix is now lowercase.
func TestMultiConnectionSequences(t *testing.T) {
	inputs := []string{
		"A1:raw:AAAA",
		"B1:raw:BBBB",
		"C1:raw:CCCC",
	}

	c := NewChecker(inputs)

	// Should have 3 sequences (one per connection)
	if len(c.sequences) != 3 {
		t.Errorf("expected 3 sequences, got %d", len(c.sequences))
		t.Logf("sequences: %v", c.sequences)
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
