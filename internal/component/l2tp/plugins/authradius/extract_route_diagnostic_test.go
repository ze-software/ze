// Design: docs/architecture/doctor-and-health-checks.md -- recent events: a RADIUS framed route skipped
// Related: extract.go -- appendFramedRoutes logs the skip and calls reportFramedRouteSkipped
package l2tpauthradius

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// Goal: a framed route Ze skips is a problem ze doctor reports, naming the
// subscriber whose RADIUS profile carries it.
// Method: extract an Access-Accept holding one unparseable Framed-Route for a
// named subscriber, decode the warning line and read the error ring: the line
// carries the doctor code and the subscriber, and the event carries the code,
// the subscriber as subject, the attribute and parse error as message, and
// the profile fix. A second Accept for the same subscriber within the window
// raises no second event; another subscriber raises its own.
func TestFramedRouteSkippedCarriesCodeAndRaisesEventForSubscriber(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var logs bytes.Buffer
	previous := logger()
	loggerPtr.Store(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	const subscriber = "subscriber-a"
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedRoute, Value: radius.AttrString("not-a-prefix 0.0.0.0 1")},
	}}

	if routes := extractFramedRoutes(resp, subscriber); len(routes) != 0 {
		t.Fatalf("routes = %v, want none: the one value does not parse", routes)
	}

	var warning map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning); err != nil {
		t.Fatalf("decode the warning line: %v\n%s", err, logs.Bytes())
	}
	if warning["code"] != codeDoctorRADIUSFramedRouteSkipped {
		t.Fatalf("the warning carries code %v, want %s", warning["code"], codeDoctorRADIUSFramedRouteSkipped)
	}
	if warning["subscriber"] != subscriber {
		t.Fatalf("the warning names subscriber %v, want %s", warning["subscriber"], subscriber)
	}
	entries := report.Errors(0)
	if len(entries) != 1 {
		t.Fatalf("one skipped route retained %d events, want one: %+v", len(entries), entries)
	}
	entry := entries[0]
	if entry.Source != Name || entry.Code != codeDoctorRADIUSFramedRouteSkipped || entry.Subject != subscriber {
		t.Fatalf("the event lost its source, code or subscriber: %+v", entry)
	}
	if !strings.HasPrefix(entry.Message, "Framed-Route: ") || !strings.Contains(entry.Message, `prefix "not-a-prefix"`) {
		t.Fatalf("the event message does not name the attribute and the parse error: %q", entry.Message)
	}
	if entry.Fix != "correct the subscriber's RADIUS profile" {
		t.Fatalf("the event fix is %q, want the profile correction", entry.Fix)
	}
	// The binary entry point registers the built-in codes; a test does it itself.
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(entry.Code) == nil || !slices.Contains(diagnostic.EventCodes(), entry.Code) {
		t.Fatalf("the code is not described and declared to recent-events: %s", entry.Code)
	}

	extractFramedRoutes(resp, subscriber)
	if got := len(report.Errors(0)); got != 1 {
		t.Fatalf("a second Accept for the same subscriber within the window raised again: %d events", got)
	}
	extractFramedRoutes(resp, "subscriber-b")
	entries = report.Errors(0)
	if len(entries) != 2 || entries[0].Subject != "subscriber-b" {
		t.Fatalf("another subscriber did not raise its own event: %+v", entries)
	}
}
