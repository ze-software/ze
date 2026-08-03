// VALIDATES: `ze isis` routes its closed member set (`decode`) to the offline
// codec, is discoverable with no args (R-6), and rejects unknown members with a
// hint instead of falling through to Run (ai/rules/cli.md closed set).
// PREVENTS: the split reintroducing a hyphenated `isis-decode` root, or an
// unknown sub-token reaching the codec.

package cli

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func TestISISSubDispatch(t *testing.T) {
	if len(isisMembers) != 1 || !isisMembers["decode"] {
		t.Fatalf("isisMembers must be the closed set {decode}, got %v", isisMembers)
	}

	var buf bytes.Buffer
	isisUsage(&buf)
	if !bytes.Contains(buf.Bytes(), []byte("decode")) {
		t.Errorf("isis usage must enumerate members, got %q", buf.String())
	}

	// `decode` delegates to Run: Run's help path returns 0 without reading stdin,
	// so a 0 here proves the token routed into Run.
	if code := dispatchISIS(nil, []string{"decode", "--help"}); code != 0 {
		t.Errorf("`isis decode --help` = %d, want 0 (delegates to Run)", code)
	}

	if code := dispatchISIS(nil, nil); code != 1 {
		t.Errorf("`isis` (no args) = %d, want 1", code)
	}
	if code := dispatchISIS(nil, []string{"bogus"}); code != 1 {
		t.Errorf("`isis bogus` = %d, want 1", code)
	}
	if code := dispatchISIS(nil, []string{"-h"}); code != 0 {
		t.Errorf("`isis -h` = %d, want 0", code)
	}

	if registry.LookupRoot("isis") == nil {
		t.Error("root `isis` must be registered")
	}
	if registry.LookupRoot("isis-decode") != nil {
		t.Error("old root `isis-decode` must be gone")
	}
}
