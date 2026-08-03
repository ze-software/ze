// VALIDATES: `ze traffic` routes its closed member set (`control`) to the tc/VPP
// tool, is discoverable with no args (R-6), and rejects unknown members with a
// hint instead of falling through to Run (ai/rules/cli.md closed set).
// PREVENTS: the split reintroducing a hyphenated `traffic-control` root, or an
// unknown sub-token reaching the tool's arg parser.

package cli

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func TestTrafficSubDispatch(t *testing.T) {
	// control is the only member; the closed set is what keeps an unknown token
	// out of Run's arg parser.
	if len(trafficMembers) != 1 || !trafficMembers["control"] {
		t.Fatalf("trafficMembers must be the closed set {control}, got %v", trafficMembers)
	}

	// The usage/hint enumerates the members so the namespace is discoverable
	// (R-6) and an unknown member gets a keyword hint.
	var buf bytes.Buffer
	trafficUsage(&buf)
	if !bytes.Contains(buf.Bytes(), []byte("control")) {
		t.Errorf("traffic usage must enumerate members, got %q", buf.String())
	}

	// `control` delegates to Run: Run's help path returns 0 without loading the tc
	// backend, so a 0 here proves the token routed into Run.
	if code := dispatchTraffic(nil, []string{"control", "--help"}); code != 0 {
		t.Errorf("`traffic control --help` = %d, want 0 (delegates to Run)", code)
	}

	// Bare `ze traffic` and an unknown member both exit non-zero; the unknown
	// member is rejected by the closed set above, never reaching Run.
	if code := dispatchTraffic(nil, nil); code != 1 {
		t.Errorf("`traffic` (no args) = %d, want 1", code)
	}
	if code := dispatchTraffic(nil, []string{"bogus"}); code != 1 {
		t.Errorf("`traffic bogus` = %d, want 1", code)
	}

	// A help flag prints usage and exits 0.
	if code := dispatchTraffic(nil, []string{"-h"}); code != 0 {
		t.Errorf("`traffic -h` = %d, want 0", code)
	}

	// The root is registered under the split name, not the old hyphen.
	if registry.LookupRoot("traffic") == nil {
		t.Error("root `traffic` must be registered")
	}
	if registry.LookupRoot("traffic-control") != nil {
		t.Error("old root `traffic-control` must be gone")
	}
}
