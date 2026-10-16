// Design: website/AI.md -- configuration reference data comes from the live Ze schema
package site

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	docyangcontract "github.com/ze-software/ze/internal/le/doc/yangcontract"
)

// schemaExtractionTimeout bounds one run of the compiled daemon's schema
// command, which compiles nothing.
const schemaExtractionTimeout = 2 * time.Minute

type yangNode struct {
	Name string `json:"name"`
}

// liveYANGConfigTree answers `show yang tree --config | json` for one checkout.
// It is a variable so a test can state a tree rather than compile the daemon.
var liveYANGConfigTree = runYANGConfigTree

// runYANGConfigTree runs the daemon's typed schema command and answers its JSON.
//
// The binary is run as it stands. A build passes the daemon it compiled from
// THIS checkout (liveDaemon), and the config-tree action passes the binary its
// caller named or one it compiled the same way.
func runYANGConfigTree(repository, binary string) ([]byte, error) {
	info, err := os.Stat(binary)
	if err != nil {
		return nil, fmt.Errorf("ze binary %s: %w", binary, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("ze binary %s: not a regular file", binary)
	}

	ctx, cancel := context.WithTimeout(context.Background(), schemaExtractionTimeout)
	defer cancel()
	// #nosec G204 -- the binary is the daemon this build compiled or the one
	// the operator named, and the argv is fixed.
	command := exec.CommandContext(ctx, binary, "cli", "-c", "show yang tree --config | json")
	command.Dir = repository
	var stderr bytes.Buffer
	command.Stderr = &stderr
	raw, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s show yang tree --config | json: %w: %s", binary, err, stderr.String())
	}
	return raw, nil
}

// liveDaemon compiles the daemon a build reads its command catalog and its
// configuration tree from, once per build, from THIS checkout's sources with
// the feature gates the shipped daemon carries. A bin/ze somebody left behind
// was built at an unknown commit with unknown gates and would publish a
// reference for a daemon nobody ships, so the build compiles its own. It is a
// variable so a test that states both answers compiles nothing.
var liveDaemon = docyangcontract.BuildDaemon

// extractYANGConfigTree writes the top-level-name-indexed configuration tree
// the site publishes, and answers how many roots it holds.
func extractYANGConfigTree(repository, output, binary string) (int, error) {
	raw, err := liveYANGConfigTree(repository, binary)
	if err != nil {
		return 0, err
	}
	return writeYANGConfigTree(output, raw)
}

// configTreeFrom extracts the configuration tree from the binary named, or,
// when none is named, from a daemon it compiles from this checkout into a
// temporary directory it removes afterwards.
func configTreeFrom(repository, output, binary string) (count int, err error) {
	if binary != "" {
		return extractYANGConfigTree(repository, output, binary)
	}
	daemonDir, err := os.MkdirTemp("", "ze-site-daemon-*")
	if err != nil {
		return 0, fmt.Errorf("daemon build directory: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(daemonDir)) }()

	daemon, err := liveDaemon(repository, daemonDir)
	if err != nil {
		return 0, err
	}
	return extractYANGConfigTree(repository, output, daemon)
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
