// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS doctor check unit tests.
// Related: doctor.go -- the config-sanity check under test
// Related: register_doctor.go -- the init() registration of the check
//
// VALIDATES: the config-sanity check fires doctor-isis-net-missing when `isis`
// is configured with no net, doctor-isis-system-id-mismatch when an explicit
// system-id disagrees with the net, and NOTHING when IS-IS is absent or the
// config is clean (so a BGP-only node sees no spurious IS-IS warning, R-4); the
// isis-3 raw-socket check and its code are registered (surfaced) but not
// re-registered here (one code, one owner).
// PREVENTS: a false-positive IS-IS warning on a non-IS-IS node, a missed
// config error, and double registration of doctor-isis-raw-socket.

package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// treeFrom parses a config snippet into a *config.Tree (the same tree shape the
// doctor runner hands the check, side-effect-free).
func treeFrom(t *testing.T, conf string) *config.Tree {
	t.Helper()
	tree, err := config.ParseTreeForValidation(conf)
	if err != nil {
		t.Fatalf("ParseTreeForValidation: %v", err)
	}
	return tree
}

func codesOf(diags []diagnostic.Diagnostic) map[string]bool {
	out := make(map[string]bool, len(diags))
	for i := range diags {
		out[diags[i].Code] = true
	}
	return out
}

// TestISISDoctorConfigSanityNETMissing: an isis block with no net fires
// doctor-isis-net-missing.
func TestISISDoctorConfigSanityNETMissing(t *testing.T) {
	tree := treeFrom(t, "isis {\n\tlevel l1-l2\n}\n")
	diags := checkISISConfigSanity(diagnostic.DoctorCheckContext{Tree: tree})
	if !codesOf(diags)[codeNETMissing] {
		t.Errorf("expected %s, got %+v", codeNETMissing, diags)
	}
}

// TestISISDoctorConfigSanityMismatch: an explicit system-id that disagrees with
// the net fires doctor-isis-system-id-mismatch.
func TestISISDoctorConfigSanityMismatch(t *testing.T) {
	tree := treeFrom(t, "isis {\n\tnet 49.0001.0000.0000.0001.00\n\tsystem-id 0000.0000.0002\n}\n")
	diags := checkISISConfigSanity(diagnostic.DoctorCheckContext{Tree: tree})
	if !codesOf(diags)[codeSystemIDMismatch] {
		t.Errorf("expected %s, got %+v", codeSystemIDMismatch, diags)
	}
}

// TestISISDoctorConfigSanityClean: a well-formed isis config emits no
// config-sanity diagnostic.
func TestISISDoctorConfigSanityClean(t *testing.T) {
	tree := treeFrom(t, "isis {\n\tnet 49.0001.0000.0000.0001.00\n}\n")
	diags := checkISISConfigSanity(diagnostic.DoctorCheckContext{Tree: tree})
	if len(diags) != 0 {
		t.Errorf("clean config emitted %+v, want none", diags)
	}
}

// TestISISDoctorConfigSanityAbsent: with no isis container the check is a no-op
// (a node that does not run IS-IS gets no IS-IS warning, R-4).
func TestISISDoctorConfigSanityAbsent(t *testing.T) {
	// An empty config tree has no isis container.
	if d := checkISISConfigSanity(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(d) != 0 {
		t.Errorf("absent IS-IS emitted %+v, want none", d)
	}
	// A nil/wrong tree is also a no-op (never panics).
	if d := checkISISConfigSanity(diagnostic.DoctorCheckContext{Tree: nil}); d != nil {
		t.Errorf("nil tree emitted %+v, want nil", d)
	}
}

// TestISISDoctorChecksRegistered: the config-sanity check is registered with its
// two codes, and the isis-3 raw-socket check is also present (surfaced) -- but
// doctor-isis-raw-socket is owned by transport, so isis-13 must NOT re-register
// it (no duplicate check name, no duplicate code owner here).
func TestISISDoctorChecksRegistered(t *testing.T) {
	names := diagnostic.DoctorCheckNames()
	have := make(map[string]bool, len(names))
	for _, n := range names {
		have[n] = true
	}
	if !have["isis-config-sanity"] {
		t.Error("isis-config-sanity doctor check not registered")
	}
	if !have["isis-raw-socket"] {
		t.Error("isis-raw-socket doctor check (isis-3) not registered/surfaced")
	}
}

// TestISISRawSocketCodeRegistered: doctor-isis-raw-socket is a registered
// diagnostic code (owned by isis-3) so `ze explain` can describe it; isis-13
// surfaces it without re-registering.
func TestISISRawSocketCodeRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup("doctor-isis-raw-socket") == nil {
		t.Error("doctor-isis-raw-socket code not registered (isis-3 owns it)")
	}
	if diagnostic.Lookup(codeNETMissing) == nil {
		t.Errorf("%s code not registered", codeNETMissing)
	}
	if diagnostic.Lookup(codeSystemIDMismatch) == nil {
		t.Errorf("%s code not registered", codeSystemIDMismatch)
	}
}
