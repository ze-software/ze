// Design: website/AI.md -- configuration reference data comes from the live Ze schema
package site

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/featuretags"
)

const schemaExtractionTimeout = 5 * time.Minute

// daemonCoreTag is the build tag the shipped daemon always carries, beside
// every feature gate feature-gates.txt declares.
const daemonCoreTag = "ze_core"

type yangNode struct {
	Name string `json:"name"`
}

// liveYANGConfigTree answers `show yang tree --config | json` for one checkout.
// It is a variable so a test can state a tree rather than compile the daemon.
var liveYANGConfigTree = runYANGConfigTree

// runYANGConfigTree runs the daemon's typed schema command and answers its JSON.
//
// A named binary is run as it stands. With no binary named, the command is
// built from THIS checkout's sources, with the feature gates the shipped daemon
// carries, because a bin/ze somebody left behind was built at an unknown commit
// with unknown gates and would publish a configuration reference for a daemon
// nobody ships. That is the staleness the site build exists to remove.
func runYANGConfigTree(repository, binary string) ([]byte, error) {
	name, args := binary, []string{"cli", "-c", "show yang tree --config | json"}
	if binary == "" {
		tags, err := featuretags.DaemonTags(repository)
		if err != nil {
			return nil, err
		}
		name = "go"
		args = append([]string{"run", "-tags",
			strings.Join(append([]string{daemonCoreTag}, tags...), ","), "./cmd/ze"}, args...)
	} else if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = fmt.Errorf("not a regular file")
		}
		return nil, fmt.Errorf("ze binary %s: %w", binary, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), schemaExtractionTimeout)
	defer cancel()
	// #nosec G204 -- the argv is fixed apart from the build tags, which are read
	// from the checkout's own feature-gates.txt.
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = repository
	var stderr bytes.Buffer
	command.Stderr = &stderr
	raw, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s show yang tree --config | json: %w: %s", name, err, stderr.String())
	}
	return raw, nil
}

// extractYANGConfigTree writes the top-level-name-indexed configuration tree
// the site publishes, and answers how many roots it holds.
func extractYANGConfigTree(repository, output, binary string) (int, error) {
	raw, err := liveYANGConfigTree(repository, binary)
	if err != nil {
		return 0, err
	}
	return writeYANGConfigTree(output, raw)
}

func writeYANGConfigTree(output string, raw []byte) (int, error) {
	var nodes []json.RawMessage
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return 0, fmt.Errorf("parse configuration tree: %w", err)
	}
	indexed := make(map[string]json.RawMessage, len(nodes))
	for _, rawNode := range nodes {
		var identity yangNode
		if err := json.Unmarshal(rawNode, &identity); err != nil {
			return 0, fmt.Errorf("parse configuration root: %w", err)
		}
		if identity.Name == "" {
			return 0, fmt.Errorf("configuration root has no name")
		}
		if _, exists := indexed[identity.Name]; exists {
			return 0, fmt.Errorf("duplicate configuration root %q", identity.Name)
		}
		indexed[identity.Name] = rawNode
	}
	encoded, err := json.MarshalIndent(indexed, "", "  ")
	if err != nil {
		return 0, err
	}
	encoded = append(encoded, '\n')
	path := filepath.Join(output, "data", "yang-config-tree.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return 0, err
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return 0, err
	}
	return len(indexed), nil
}
