package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGokBuildReadsMutableRuntimeSources requires Go to scan the runtime source
// tree rather than reuse an immutable module index after a tracked patch changes.
func TestGokBuildReadsMutableRuntimeSources(t *testing.T) {
	for _, settings := range []string{"", "netdns=go", "goindex=1,netdns=go", "goindex=0,netdns=go,goindex=1"} {
		t.Run(settings, func(t *testing.T) {
			underPatchedTree(t, patchedTreeFixture(t, true))
			t.Setenv("GOMODCACHE", "")
			t.Setenv("GOFLAGS", "")
			t.Setenv("GOPROXY", "off")
			t.Setenv("GODEBUG", settings)
			if err := SetGokGoEnv(filepath.Join(t.TempDir(), "modcache")); err != nil {
				t.Fatal(err)
			}
			last := "unset"
			for setting := range strings.SplitSeq(os.Getenv("GODEBUG"), ",") {
				if value, ok := strings.CutPrefix(setting, "goindex="); ok {
					last = value
				}
			}
			if last != "0" {
				t.Errorf("mutable runtime source keeps goindex=%s", last)
			}
			if strings.Contains(settings, "netdns=go") {
				if !strings.Contains(os.Getenv("GODEBUG"), "netdns=go") {
					t.Error("setting the module index discarded the existing resolver setting")
				}
			}
		})
	}
}
