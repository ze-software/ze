package config

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// TestExternalPluginNameRefusedAtCommit is AC-23's config half: LoadConfig,
// the walk the commit and `ze config validate` share, refuses a
// `bus endpoint` name the connect-back would refuse, and names the allowed
// form. Before the ze:validate on bus/endpoint/name, `a_b` loaded and the
// plugin hung at its connect-back. The validator itself is checked at the
// length boundary: 64 characters accepted, 0 and 65 refused.
func TestExternalPluginNameRefusedAtCommit(t *testing.T) {
	const src = `bus {
	endpoint a_b {
		external {
			run "/bin/true"
		}
	}
}
`
	_, err := LoadConfig(src, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted bus endpoint a_b, which the connect-back refuses")
	}
	for _, want := range []string{"bus", "a_b", ipc.PluginNameForm} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}

	validate := ExternalPluginNameValidator().ValidateFn
	long64 := strings.Repeat("a", 64)
	if err := validate("bus/endpoint/name", long64); err != nil {
		t.Errorf("a 64-character name is refused: %v", err)
	}
	for _, name := range []string{"", long64 + "a"} {
		if err := validate("bus/endpoint/name", name); err == nil {
			t.Errorf("name of %d characters accepted", len(name))
		}
	}
}
