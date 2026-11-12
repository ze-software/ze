// Design: docs/guide/netlab.md -- execute initial configuration inside the shell-equipped node.
package testnetlab

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestPackagedZeOverridesInheritedInitialNamespaceMode checks the real
// contributed definition after the production exporter wraps it for netlab.
// netlab26.10 linux.yml supplies initial=:ns, and add_default_config_mode
// leaves explicit inherited methods intact despite netlab_config_mode=sh.
// Authority: https://github.com/ipspace/netlab/blob/release_26.10/netsim/providers/__init__.py
// Docker exec dispatch: https://github.com/ipspace/netlab/blob/release_26.10/netsim/providers/clab/configs.py
func TestPackagedZeOverridesInheritedInitialNamespaceMode(t *testing.T) {
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
				NodeConfig map[string]string `yaml:"node_config"`
			} `yaml:"clab"`
		} `yaml:"daemons"`
	}
	if err := yaml.Unmarshal(body, &defaults); err != nil {
		t.Fatal(err)
	}
	ze, exists := defaults.Daemons["ze"]
	if !exists {
		t.Fatal("exported lab lost its Ze device definition")
	}
	if initial := ze.Clab.NodeConfig["initial"]; initial != ":sh" {
		t.Fatalf("packaged initial method=%q; want explicit :sh so netlab uses docker exec instead of inherited host ip netns exec", initial)
	}
}
