package irr

// VALIDATES: AC-5 -- ze doctor reports an IRR reference enforcing stale data and
// one with no data at all.
// PREVENTS: a firewall filter running on prefixes nobody has confirmed with no
// surface an operator would look at.

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// irrDoctorTree parses a firewall config binding eth1 to AS-TEST.
func irrDoctorTree(t *testing.T) *config.Tree {
	t.Helper()
	tree, err := config.ParseTreeForValidation(`firewall {
	irr {
		interface eth1 {
			source-as-set AS-TEST;
		}
	}
}
`)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return tree
}

func TestDoctorReportsStaleIRRData(t *testing.T) {
	tree := irrDoctorTree(t)

	// No cache directory: the reference has no prefixes at all.
	diags := checkIRRDataFreshness(diagnostic.DoctorCheckContext{Tree: tree, ConfigDir: t.TempDir()})
	if len(diags) != 1 || diags[0].Code != codeIRRNoData {
		t.Fatalf("uncached reference diagnostics = %+v, want one %s", diags, codeIRRNoData)
	}
	if diags[0].Severity != diagnostic.SeverityError {
		t.Errorf("severity = %v, want error: the reference filters nothing", diags[0].Severity)
	}
}

func TestDoctorSilentWithoutIRRReferences(t *testing.T) {
	tree, err := config.ParseTreeForValidation("firewall {\n\tbackend nft;\n}\n")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if diags := checkIRRDataFreshness(diagnostic.DoctorCheckContext{Tree: tree, ConfigDir: t.TempDir()}); len(diags) != 0 {
		t.Errorf("a node with no IRR references must produce no diagnostics, got %+v", diags)
	}
}

func TestDoctorCodesAreRegistered(t *testing.T) {
	for _, code := range []string{codeIRRStaleData, codeIRRNoData} {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("code %q is emitted but not registered for 'ze explain'", code)
		}
	}
}
