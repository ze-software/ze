// Design: docs/architecture/api/process-protocol.md -- bounded state requests.
package rpc_test

import (
	"strconv"
	"strings"
	"testing"

	ipcyang "github.com/ze-software/ze/internal/core/ipc/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestStateDataYANGLengthIsTheLargestBound proves the YANG model of a state
// value admits exactly the largest bound a key registration declares, so the
// schema and the Go bound cannot drift apart.
//
// Method: find the state-data grouping in ze-plugin-engine.yang, read the upper
// end of its binary length, and compare it with rpc.StateDataLargest.
func TestStateDataYANGLengthIsTheLargestBound(t *testing.T) {
	module := ipcyang.ZePluginEngineYANG
	_, grouping, found := strings.Cut(module, "grouping state-data {")
	if !found {
		t.Fatal("ze-plugin-engine.yang has no state-data grouping")
	}
	_, length, found := strings.Cut(grouping, `type binary { length "0..`)
	if !found {
		t.Fatal("state-data grouping has no binary length")
	}
	upper, _, found := strings.Cut(length, `"`)
	if !found {
		t.Fatal("state-data binary length is not closed")
	}
	modeled, err := strconv.Atoi(upper)
	if err != nil {
		t.Fatalf("state-data length upper bound %q: %v", upper, err)
	}
	if modeled != rpc.StateDataLargest() {
		t.Fatalf("ze-plugin-engine.yang state-data length is 0..%d, want 0..%d (rpc.StateDataLargest)", modeled, rpc.StateDataLargest())
	}
}
