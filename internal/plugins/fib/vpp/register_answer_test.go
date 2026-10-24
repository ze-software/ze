// Design: docs/research/vpp-deployment-reference.md -- VPP FIB route programming
// Related: register.go -- openFibVPPWriter, answerShowFibVPP

package fibvpp

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// TestOpenFibVPPWriterRefusesWithoutVPP pins the refusal when the vpp
// component is not running. The writer used to fall back to the mockBackend
// test double, which "installed" every route into memory while VPP held none.
// Method: open a writer with no connector.
//
// VALIDATES: no connector answers no writer and an error naming the fix.
// PREVENTS: a writer that programs nothing and reports routes as installed.
func TestOpenFibVPPWriterRefusesWithoutVPP(t *testing.T) {
	f, err := openFibVPPWriter(nil, 0, nil)
	if f != nil {
		t.Fatal("openFibVPPWriter answered a writer with no VPP connection")
	}
	if !errors.Is(err, errFibVPPNoConnector) {
		t.Fatalf("error = %v, want errFibVPPNoConnector", err)
	}
	if !strings.Contains(err.Error(), "ze doctor") {
		t.Errorf("error %q names no command that fixes it", err)
	}
}

// TestShowFibVPPRefusesBeforeStart pins `show fib vpp` before OnStarted: an
// error, never "[]", which read as "VPP holds no Ze route" and marshaled to a
// JSON string scalar.
func TestShowFibVPPRefusesBeforeStart(t *testing.T) {
	answer, err := answerShowFibVPP(nil, nil)
	if !errors.Is(err, errFibVPPNotStarted) {
		t.Fatalf("error = %v, want errFibVPPNotStarted", err)
	}
	if answer != nil {
		t.Errorf("answer = %v beside the refusal", answer)
	}
}

// TestShowFibVPPAnswersWhyVPPIsUnreachable pins `show fib vpp` after OnStarted
// found no VPP: the answer carries the cause, never an empty route list.
func TestShowFibVPPAnswersWhyVPPIsUnreachable(t *testing.T) {
	_, err := answerShowFibVPP(nil, errFibVPPNoConnector)
	if !errors.Is(err, errFibVPPNoConnector) {
		t.Fatalf("error = %v, want it to wrap errFibVPPNoConnector", err)
	}
}

// TestShowFibVPPAnswersInstalledRoutesAsAList pins the answer of a running
// writer: a JSON array of the installed routes.
func TestShowFibVPPAnswersInstalledRoutesAsAList(t *testing.T) {
	f := newFibVPP(&mockBackend{})
	f.installed[srv6RouteKey{prefix: netip.MustParsePrefix("10.0.0.0/24")}] = installedRoute{nextHop: "192.0.2.1"}
	answer, err := answerShowFibVPP(f, nil)
	if err != nil {
		t.Fatalf("answerShowFibVPP: %v", err)
	}
	wire, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var routes []map[string]any
	if err := json.Unmarshal(wire, &routes); err != nil {
		t.Fatalf("answer %s is not a JSON list: %v", wire, err)
	}
	if len(routes) != 1 || routes[0]["prefix"] != "10.0.0.0/24" {
		t.Errorf("answer %s, want the one installed route", wire)
	}
}
