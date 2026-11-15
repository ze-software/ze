// Design: docs/architecture/core-design.md -- raw writes for kernel controls and external artifacts
// Related: fspersistence.go -- exact file exemptions, separate from live-store rules

package archfspersistence

import "testing"

const (
	espFormSysctlArtifact     = "internal/component/ike/dataplane/espform_tun_linux.go"
	agentHookSettingsArtifact = "internal/plugins/skills/agent_hook.go"
)

// TestArtifactExemptionsStayFileScoped exercises both permitted artifact owners
// beside the same raw-write operations in sibling files. The owners are exempt;
// sharing their package never permits another file to persist daemon state.
func TestArtifactExemptionsStayFileScoped(t *testing.T) {
	const source = `package fixture
import "os"
func open(path string) (*os.File, error) { return os.OpenFile(path, os.O_WRONLY, 0) }
func replace(source, target string) error { return os.Rename(source, target) }
`
	for _, item := range []struct {
		name     string
		artifact string
		sibling  string
	}{
		{"kernel-controls", espFormSysctlArtifact, "internal/component/ike/dataplane/state_linux.go"},
		{"harness-settings", agentHookSettingsArtifact, "internal/plugins/skills/state.go"},
	} {
		t.Run(item.name, func(t *testing.T) {
			tree := writeTree(t, map[string]string{item.artifact: source, item.sibling: source})
			findings, err := Check(tree, 0)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{"OpenFile": true, "Rename": true}
			if len(findings) != len(want) {
				t.Errorf("got %d findings, want only the %d sibling writes: %+v", len(findings), len(want), findings)
			}
			for _, finding := range findings {
				if finding.File != item.sibling {
					t.Errorf("artifact owner %s was refused: %+v", item.artifact, finding)
					continue
				}
				if !want[finding.Fn] {
					t.Errorf("unexpected sibling write: %+v", finding)
				}
				delete(want, finding.Fn)
			}
			if len(want) != 0 {
				t.Errorf("sibling writes escaped the guard: %v", want)
			}
		})
	}
}

// TestRawArtifactExemptionsKeepLiveStoresGuarded opens a blob at each raw-write
// owner's exact path. Neither legitimate artifact permits bypassing Storage.
func TestRawArtifactExemptionsKeepLiveStoresGuarded(t *testing.T) {
	const source = `package fixture
import "github.com/ze-software/ze/pkg/zefs"
var open = zefs.Open
`
	for _, path := range []string{espFormSysctlArtifact, agentHookSettingsArtifact} {
		t.Run(path, func(t *testing.T) {
			tree := writeTree(t, map[string]string{path: source})
			findings, err := Check(tree, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 {
				t.Fatalf("got %d live-store findings, want 1: %+v", len(findings), findings)
			}
			finding := findings[0]
			if finding.File != path {
				t.Errorf("live-store finding names %s, want %s", finding.File, path)
			}
			if finding.Pkg != zefsPkg {
				t.Errorf("live-store finding names package %s, want %s", finding.Pkg, zefsPkg)
			}
			if finding.Fn != "Open" {
				t.Errorf("live-store finding names %s, want Open", finding.Fn)
			}
		})
	}
}
