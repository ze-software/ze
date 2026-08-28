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
	"time"
)

const schemaExtractionTimeout = 2 * time.Minute

type yangNode struct {
	Name string `json:"name"`
}

// extractYANGConfigTree runs the production binary's typed schema command and
// writes the top-level-name-indexed JSON consumed by the site.
func extractYANGConfigTree(repository, output, binary string) (int, error) {
	if binary == "" {
		binary = filepath.Join(repository, "bin", "ze")
	}
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = fmt.Errorf("not a regular file")
		}
		return 0, fmt.Errorf("ze binary %s: %w", binary, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), schemaExtractionTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "yang", "tree", "--json", "--config")
	command.Dir = repository
	var stderr bytes.Buffer
	command.Stderr = &stderr
	raw, err := command.Output()
	if err != nil {
		return 0, fmt.Errorf("%s yang tree --json --config: %w: %s", binary, err, stderr.String())
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
