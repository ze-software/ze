// VALIDATES: `ze ospf` routes its closed member set (`decode`) to the offline
// codec, is discoverable with no args (R-6), and rejects unknown members with a
// hint instead of falling through to Run (ai/rules/cli.md closed set).
// PREVENTS: the split reintroducing a hyphenated `ospf-decode` root, or an
// unknown sub-token reaching the codec.

package cli

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func TestOSPFSubDispatch(t *testing.T) {
	if len(ospfMembers) != 1 || !ospfMembers["decode"] {
		t.Fatalf("ospfMembers must be the closed set {decode}, got %v", ospfMembers)
	}

	var buf bytes.Buffer
	ospfUsage(&buf)
	if !bytes.Contains(buf.Bytes(), []byte("decode")) {
		t.Errorf("ospf usage must enumerate members, got %q", buf.String())
	}

	// `decode` delegates to Run: Run's help path returns 0 without reading stdin,
	// so a 0 here proves the token routed into Run.
	if code := dispatchOSPF(nil, []string{"decode", "--help"}); code != 0 {
		t.Errorf("`ospf decode --help` = %d, want 0 (delegates to Run)", code)
	}

	if code := dispatchOSPF(nil, nil); code != 1 {
		t.Errorf("`ospf` (no args) = %d, want 1", code)
	}
	if code := dispatchOSPF(nil, []string{"bogus"}); code != 1 {
		t.Errorf("`ospf bogus` = %d, want 1", code)
	}
	if code := dispatchOSPF(nil, []string{"-h"}); code != 0 {
		t.Errorf("`ospf -h` = %d, want 0", code)
	}

	if registry.LookupRoot("ospf") == nil {
		t.Error("root `ospf` must be registered")
	}
	if registry.LookupRoot("ospf-decode") != nil {
		t.Error("old root `ospf-decode` must be gone")
	}
}
