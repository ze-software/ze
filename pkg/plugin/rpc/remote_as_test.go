package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
)

// TestRemoteASRendersThroughReflectiveConsumers proves that every consumer
// that renders a value by reflection (encoding/json, fmt, log/slog) writes the
// AS number when it is known and the documented unknown form when it is not.
// A RemoteAS is a struct with unexported fields, so without its own methods
// json.Marshal writes `{}` and fmt writes `{65001 true}`: the defect
// handleShowBGPHealth shipped. Method: render a known and an unknown value
// through each consumer and compare the bytes.
func TestRemoteASRendersThroughReflectiveConsumers(t *testing.T) {
	known := RemoteASOf(65001)
	unknown := RemoteAS{}

	type untagged struct{ PeerAS RemoteAS }
	type omitted struct {
		PeerAS RemoteAS `json:"as,omitzero"`
	}

	jsonCases := []struct {
		name string
		in   any
		want string
	}{
		{"known untagged", untagged{known}, `{"PeerAS":65001}`},
		{"unknown untagged is null", untagged{unknown}, `{"PeerAS":null}`},
		{"AS 0 untagged is null", untagged{RemoteASOf(0)}, `{"PeerAS":null}`},
		{"known omitzero", omitted{known}, `{"as":65001}`},
		{"unknown omitzero is omitted", omitted{unknown}, `{}`},
		{"map value", map[string]any{"as": known}, `{"as":65001}`},
	}
	for _, tc := range jsonCases {
		got, err := json.Marshal(tc.in)
		if err != nil {
			t.Fatalf("%s: json.Marshal: %v", tc.name, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: json.Marshal = %s, want %s", tc.name, got, tc.want)
		}
	}

	fmtCases := []struct {
		verb string
		in   RemoteAS
		want string
	}{
		{"%v", known, "65001"},
		{"%s", known, "65001"},
		{"%v", unknown, "unknown"},
		{"%s", unknown, "unknown"},
	}
	for _, tc := range fmtCases {
		if got := fmt.Sprintf(tc.verb, tc.in); got != tc.want {
			t.Errorf("fmt %s of %#v = %q, want %q", tc.verb, tc.in, got, tc.want)
		}
	}

	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	log.Info("m", "peer-as", known, "other-as", unknown)
	if want := "level=INFO msg=m peer-as=65001 other-as=unknown\n"; logged.String() != want {
		t.Errorf("slog = %q, want %q", logged.String(), want)
	}
}

// TestRemoteASDecodesWhatItEncodes proves a RemoteAS read back from JSON is
// the value that was written, and that a value no peer can hold is refused
// rather than read as a number. Method: round-trip a known and an unknown
// value, then decode each refused input and expect an error.
func TestRemoteASDecodesWhatItEncodes(t *testing.T) {
	type holder struct{ PeerAS RemoteAS }

	for _, in := range []RemoteAS{RemoteASOf(4200000000), {}} {
		encoded, err := json.Marshal(holder{in})
		if err != nil {
			t.Fatalf("json.Marshal(%v): %v", in, err)
		}
		var out holder
		if err := json.Unmarshal(encoded, &out); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", encoded, err)
		}
		if out.PeerAS != in {
			t.Errorf("round trip of %s gave %#v, want %#v", encoded, out.PeerAS, in)
		}
	}

	// RFC 7607 Section 2: AS 0 names no peer, so a document claiming it is
	// refused, and so is anything that is not a 4-octet AS number.
	for _, bad := range []string{`0`, `-1`, `4294967296`, `"65001"`, `1.5`, `{}`} {
		var out holder
		if err := json.Unmarshal([]byte(`{"PeerAS":`+bad+`}`), &out); err == nil {
			t.Errorf("json.Unmarshal of PeerAS %s answered %#v, want an error", bad, out.PeerAS)
		}
	}
}
