// Design: docs/architecture/isis/isis-3-l2-transport.md -- raw-socket doctor check tests

package transport

import (
	"errors"
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
	rawSocketProbe = func() error { return errProbeRefused }
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
	rawSocketProbe = func() error { return nil }
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
	rawSocketProbe = func() error { return errProbeRefused }
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

// errProbeRefused stands in for any reason the platform probe gives.
var errProbeRefused = errors.New("probe refused")

// TestISISDoctorRawSocketCarriesProbeReason proves the warning carries the reason the probe
// gives, so a host that is not Linux reads that the transport is Linux-only
// instead of a CAP_NET_RAW hint that cannot apply there.
//
// VALIDATES: the diagnostic message is the probe's own reason, word for word.
// PREVENTS: a fixed capability message hiding why the transport cannot run.
//
// Method: swap the rawSocketProbe seam for one refusing with a known reason.
func TestISISDoctorRawSocketCarriesProbeReason(t *testing.T) {
	const reason = "the IS-IS raw transport is Linux-only, and this host runs plan9"
	old := rawSocketProbe
	rawSocketProbe = func() error { return errors.New(reason) }
	t.Cleanup(func() { rawSocketProbe = old })
	tree := config.NewTree()
	tree.GetOrCreateContainer("isis")

	diags := checkISISRawSocket(diagnostic.DoctorCheckContext{Tree: tree})
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	if diags[0].Message != reason {
		t.Errorf("message = %q, want the probe's reason %q", diags[0].Message, reason)
	}
}
