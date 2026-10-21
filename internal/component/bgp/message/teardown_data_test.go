package message

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestBuildTeardownData pins the Data field an operator's notify sends.
//
// GOAL: the text goes on the wire as its own UTF-8 octets, as the RFC 9003
// Shutdown Communication under Cease 2 and 4 and as the Data field itself
// under every other pair, and no text means an empty Data field.
// METHOD: compare whole byte slices for each class.
//
// VALIDATES: BuildTeardownData for every branch that sends.
// PREVENTS: the subcode's name written where no text was given, the text
// dropped under a pair other than Cease 2 and 4, or a length octet added to a
// Data field that is not a Shutdown Communication.
func TestBuildTeardownData(t *testing.T) {
	cases := []struct {
		name    string
		code    NotifyErrorCode
		subcode uint8
		text    string
		want    []byte
	}{
		{"no text under admin shutdown is empty", NotifyCease, NotifyCeaseAdminShutdown, "", nil},
		{"no text under another pair is empty", NotifyUpdateMessage, 1, "", nil},
		{"admin shutdown carries a communication", NotifyCease, NotifyCeaseAdminShutdown, "test", []byte("\x04test")},
		{"admin reset carries a communication", NotifyCease, NotifyCeaseAdminReset, "back", []byte("\x04back")},
		{"a communication is UTF-8", NotifyCease, NotifyCeaseAdminShutdown, "café", []byte("\x05caf\xc3\xa9")},
		{"another cease subcode carries the text", NotifyCease, 3, "gone", []byte("gone")},
		{"another code carries the text", NotifyMessageHeader, 2, "bad length", []byte("bad length")},
		{"Data outside a communication is UTF-8", NotifyCease, 1, "café", []byte("caf\xc3\xa9")},
		{"a communication of exactly 128 octets", NotifyCease, NotifyCeaseAdminShutdown, strings.Repeat("a", MaxShutdownMessageLen), append([]byte{MaxShutdownMessageLen}, strings.Repeat("a", MaxShutdownMessageLen)...)},
		{"Data of exactly the field's maximum", NotifyCease, 1, strings.Repeat("a", MaxNotificationDataLen), []byte(strings.Repeat("a", MaxNotificationDataLen))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildTeardownData(tc.code, tc.subcode, tc.text)
			if err != nil {
				t.Fatalf("BuildTeardownData(%d, %d, %q) refused: %v", tc.code, tc.subcode, tc.text, err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Errorf("BuildTeardownData(%d, %d, %q) = %q, want %q", tc.code, tc.subcode, tc.text, got, tc.want)
			}
		})
	}
}

// TestBuildTeardownDataRefuses pins that a text which does not fit is refused,
// never cut.
//
// GOAL: an operator's NOTIFICATION text reaches the wire whole or not at all.
// METHOD: one octet past each limit, and an invalid UTF-8 octet, under the
// pairs each limit governs; every answer must be a refusal naming the fix.
//
// VALIDATES: RFC 9003 Section 3, the 128-octet limit to a peer not known to
// support 255; RFC 9003 Section 2, UTF-8; the Data field's maximum.
// PREVENTS: a silent cut that sends a text the operator never typed.
func TestBuildTeardownDataRefuses(t *testing.T) {
	cases := []struct {
		name    string
		code    NotifyErrorCode
		subcode uint8
		text    string
		want    error
		fix     string
	}{
		{"admin shutdown past 128 octets", NotifyCease, NotifyCeaseAdminShutdown, strings.Repeat("a", MaxShutdownMessageLen+1), errTeardownTextTooLong, "shorten the text to 128 octets or fewer"},
		{"admin reset past 128 octets", NotifyCease, NotifyCeaseAdminReset, strings.Repeat("é", 65), errTeardownTextTooLong, "shorten the text to 128 octets or fewer"},
		{"Data past the field's maximum", NotifyUpdateMessage, 1, strings.Repeat("a", MaxNotificationDataLen+1), errTeardownDataTooLong, "shorten it to 4075 octets or fewer"},
		{"invalid UTF-8 in a communication", NotifyCease, NotifyCeaseAdminShutdown, "bad\xff", errTeardownTextNotUTF8, "retype the text in UTF-8"},
		{"invalid UTF-8 in Data", NotifyCease, 1, "bad\xff", errTeardownTextNotUTF8, "retype the text in UTF-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildTeardownData(tc.code, tc.subcode, tc.text)
			if err == nil {
				t.Fatalf("BuildTeardownData sent %d octets, want a refusal", len(got))
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
			if !errors.Is(err, ErrTeardownTextRefused) {
				t.Errorf("got %v, want it to wrap ErrTeardownTextRefused", err)
			}
			if !strings.Contains(err.Error(), tc.fix) {
				t.Errorf("refusal %q does not name the fix %q", err, tc.fix)
			}
		})
	}
}
