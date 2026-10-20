package bridge

import (
	"errors"
	"strings"
	"testing"
)

// TestCommandAPIVersion holds CommandAPIVersion to ExaBGP's command_api_version
// row by row: every v6 root, `neighbor`, every key of V4_SIMPLE_TRANSLATIONS and
// every branch of translate_v4_to_v6, and the words that decide nothing.
//
// VALIDATES: X-1 (src/exabgp/reactor/api/dispatch/version.py and dispatch/v4.py
// at ExaBGP 685f772e4).
// PREVENTS: a 5.x helper held to v6 by a word ExaBGP reads as v4, or a typing
// mistake in a first line holding a helper to either version.
func TestCommandAPIVersion(t *testing.T) {
	cases := []struct {
		line string
		want APIVersion
	}{
		// V6_ROOTS.
		{"daemon shutdown", APIV6},
		{"session ack disable", APIV6},
		{"system api version 4", APIV6},
		{"rib flush out", APIV6},
		{"peer * announce route 10.0.0.0/24 next-hop 1.2.3.4", APIV6},
		{"peer", APIV6},
		// `neighbor` is v4 whatever follows.
		{"neighbor 10.0.0.1 announce route 10.0.0.0/24 next-hop 1.2.3.4", APIV4},
		{"neighbor", APIV4},
		// V4_SIMPLE_TRANSLATIONS, every key.
		{"shutdown", APIV4},
		{"reload", APIV4},
		{"restart", APIV4},
		{"status", APIV4},
		{"enable-ack", APIV4},
		{"disable-ack", APIV4},
		{"silence-ack", APIV4},
		{"enable-sync", APIV4},
		{"disable-sync", APIV4},
		{"reset", APIV4},
		{"ping", APIV4},
		{"bye", APIV4},
		{"help", APIV4},
		{"version", APIV4},
		{"crash", APIV4},
		{"queue-status", APIV4},
		// The other branches of translate_v4_to_v6.
		{"api version", APIV4},
		{"api version 6", APIV4},
		{"api", APIVersionAuto},
		{"api versions", APIVersionAuto},
		{"show adj-rib in", APIV4},
		{"show adj-rib out extensive", APIV4},
		{"show adj-rib", APIVersionAuto},
		{"show adj-rib both", APIVersionAuto},
		{"show neighbor", APIV4},
		{"show neighbor summary", APIV4},
		{"show", APIVersionAuto},
		{"show routes", APIVersionAuto},
		{"flush adj-rib out", APIV4},
		{"flush adj-rib in", APIVersionAuto},
		{"flush", APIVersionAuto},
		{"clear adj-rib in", APIV4},
		{"clear adj-rib out", APIV4},
		{"clear adj-rib", APIVersionAuto},
		{"clear", APIVersionAuto},
		{"create neighbor 10.0.0.9 local-as 65000", APIV4},
		{"create", APIVersionAuto},
		{"create peer", APIVersionAuto},
		{"delete neighbor 10.0.0.9", APIV4},
		{"delete", APIVersionAuto},
		{"teardown 2", APIV4},
		{"teardown", APIV4},
		{"announce route 10.0.0.0/24 next-hop 1.2.3.4", APIV4},
		{"announce", APIV4},
		{"withdraw route 10.0.0.0/24", APIV4},
		{"withdraw", APIV4},
		// Words neither dispatcher's vocabulary decides.
		{"group start", APIVersionAuto},
		{"group announce route 10.0.0.0/24 next-hop 1.2.3.4", APIVersionAuto},
		{"", APIVersionAuto},
		{"   ", APIVersionAuto},
		{"# a comment", APIVersionAuto},
		{"#neighbor 10.0.0.1", APIVersionAuto},
		{"anounce route 10.0.0.0/24 next-hop 1.2.3.4", APIVersionAuto},
		{"Announce route 10.0.0.0/24", APIVersionAuto},
		// Spaces and tabs split words as Python's str.split does.
		{"  peer\t*  announce", APIV6},
		{"\tannounce route", APIV4},
	}
	for _, tc := range cases {
		if got := CommandAPIVersion(tc.line); got != tc.want {
			t.Errorf("CommandAPIVersion(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

// TestGroupDispatchedWhateverTheVersion: `group` decides nothing, and a helper
// held to either version may write it.
//
// VALIDATES: X-4 ("`group` is the v6 dispatcher's whatever the version"), AC-8.
func TestGroupDispatchedWhateverTheVersion(t *testing.T) {
	for _, held := range []APIVersion{APIVersionAuto, APIV4, APIV6} {
		if err := held.Admits("group start"); err != nil {
			t.Errorf("held %v: group start refused: %v", held, err)
		}
	}
}

// TestHeldVersionRefusesTheOther: a helper held to one version is refused the
// other's commands, with the error every runner answers `error` for.
//
// VALIDATES: X-3, AC-5, AC-6.
func TestHeldVersionRefusesTheOther(t *testing.T) {
	cases := []struct {
		held    APIVersion
		line    string
		refused bool
	}{
		{APIV4, "peer * announce route 10.0.0.0/24 next-hop 1.2.3.4", true},
		{APIV4, "daemon shutdown", true},
		{APIV4, "announce route 10.0.0.0/24 next-hop 1.2.3.4", false},
		{APIV4, "neighbor 10.0.0.1 teardown 2", false},
		{APIV6, "announce route 10.0.0.0/24 next-hop 1.2.3.4", true},
		{APIV6, "neighbor 10.0.0.1 teardown 2", true},
		{APIV6, "peer * announce route 10.0.0.0/24 next-hop 1.2.3.4", false},
		{APIVersionAuto, "announce route 10.0.0.0/24 next-hop 1.2.3.4", false},
		{APIVersionAuto, "peer * announce route 10.0.0.0/24 next-hop 1.2.3.4", false},
	}
	for _, tc := range cases {
		err := tc.held.Admits(tc.line)
		if !tc.refused {
			if err != nil {
				t.Errorf("held %v, %q: refused: %v", tc.held, tc.line, err)
			}
			continue
		}
		if !errors.Is(err, ErrUnknownCommand) {
			t.Errorf("held %v, %q: got %v, want ErrUnknownCommand", tc.held, tc.line, err)
			continue
		}
		if !strings.Contains(err.Error(), "API "+tc.held.String()) {
			t.Errorf("held %v, %q: reason %q does not name the version", tc.held, tc.line, err)
		}
	}
}

// TestParseAPIVersion holds the parser to ExaBGP's environment/parsing.py
// api_version, including its two error sentences.
//
// VALIDATES: X-7, X-8; the boundary row `api version` 3, 4, 5, 6, 7.
func TestParseAPIVersion(t *testing.T) {
	good := map[string]APIVersion{"auto": APIVersionAuto, " AUTO ": APIVersionAuto, "0": APIVersionAuto, "4": APIV4, "6": APIV6}
	for word, want := range good {
		got, err := ParseAPIVersion(word)
		if err != nil {
			t.Errorf("ParseAPIVersion(%q): %v", word, err)
			continue
		}
		if got != want {
			t.Errorf("ParseAPIVersion(%q) = %v, want %v", word, got, want)
		}
	}
	bad := map[string]string{
		"3":    "API version must be auto, 4 or 6, got 3",
		"5":    "API version must be auto, 4 or 6, got 5",
		"7":    "API version must be auto, 4 or 6, got 7",
		"x":    "invalid API version: x",
		"":     "invalid API version: ",
		"4.0":  "invalid API version: 4.0",
		"json": "invalid API version: json",
	}
	for word, want := range bad {
		_, err := ParseAPIVersion(word)
		if err == nil {
			t.Errorf("ParseAPIVersion(%q) accepted", word)
			continue
		}
		if err.Error() != want {
			t.Errorf("ParseAPIVersion(%q) = %q, want %q", word, err, want)
		}
	}
}

// TestEncoderForVersion is the X-5 matrix: the format a helper reads for each
// API version and `encoder`.
//
// VALIDATES: X-5 (Processes._select_encoder, _adopt_v4, _adopt_v6), AC-5, AC-6, AC-7.
func TestEncoderForVersion(t *testing.T) {
	cases := []struct {
		version APIVersion
		encoder Encoder
		want    Format
	}{
		{APIVersionAuto, EncoderText, FormatV4Text},
		{APIVersionAuto, EncoderJSON, FormatV6JSON},
		{APIV4, EncoderText, FormatV4Text},
		{APIV4, EncoderJSON, FormatV4JSON},
		{APIV6, EncoderText, FormatV6JSON},
		{APIV6, EncoderJSON, FormatV6JSON},
	}
	for _, tc := range cases {
		if got := FormatFor(tc.version, tc.encoder); got != tc.want {
			t.Errorf("FormatFor(%v, %v) = %v, want %v", tc.version, tc.encoder, got, tc.want)
		}
	}
}

// TestAPIVersionCommand holds the bridge's answer to `api version` to
// ExaBGP's api_version_cmd and _show_api_version, byte for byte.
//
// VALIDATES: X-7, AC-9.
func TestAPIVersionCommand(t *testing.T) {
	cases := []struct {
		line    string
		helper  APIVersion
		setting APIVersion
		reply   string
		failed  bool
		newSet  APIVersion
	}{
		{"api version", APIV4, APIVersionAuto, "API version: auto (this helper: 4)\n", false, APIVersionAuto},
		// ExaBGP hands api_version_cmd the words after `version` unchanged, so a
		// trailing `json` is the value it parses as well as the format it asks for.
		{"api version json", APIV4, APIVersionAuto, `{"error": "invalid API version: json"}` + "\n", true, APIVersionAuto},
		{"api version 6 json", APIV4, APIVersionAuto, `{"status": "API version set", "version": "6", "note": "effective for helpers started afterwards"}` + "\n", false, APIV6},
		{"system api version", APIV6, APIV4, `{"api_version": "4", "helper_api_version": "6"}` + "\n", false, APIV4},
		{"api version 6", APIV4, APIVersionAuto, "API version set to 6 (effective for helpers started afterwards)\n", false, APIV6},
		{"api version auto", APIV4, APIV6, "API version set to auto (effective for helpers started afterwards)\n", false, APIVersionAuto},
		{"system api version 4", APIV6, APIVersionAuto, `{"status": "API version set", "version": "4", "note": "effective for helpers started afterwards"}` + "\n", false, APIV4},
		{"api version 5", APIV4, APIVersionAuto, "error: API version must be auto, 4 or 6, got 5\n", true, APIVersionAuto},
		{"system api version 5", APIV6, APIVersionAuto, `{"error": "API version must be auto, 4 or 6, got 5"}` + "\n", true, APIVersionAuto},
	}
	for _, tc := range cases {
		translation, err := Translator{}.Line(tc.line)
		if err != nil {
			t.Errorf("%q: refused: %v", tc.line, err)
			continue
		}
		if translation.Local != LocalAPIVersion {
			t.Errorf("%q: Local = %v, want LocalAPIVersion", tc.line, translation.Local)
			continue
		}
		setting := tc.setting
		reply, failed := translation.APIVersion.Answer(tc.helper, &setting)
		if string(reply) != tc.reply {
			t.Errorf("%q: reply %q, want %q", tc.line, reply, tc.reply)
		}
		if failed != tc.failed {
			t.Errorf("%q: failed %v, want %v", tc.line, failed, tc.failed)
		}
		if setting != tc.newSet {
			t.Errorf("%q: setting %v, want %v", tc.line, setting, tc.newSet)
		}
	}
}

// TestJSONEnvelopePerFormat: a v4 JSON helper reads ExaBGP's frozen v4 envelope,
// a v6 JSON helper the 6.0.0 one.
//
// VALIDATES: X-6 (`json_v4 = '4.0.1'`, src/exabgp/version.py), AC-5, AC-6.
func TestJSONEnvelopePerFormat(t *testing.T) {
	event := ReadEvent(map[string]any{"type": "state", "peer": map[string]any{"address": "127.0.0.1"}, "state": "up"})
	if got := event.ExabgpJSON(FormatV4JSON)["exabgp"]; got != "4.0.1" {
		t.Errorf("v4 JSON envelope = %v, want 4.0.1", got)
	}
	if got := event.ExabgpJSON(FormatV6JSON)["exabgp"]; got != "6.0.0" {
		t.Errorf("v6 JSON envelope = %v, want 6.0.0", got)
	}
}
