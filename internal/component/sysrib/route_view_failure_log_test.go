// Design: docs/architecture/rib/unified-locrib.md -- routing-view diagnostics.
// Related: register_route_view.go -- the failure producer supplies its doctor code.

package sysrib

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

func TestRouteViewFailureLogCarriesDeclaredDoctorCode(t *testing.T) {
	owner := routeViewOwner()
	cause := errors.New("controlled output cleanup failure")
	fix := "ze request plugin restart " + owner.Name()
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "plain", err: cause},
		{name: "operator", err: opererr.Wrap(cause, "System routing output cleanup failed", "The previous output is still owned", fix)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var capture bytes.Buffer
			previous := logger()
			setLogger(slog.New(slog.NewJSONHandler(&capture, nil)))
			t.Cleanup(func() { setLogger(previous) })
			reportRouteViewFailure(owner, test.err)
			var record struct {
				Level  string `json:"level"`
				Code   string `json:"code"`
				Plugin string `json:"plugin"`
				Why    string `json:"why"`
				Fix    string `json:"fix"`
				Error  string `json:"error"`
			}
			if err := json.Unmarshal(capture.Bytes(), &record); err != nil {
				t.Fatalf("failure log is not one JSON record: %v; captured %q", err, capture.String())
			}
			if record.Level != "ERROR" {
				t.Errorf("failure level %q, want ERROR", record.Level)
			}
			if record.Code != routeViewDoctorCode {
				t.Errorf("failure doctor code %q, want %q", record.Code, routeViewDoctorCode)
			}
			declared := false
			for _, check := range routeViewDoctorDeclarations() {
				if slices.Contains(check.Codes, record.Code) {
					declared = true
				}
			}
			if !declared {
				t.Errorf("failure doctor code %q has no routing-view declaration", record.Code)
			}
			if record.Plugin != owner.Name() {
				t.Errorf("failure plugin %q, want %q", record.Plugin, owner.Name())
			}
			if record.Why == "" {
				t.Error("failure omitted its operator explanation")
			}
			if record.Fix != fix {
				t.Errorf("failure fix %q, want %q", record.Fix, fix)
			}
			if record.Error != cause.Error() {
				t.Errorf("failure cause %q, want %q", record.Error, cause.Error())
			}
		})
	}
}
