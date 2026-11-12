// Design: docs/architecture/api/process-protocol.md -- the report-issue bounds the YANG declares.
//
// This test lives beside the loader rather than beside the IPC modules
// because internal/core is the leaf tier and does not import a component.

package yang_test

import (
	"strconv"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/core/ipc/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReportIssueYANGLengthsMatchTheBus proves the length restriction each
// string leaf of the report-issue rpc declares is the bound the SDK and the
// engine enforce, which pkg/plugin/rpc takes from the report bus's own
// declaration (internal/core/report). YANG cannot import a Go constant, so
// this comparison is what keeps the copy honest. Method: load the module
// through the schema loader and read each leaf's length from the parsed
// rpc, never from the file's text.
//
// PREVENTS: a YANG that advertises a bound the daemon does not enforce.
func TestReportIssueYANGLengthsMatchTheBus(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("load YANG: %v", err)
	}
	mod := loader.GetModule("ze-plugin-engine")
	if mod == nil {
		t.Fatal("module ze-plugin-engine is not loaded")
	}
	want := map[string]string{
		"code":    "1.." + strconv.Itoa(rpc.ReportCodeMax),
		"subject": "1.." + strconv.Itoa(rpc.ReportSubjectMax),
		"message": "1.." + strconv.Itoa(rpc.ReportMessageMax),
		"fix":     "0.." + strconv.Itoa(rpc.ReportFixMax),
	}
	for _, r := range mod.RPC {
		if r.Name != "report-issue" {
			continue
		}
		if r.Input == nil {
			t.Fatal("report-issue declares no input")
		}
		got := map[string]string{}
		for _, leaf := range r.Input.Leaf {
			if leaf.Type != nil && leaf.Type.Length != nil {
				got[leaf.Name] = leaf.Type.Length.Name
			}
		}
		for name, length := range want {
			if got[name] != length {
				t.Errorf("report-issue leaf %s declares length %q, the bus enforces %q", name, got[name], length)
			}
		}
		return
	}
	t.Fatal("ze-plugin-engine declares no report-issue rpc")
}
