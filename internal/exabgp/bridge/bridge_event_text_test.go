// GOAL: the bridge writes the ExaBGP TEXT event format a script declaring
// `encoder text` reads, byte for byte.
//
// METHOD: feed the ze JSON event a peer's UPDATE produces and compare the whole
// rendered block against the lines ExaBGP's own Response.Text.update writes
// (src/exabgp/reactor/api/response/text.py). The announce line is the one
// test/exabgp-compat/etc/run/api-check.run reads off its stdin and exits 1
// without, so it is quoted here exactly as that script quotes it.

package bridge

import (
	"encoding/json"
	"strings"
	"testing"
)

// readTextEvent renders one ze JSON event as ExaBGP text.
func readTextEvent(t *testing.T, event string) (string, TextForm) {
	t.Helper()
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(event), &zebgp); err != nil {
		t.Fatalf("unmarshal the ze event: %v", err)
	}
	out, form := ReadEvent(zebgp).AppendText(nil)
	return string(out), form
}

// TestTextEncoderWritesTheAnnouncedLineAPICheckReads pins the exact line the
// compatibility corpus depends on. api-check.run compares its stdin against
// this string and announces 0.0.0.0/0 and exits 1 when it does not match, so a
// drift of one character is a failing compatibility case.
func TestTextEncoderWritesTheAnnouncedLineAPICheckReads(t *testing.T) {
	event := `{"type":"bgp","bgp":{
		"message":{"type":"update","id":1,"direction":"received"},
		"peer":{"local":{"address":"127.0.0.1","as":65512},"remote":{"address":"127.0.0.1","as":65512}},
		"attr":{"origin":"igp","local-preference":100},
		"nlri":{"ipv4/unicast":[{"next-hop":"127.0.0.1","action":"add","nlri":["0.0.0.0/32"]}]}
	}}`

	got, form := readTextEvent(t, event)
	if form != TextWritten {
		t.Fatalf("form = %d, want TextWritten", form)
	}

	want := "neighbor 127.0.0.1 receive update start\n" +
		"neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100\n" +
		"neighbor 127.0.0.1 receive update end\n"
	if got != want {
		t.Errorf("text event:\n%q\nwant:\n%q", got, want)
	}
}

// TestTextEncoderWritesAWithdrawnLine checks the other half of an UPDATE.
// ExaBGP writes no attributes and no next hop on a withdrawn line, so a naive
// encoder that renders one shape for both directions fails here.
func TestTextEncoderWritesAWithdrawnLine(t *testing.T) {
	event := `{"type":"bgp","bgp":{
		"message":{"type":"update","direction":"received"},
		"peer":{"remote":{"address":"10.0.0.1","as":65001}},
		"attr":{"origin":"igp","local-preference":100},
		"nlri":{"ipv4/unicast":[{"action":"del","nlri":["10.0.0.0/24","10.0.1.0/24"]}]}
	}}`

	got, form := readTextEvent(t, event)
	if form != TextWritten {
		t.Fatalf("form = %d, want TextWritten", form)
	}

	want := "neighbor 10.0.0.1 receive update start\n" +
		"neighbor 10.0.0.1 receive update withdrawn 10.0.0.0/24\n" +
		"neighbor 10.0.0.1 receive update withdrawn 10.0.1.0/24\n" +
		"neighbor 10.0.0.1 receive update end\n"
	if got != want {
		t.Errorf("text event:\n%q\nwant:\n%q", got, want)
	}
}

// TestTextEncoderOrdersAttributesByCode proves the rendering follows ExaBGP's
// own order rather than the order a Go map walk produces. ExaBGP sorts the
// attributes of one UPDATE by attribute code (_generate_text), and a ze event
// delivers them in a JSON object, which has no order at all.
func TestTextEncoderOrdersAttributesByCode(t *testing.T) {
	event := `{"type":"bgp","bgp":{
		"message":{"type":"update","direction":"sent"},
		"peer":{"remote":{"address":"10.0.0.1","as":65001}},
		"attr":{"large-communities":["1:2:3"],"med":200,"communities":["2:1"],
		        "local-preference":100,"as-path":[65001,65002],"origin":"igp"},
		"nlri":{"ipv4/unicast":[{"next-hop":"1.1.1.1","action":"add","nlri":["1.0.0.0/24"]}]}
	}}`

	got, _ := readTextEvent(t, event)

	want := "neighbor 10.0.0.1 send update announced 1.0.0.0/24 next-hop 1.1.1.1" +
		" origin igp as-path [ 65001 65002 ] med 200 local-preference 100" +
		" community [ 2:1 ] large-community [ 1:2:3 ]\n"
	if !strings.Contains(got, want) {
		t.Errorf("text event:\n%q\ndoes not carry:\n%q", got, want)
	}
}

