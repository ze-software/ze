// Design: docs/architecture/container/runtime.md -- the doctor codes the container plugin owns

package container

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestContainerDiagnosticCodesRegisterFromThePlugin proves every code of the
// plugin's problem-class table resolves in the code registry once the plugin
// is linked, with a title and an explanation, and that each carries the
// prefix every container code shares.
//
// Method: the plugin's init() ran when this package was loaded, so the test
// looks each code up in the registry the doctor reads.
//
// MUTATION: remove the registerContainerDiagnosticCodes call from
// registerContainer. Every lookup then answers nil.
func TestContainerDiagnosticCodesRegisterFromThePlugin(t *testing.T) {
	owned := []string{
		codeNotPrivileged,
		codeCgroupUnavailable,
		codeSealCopiesBinary,
		codeOutOfMemory,
		codeProcessLimit,
		codeStopKilled,
		codeCleanupIncomplete,
	}
	if len(containerDiagnosticCodes) != len(owned) {
		t.Fatalf("the code table holds %d codes, want the %d of the problem-class table", len(containerDiagnosticCodes), len(owned))
	}
	for _, code := range owned {
		meta := diagnostic.Lookup(code)
		if meta == nil {
			t.Errorf("code %s is not in the code registry", code)
			continue
		}
		if !strings.HasPrefix(meta.Code, "doctor-container-") {
			t.Errorf("code %s does not start with doctor-container-", meta.Code)
		}
		if meta.Title == "" {
			t.Errorf("code %s has no title", code)
		}
		if meta.Description == "" {
			t.Errorf("code %s has no explanation", code)
		}
	}
}

// TestHostCodesAreMarkedAsHostVerdicts proves the three codes that judge the
// host, never the configuration, say so: a tool that validates a config
// written for another machine reads the mark to tell the two apart.
func TestHostCodesAreMarkedAsHostVerdicts(t *testing.T) {
	host := map[string]bool{
		codeNotPrivileged:     true,
		codeCgroupUnavailable: true,
		codeSealCopiesBinary:  true,
	}
	for _, meta := range containerDiagnosticCodes {
		if meta.HostVerdict != host[meta.Code] {
			t.Errorf("code %s has HostVerdict %v, want %v", meta.Code, meta.HostVerdict, host[meta.Code])
		}
	}
}
