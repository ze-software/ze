// Design: plan/spec-isis-3-l2-transport.md -- raw-socket doctor check tests

package transport

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestISISDoctorRawSocketUnavailable(t *testing.T) {
	// VALIDATES: AC-7 isis configured + no raw socket -> doctor-isis-raw-socket.
	// PREVENTS: IS-IS silently failing to send/receive because CAP_NET_RAW is
	// missing at startup.
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })

	tree := config.NewTree()
	tree.GetOrCreateContainer("isis")

	diags := checkISISRawSocket(diagnostic.DoctorCheckContext{Tree: tree})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	if diags[0].Code != "doctor-isis-raw-socket" {
		t.Errorf("code = %q, want doctor-isis-raw-socket", diags[0].Code)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("severity = %q, want warning", diags[0].Severity)
	}
}

func TestISISDoctorRawSocketAvailable(t *testing.T) {
	// VALIDATES: when the raw socket opens, no warning is emitted.
	old := rawSocketProbe
	rawSocketProbe = func() bool { return true }
	t.Cleanup(func() { rawSocketProbe = old })

	tree := config.NewTree()
	tree.GetOrCreateContainer("isis")
	if diags := checkISISRawSocket(diagnostic.DoctorCheckContext{Tree: tree}); len(diags) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(diags))
	}
}

func TestISISDoctorRawSocketAbsentConfig(t *testing.T) {
	// VALIDATES: the check fires only when isis is configured.
	// PREVENTS: doctor warning about IS-IS on boxes that do not use it.
	old := rawSocketProbe
	rawSocketProbe = func() bool { return false }
	t.Cleanup(func() { rawSocketProbe = old })

	if diags := checkISISRawSocket(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Errorf("empty tree: got %d diagnostics, want 0", len(diags))
	}
	if diags := checkISISRawSocket(diagnostic.DoctorCheckContext{Tree: nil}); len(diags) != 0 {
		t.Errorf("nil tree: got %d diagnostics, want 0", len(diags))
	}
}

func TestISISDoctorCheckRegistered(t *testing.T) {
	// VALIDATES: the transport registers the isis-raw-socket doctor check so
	// `ze doctor` runs it (and removing the package removes the check).
	if !slices.Contains(diagnostic.DoctorCheckNames(), "isis-raw-socket") {
		t.Fatal("doctor check isis-raw-socket not registered via diagnostic.RegisterDoctorCheck")
	}
}

func TestISISDoctorCodeRegistered(t *testing.T) {
	// VALIDATES: AC-7 the doctor-isis-raw-socket diagnostic code is explainable.
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup("doctor-isis-raw-socket")
	if meta == nil {
		t.Fatal("doctor-isis-raw-socket not registered in codes.go")
	}
	if meta.Title == "" || meta.Description == "" {
		t.Error("doctor-isis-raw-socket missing title/description")
	}
}
