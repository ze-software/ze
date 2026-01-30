package peer

import (
	"encoding/hex"
	"strings"
	"testing"
)

// TestNotificationMsg verifies NOTIFICATION message construction per RFC 9003.
//
// VALIDATES: NotificationMsg builds RFC 9003 compliant NOTIFICATION with:
//   - Error Code 6 (Cease)
//   - Subcode 2 (Administrative Shutdown)
//   - Length byte
//   - UTF-8 shutdown communication (max 255 bytes)
//
// PREVENTS: Malformed notification messages being sent to ZeBGP during tests.
func TestNotificationMsg(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantHex string
	}{
		{
			name: "simple shutdown message",
			text: "closing session because we can",
			// RFC 9003 format:
			// Marker (16) + Length (2) + Type (1) + Code (1) + Subcode (1) + MsgLen (1) + Text
			wantHex: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" + // Marker
				"0034" + // Length: 19 + 3 + 30 = 52 = 0x34
				"03" + // Type: NOTIFICATION
				"06" + // Error code: Cease
				"02" + // Subcode: 2 (Administrative Shutdown per RFC 9003)
				"1E" + // Message length: 30 = 0x1E
				hex.EncodeToString([]byte("closing session because we can")),
		},
		{
			name: "empty message",
			text: "",
			wantHex: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" +
				"0016" + // Length: 19 + 3 + 0 = 22 = 0x16
				"03" +
				"06" +
				"02" +
				"00", // Message length: 0
		},
		{
			name: "short message",
			text: "bye",
			wantHex: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" +
				"0019" + // Length: 19 + 3 + 3 = 25 = 0x19
				"03" +
				"06" +
				"02" +
				"03" + // Message length: 3
				hex.EncodeToString([]byte("bye")),
		},
		{
			name: "utf8 with emoji",
			text: "goodbye 👋",
			// "goodbye " = 8 bytes, "👋" = 4 bytes (U+1F44B), total 12 bytes
			wantHex: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" +
				"0022" + // Length: 19 + 3 + 12 = 34 = 0x22
				"03" +
				"06" +
				"02" +
				"0C" + // Message length: 12 = 0x0C
				hex.EncodeToString([]byte("goodbye 👋")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NotificationMsg(tt.text)
			gotHex := strings.ToUpper(hex.EncodeToString(got))
			wantHex := strings.ToUpper(tt.wantHex)

			if gotHex != wantHex {
				t.Errorf("NotificationMsg(%q) =\n  got:  %s\n  want: %s", tt.text, gotHex, wantHex)
			}

			// Verify message structure (min 22 bytes: header + code + subcode + len)
			if len(got) < 22 {
				t.Fatalf("message too short: %d bytes", len(got))
			}

			// Check marker
			for i := 0; i < 16; i++ {
				if got[i] != 0xFF {
					t.Errorf("marker byte %d = %02x, want FF", i, got[i])
				}
			}

			// Check type
			if got[18] != MsgNOTIFICATION {
				t.Errorf("type = %d, want %d (NOTIFICATION)", got[18], MsgNOTIFICATION)
			}

			// Check error code (Cease = 6)
			if got[19] != 6 {
				t.Errorf("error code = %d, want 6 (Cease)", got[19])
			}

			// Check error subcode (Administrative Shutdown = 2, per RFC 9003)
			if got[20] != 2 {
				t.Errorf("error subcode = %d, want 2 (Administrative Shutdown)", got[20])
			}

			// Check length byte matches actual text
			textLen := got[21]
			if int(textLen) != len(tt.text) && len(tt.text) <= 255 {
				t.Errorf("length byte = %d, want %d", textLen, len(tt.text))
			}
		})
	}
}

// TestCheckerNotificationAction verifies that action:notification is treated as a SEND action.
//
// VALIDATES: Checker correctly identifies notification action, not receive expectation.
//
// PREVENTS: Testpeer hanging waiting to receive notification instead of sending it.
func TestCheckerNotificationAction(t *testing.T) {
	tests := []struct {
		name     string
		expected []string
		want     string
	}{
		{
			name:     "notification action",
			expected: []string{"action:notification:conn=1:seq=1:text=closing session because we can"},
			want:     "closing session because we can",
		},
		{
			name:     "mixed case",
			expected: []string{"action:notification:conn=1:seq=1:text=Closing Session"},
			want:     "Closing Session", // Preserve case for notification text
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewChecker(tt.expected)
			if err != nil {
				t.Fatalf("NewChecker failed: %v", err)
			}
			c.Init()

			action, text := c.NextNotificationAction()
			if !action {
				t.Error("expected notification action, got none")
				return
			}

			if text != tt.want {
				t.Errorf("notification text = %q, want %q", text, tt.want)
			}
		})
	}
}

// TestCheckerNoNotificationAction verifies expect:bgp are not treated as actions.
//
// VALIDATES: BGP expectations are correctly identified as receive expectations.
//
// PREVENTS: Testpeer incorrectly trying to send BGP messages as notifications.
func TestCheckerNoNotificationAction(t *testing.T) {
	expected := []string{"expect:bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"}
	c, err := NewChecker(expected)
	if err != nil {
		t.Fatalf("NewChecker failed: %v", err)
	}
	c.Init()

	action, _ := c.NextNotificationAction()
	if action {
		t.Error("expect:bgp should not be treated as notification action")
	}
}