// TestTextEncoderWritesTheSessionLines covers the kinds a script watching a
// session reads: the three state changes, a KEEPALIVE and a NOTIFICATION.
func TestTextEncoderWritesTheSessionLines(t *testing.T) {
	cases := []struct {
		name  string
		event string
		want  string
	}{
		{
			name: "up",
			event: `{"type":"bgp","bgp":{"message":{"type":"state"},
			         "peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"up"}}`,
			want: "neighbor 10.0.0.1 up\n",
		},
		{
			name: "connected",
			event: `{"type":"bgp","bgp":{"message":{"type":"state"},
			         "peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"connected"}}`,
			want: "neighbor 10.0.0.1 connected\n",
		},
		{
			name: "down",
			event: `{"type":"bgp","bgp":{"message":{"type":"state"},
			         "peer":{"remote":{"address":"10.0.0.1","as":65001}},
			         "state":"down","reason":"hold timer expired"}}`,
			want: "neighbor 10.0.0.1 down - hold timer expired\n",
		},
		{
			name: "keepalive",
			event: `{"type":"bgp","bgp":{"message":{"type":"keepalive","direction":"sent"},
			         "peer":{"remote":{"address":"10.0.0.1","as":65001}},"keepalive":{}}}`,
			want: "neighbor 10.0.0.1 send keepalive\n",
		},
		{
			name: "notification",
			event: `{"type":"bgp","bgp":{"message":{"type":"notification","direction":"sent"},
			         "peer":{"remote":{"address":"10.0.0.1","as":65001}},
			         "notification":{"code":6,"subcode":2,"data":"41424344"}}}`,
			want: "neighbor 10.0.0.1 send notification code 6 subcode 2 data 41424344\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, form := readTextEvent(t, tc.event)
			if form != TextWritten {
				t.Fatalf("form = %d, want TextWritten", form)
			}
			if got != tc.want {
				t.Errorf("text event = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTextEncoderWritesNothingForTheKindsExaBGPSkips proves the two silences
// are told apart. ExaBGP's Text encoder answers None for `negotiated`, `fsm`
// and `signal`, so a script gets no line; an event kind this encoder does not
// know is a gap and is reported so the caller can log it.
func TestTextEncoderWritesNothingForTheKindsExaBGPSkips(t *testing.T) {
	skipped := `{"type":"bgp","bgp":{"message":{"type":"negotiated"},
	             "peer":{"remote":{"address":"10.0.0.1","as":65001}},
	             "negotiated":{"hold-time":90,"asn4":true}}}`
	got, form := readTextEvent(t, skipped)
	if form != TextNone {
		t.Errorf("negotiated: form = %d, want TextNone", form)
	}
	if got != "" {
		t.Errorf("negotiated: text = %q, want no line", got)
	}

	unknown := `{"type":"bgp","bgp":{"message":{"type":"operational"},
	             "peer":{"remote":{"address":"10.0.0.1","as":65001}},"operational":{}}}`
	if _, form := readTextEvent(t, unknown); form != TextUnknown {
		t.Errorf("operational: form = %d, want TextUnknown", form)
	}
}

// TestTextEncoderKeepsOneEventOnOneLine is the security property of the format.
// A consumer splits the stream on newlines, so a value a PEER chose must not be
// able to end the line: a shutdown reason carrying a newline would otherwise
// forge a whole event on the script's stdin (CWE-116, the reason ExaBGP's own
// oneline() exists).
func TestTextEncoderKeepsOneEventOnOneLine(t *testing.T) {
	event := `{"type":"bgp","bgp":{"message":{"type":"state"},
	           "peer":{"remote":{"address":"10.0.0.1","as":65001}},
	           "state":"down","reason":"bye\nneighbor 1.2.3.4 down - forged"}}`

	got, _ := readTextEvent(t, event)
	if strings.Count(got, "\n") != 1 {
		t.Errorf("text = %q, want exactly one newline: the terminator", got)
	}
	if !strings.Contains(got, `bye\nneighbor 1.2.3.4 down - forged`) {
		t.Errorf("text = %q, want the newline escaped rather than dropped", got)
	}
}

// TestParseEncoderRefusesAThirdWord checks the config leaf fails closed. A word
// that names neither format is refused rather than answered with one of them.
func TestParseEncoderRefusesAThirdWord(t *testing.T) {
	for _, word := range []string{"text", "json"} {
		if _, err := ParseEncoder(word); err != nil {
			t.Errorf("ParseEncoder(%q): %v", word, err)
		}
	}
	got, err := ParseEncoder("yaml")
	if err == nil {
		t.Fatalf("ParseEncoder(\"yaml\") answered %v and no error", got)
	}
	if got != EncoderUnspecified {
		t.Errorf("ParseEncoder(\"yaml\") = %v, want EncoderUnspecified", got)
	}
}

// TestReadEventFoldsTheSentUpdateKind proves the bridge learns ze's own name
// for an UPDATE this speaker sent. ze publishes it as `message.type: "sent"`
// with the body under `update`, and ze's own reader folds it back
// (internal/component/bgp/event.go, the EventKindSent branch).
//
// Without the fold a script is never told about a route ze announced: the
// ExaBGP vocabulary has no `sent` event, so the JSON encoder wrote an envelope
// with no message in it.
func TestReadEventFoldsTheSentUpdateKind(t *testing.T) {
	event := `{"type":"bgp","bgp":{
		"message":{"type":"sent","id":5,"direction":"sent"},
		"peer":{"remote":{"address":"127.0.0.1","as":65512}},
		"update":{"attr":{"origin":"igp"},
		          "nlri":{"ipv4/unicast":[{"next-hop":"127.0.0.2","action":"add","nlri":["127.0.0.1/32"]}]}}
	}}`

	var zebgp map[string]any
	if err := json.Unmarshal([]byte(event), &zebgp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	read := ReadEvent(zebgp)

	if read.Kind != msgTypeUpdate {
		t.Errorf("Kind = %q, want update", read.Kind)
	}
	if read.Direction != modeSend {
		t.Errorf("Direction = %q, want send", read.Direction)
	}
	if got := read.APIKey(); got != "send-update" {
		t.Errorf("APIKey = %q, want send-update", got)
	}

	got, form := read.AppendText(nil)
	if form != TextWritten {
		t.Fatalf("form = %d, want TextWritten", form)
	}
	want := "neighbor 127.0.0.1 send update announced 127.0.0.1/32 next-hop 127.0.0.2 origin igp\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("text event:\n%q\ndoes not carry:\n%q", got, want)
	}
}

// TestReadEventRefusesToNameAnEnvelopeWithNoMessage pins the other half. The
// bridge subscribes to every event ze publishes, so it meets envelopes that
// carry no BGP message; naming one `update` sent every script an UPDATE that
// announced nothing, from a peer named by the empty string.
func TestReadEventRefusesToNameAnEnvelopeWithNoMessage(t *testing.T) {
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(`{"direction":"received","peer":"127.0.0.1"}`), &zebgp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	read := ReadEvent(zebgp)
	if read.Kind != "" {
		t.Errorf("Kind = %q, want the empty string: this envelope carries no message", read.Kind)
	}

	// An UPDATE body with no metadata is still named, by the keys it carries.
	if err := json.Unmarshal([]byte(`{"attr":{"origin":"igp"},"nlri":{}}`), &zebgp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := ReadEvent(zebgp).Kind; got != msgTypeUpdate {
		t.Errorf("Kind = %q, want update", got)
	}
}

// TestTextEncoderOmitsAnEmptyAttributeList pins the one token that failed
// api-check. A route this speaker ORIGINATED has an empty AS_PATH, ze reports
// it as `"as-path": []`, and ExaBGP writes nothing for it: `_generate_text`
// renders a list attribute through `str(attribute)` and skips it when that
// answers the empty string, which `ASPath.string` does for a path with no
// segments.
//
// api-check.run reads the whole line and exits 1 on the first one that is not
// the announce it waits for, so ` as-path [ ]` was the difference between the
// case passing and the case announcing 0.0.0.0/0 and failing.
func TestTextEncoderOmitsAnEmptyAttributeList(t *testing.T) {
	event := `{"type":"bgp","bgp":{
		"message":{"type":"update","direction":"received"},
		"peer":{"remote":{"address":"127.0.0.1","as":65512}},
		"attr":{"origin":"igp","as-path":[],"communities":[],"local-preference":100},
		"nlri":{"ipv4/unicast":[{"next-hop":"127.0.0.1","action":"add","nlri":["0.0.0.0/32"]}]}
	}}`

	got, form := readTextEvent(t, event)
	if form != TextWritten {
		t.Fatalf("form = %d, want TextWritten", form)
	}

	want := "neighbor 127.0.0.1 receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100\n"
	if !strings.Contains(got, want) {
		t.Errorf("text event:\n%q\nwant it to carry:\n%q", got, want)
	}
	for _, unwanted := range []string{"as-path", "community"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("text event carries %q for an empty list:\n%q", unwanted, got)
		}
	}

	// A list with members is still written, so the skip is about EMPTINESS and
	// not about the attribute.
	filled := `{"type":"bgp","bgp":{
		"message":{"type":"update","direction":"received"},
		"peer":{"remote":{"address":"127.0.0.1","as":65512}},
		"attr":{"origin":"igp","as-path":[65001]},
		"nlri":{"ipv4/unicast":[{"next-hop":"127.0.0.1","action":"add","nlri":["0.0.0.0/32"]}]}
	}}`
	got, _ = readTextEvent(t, filled)
	if !strings.Contains(got, "origin igp as-path [ 65001 ]\n") {
		t.Errorf("text event:\n%q\nwant it to carry a non-empty as-path", got)
	}
}
