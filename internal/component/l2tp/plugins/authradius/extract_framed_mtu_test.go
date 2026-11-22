package l2tpauthradius

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// framedMTUAttr is a Framed-MTU attribute carrying value as four octets.
func framedMTUAttr(value uint32) radius.Attr {
	raw := make([]byte, 4)
	binary.BigEndian.PutUint32(raw, value)
	return radius.Attr{Type: radius.AttrFramedMTU, Value: raw}
}

// TestExtractFramedMTU proves an Access-Accept's Framed-MTU reaches the
// session metadata, and that an Accept without one leaves the field unset
// rather than inventing a value. Method: extractAuthMetadata over an Accept
// with 1400, one with 1400 beside another attribute, and one with none.
func TestExtractFramedMTU(t *testing.T) {
	present := &radius.Packet{Attrs: []radius.Attr{framedMTUAttr(1400)}}
	meta := extractAuthMetadata(present, "subscriber-a")
	if meta == nil {
		t.Fatal("an Accept carrying only Framed-MTU 1400 produced no metadata")
	}
	if meta.FramedMTU != 1400 {
		t.Fatalf("FramedMTU = %d, want 1400", meta.FramedMTU)
	}

	absent := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedPool, Value: radius.AttrString("gold")},
	}}
	meta = extractAuthMetadata(absent, "subscriber-a")
	if meta == nil {
		t.Fatal("an Accept carrying Framed-Pool produced no metadata")
	}
	if meta.FramedMTU != 0 {
		t.Fatalf("FramedMTU = %d for an Accept without Framed-MTU, want 0 (none)", meta.FramedMTU)
	}
}

// TestExtractFramedMTURange proves each edge of the accepted range: 68 and
// 65535 pass unchanged, 64 to 67 are raised to the IPv4 minimum 68, and a
// value outside the RFC 2865 range or of the wrong length is ignored, so the
// interface MTU then comes from PPP alone. Method: extractFramedMTU per case.
func TestExtractFramedMTURange(t *testing.T) {
	cases := []struct {
		name  string
		attr  radius.Attr
		want  uint16
		apply bool
	}{
		{"IPv4 minimum kept", framedMTUAttr(68), 68, true},
		{"RFC 2865 maximum kept", framedMTUAttr(65535), 65535, true},
		{"RFC 2865 minimum raised to 68", framedMTUAttr(64), 68, true},
		{"just below IPv4 minimum raised to 68", framedMTUAttr(67), 68, true},
		{"below RFC 2865 minimum ignored", framedMTUAttr(63), 0, false},
		{"zero ignored", framedMTUAttr(0), 0, false},
		{"above RFC 2865 maximum ignored", framedMTUAttr(65536), 0, false},
		{"two octets ignored", radius.Attr{Type: radius.AttrFramedMTU, Value: []byte{0x05, 0x78}}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			resp := &radius.Packet{Attrs: []radius.Attr{tc.attr}}
			got, ok := extractFramedMTU(resp, "subscriber-a")
			if ok != tc.apply {
				t.Fatalf("applied = %v, want %v", ok, tc.apply)
			}
			if got != tc.want {
				t.Fatalf("Framed-MTU = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestFramedMTUAdjustedCarriesCodeAndRaisesEventForSubscriber proves a
// Framed-MTU Ze does not apply as sent is never silent: the WARN line
// carries the doctor code and the subscriber, the report event names the
// subscriber, the value and what Ze did, with the profile fix, and the code
// is described and declared to recent-events. A value Ze applies unchanged
// raises nothing. Method: extractFramedMTU on 66, then on 1400.
func TestFramedMTUAdjustedCarriesCodeAndRaisesEventForSubscriber(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var logs bytes.Buffer
	previous := logger()
	loggerPtr.Store(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	const subscriber = "subscriber-a"

	if got, ok := extractFramedMTU(&radius.Packet{Attrs: []radius.Attr{framedMTUAttr(66)}}, subscriber); !ok || got != 68 {
		t.Fatalf("Framed-MTU 66 gave (%d, %v), want (68, true)", got, ok)
	}

	var warning map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning); err != nil {
		t.Fatalf("decode the warning line: %v\n%s", err, logs.Bytes())
	}
	if warning["code"] != ppp.CodeDoctorRADIUSFramedMTU {
		t.Fatalf("the warning carries code %v, want %s", warning["code"], ppp.CodeDoctorRADIUSFramedMTU)
	}
	if warning["subscriber"] != subscriber {
		t.Fatalf("the warning names subscriber %v, want %s", warning["subscriber"], subscriber)
	}
	entries := report.Errors(0)
	if len(entries) != 1 {
		t.Fatalf("one raised Framed-MTU retained %d events, want one: %+v", len(entries), entries)
	}
	entry := entries[0]
	if entry.Source != Name || entry.Code != ppp.CodeDoctorRADIUSFramedMTU || entry.Subject != subscriber {
		t.Fatalf("the event lost its source, code or subscriber: %+v", entry)
	}
	if !strings.Contains(entry.Message, "Framed-MTU 66") || !strings.Contains(entry.Message, "raised to 68") {
		t.Fatalf("the event message does not name the value and what Ze did: %q", entry.Message)
	}
	if entry.Fix != "correct the subscriber's RADIUS profile" {
		t.Fatalf("the event fix is %q, want the profile correction", entry.Fix)
	}
	// The binary entry point registers the built-in codes; a test does it itself.
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(entry.Code) == nil || !slices.Contains(diagnostic.EventCodes(), entry.Code) {
		t.Fatalf("the code is not described and declared to recent-events: %s", entry.Code)
	}

	logs.Reset()
	report.ResetForTest()
	if got, ok := extractFramedMTU(&radius.Packet{Attrs: []radius.Attr{framedMTUAttr(1400)}}, "subscriber-b"); !ok || got != 1400 {
		t.Fatalf("Framed-MTU 1400 gave (%d, %v), want (1400, true)", got, ok)
	}
	if logs.Len() != 0 || len(report.Errors(0)) != 0 {
		t.Fatalf("a Framed-MTU applied as sent logged or raised: %s %+v", logs.Bytes(), report.Errors(0))
	}
}
