// Design: docs/guide/netlab.md -- start Ze after netlab assigns node addresses.
package testnetlab

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// VALIDATES: a packaged netlab node waits for successful interface configuration
// before its root process execs Ze, preserving the daemon's exit status and logs.
// Authority: netlab release_26.10 templates/provider/clab/dataplane-wait.j2 and
// ansible/templates/initial/linux/vanilla.j2.
func TestPackagedZeStartsAfterInitialInterfaceConfiguration(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	directory := t.TempDir()
	if err := newChecker(root).buildLab(directory); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(directory, "topology-defaults.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var defaults struct {
		Daemons map[string]struct {
			Clab struct {
				Node struct {
					Command   string            `yaml:"cmd"`
					Templates map[string]string `yaml:"config_templates"`
				} `yaml:"node"`
				Variables struct {
					Modules []string `yaml:"netlab_dp_module"`
					Start   string   `yaml:"netlab_start_daemon"`
				} `yaml:"group_vars"`
			} `yaml:"clab"`
		} `yaml:"daemons"`
	}
	if err := yaml.Unmarshal(body, &defaults); err != nil {
		t.Fatal(err)
	}
	ze := defaults.Daemons["ze"].Clab
	if ze.Node.Command != "/etc/dataplane-wait.sh" {
		t.Errorf("node command %q starts before netlab assigns its local addresses", ze.Node.Command)
	}
	if ze.Node.Templates["dataplane-wait"] != "/etc/dataplane-wait.sh:sh" {
		t.Error("node does not bind netlab's data-plane startup wrapper")
	}
	if !reflect.DeepEqual(ze.Variables.Modules, []string{"initial"}) {
		t.Errorf("startup waits for %v, want initial interface configuration", ze.Variables.Modules)
	}
	if ze.Variables.Start != "/usr/local/bin/ze start /etc/ze/ze.conf" {
		t.Errorf("startup wrapper lost the Ze daemon command: %q", ze.Variables.Start)
	}
	initial, err := os.ReadFile(filepath.Join(directory, "templates", "ze", "initial.j2"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(initial), "#!/bin/sh\n") {
		t.Error("initial template must execute as a shell script inside the node")
	}
	stopOnFailure := strings.Index(string(initial), "\nset -e\n")
	setup := strings.Index(string(initial), "include 'initial/linux/vanilla.j2'")
	ready := strings.Index(string(initial), "touch /var/run/initial.done")
	if stopOnFailure < 0 || setup <= stopOnFailure {
		t.Errorf("initial template must stop on failure before Linux interface setup: %s", initial)
	}
	if setup < 0 || ready <= setup {
		t.Fatalf("initial template must complete Linux interface setup before signaling startup: %s", initial)
	}
}
