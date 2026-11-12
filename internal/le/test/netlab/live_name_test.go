// Design: docs/guide/netlab.md -- generated topology names satisfy upstream identifiers.
package testnetlab

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLiveTopologyNamesAreValidAndIsolated reads the actual generated YAML
// from long, hyphenated work paths and checks independent names for each run.
func TestLiveTopologyNamesAreValidAndIsolated(t *testing.T) {
	identifier := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,15}$`)
	names := make(map[string]bool)
	for range 64 {
		directory := filepath.Join(t.TempDir(), "netlab-live-3886936784", "lab")
		path := filepath.Join(directory, "topology.yml")
		writeFixtureFile(t, path, "nodes:\n  r1: {}\n", 0o644)
		if err := writeLiveTopologyName(path); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var topology struct {
			Name  string         `yaml:"name"`
			Nodes map[string]any `yaml:"nodes"`
		}
		if err := yaml.Unmarshal(body, &topology); err != nil {
			t.Fatal(err)
		}
		if len(topology.Name) != 16 || !identifier.MatchString(topology.Name) || !strings.HasPrefix(topology.Name, "ze") {
			t.Fatalf("upstream rejects generated topology identifier %q", topology.Name)
		}
		if names[topology.Name] {
			t.Fatalf("independent live runs share topology identity %q", topology.Name)
		}
		names[topology.Name] = true
		if _, exists := topology.Nodes["r1"]; !exists {
			t.Fatal("naming removed the packaged topology")
		}
	}
}
