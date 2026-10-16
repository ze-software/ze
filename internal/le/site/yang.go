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
	"strings"
	"time"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// daemonBuildTimeout bounds the one compile of cmd/ze a build performs.
const daemonBuildTimeout = 5 * time.Minute

// schemaExtractionTimeout bounds one run of the compiled daemon's schema
// command, which compiles nothing.
const schemaExtractionTimeout = 2 * time.Minute

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
// The binary is run as it stands. A build passes the daemon it compiled from
// THIS checkout (buildDaemon), and the config-tree action passes the binary its
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
// configuration tree from. It is a variable so a test that states both
// answers compiles nothing.
var liveDaemon = buildDaemon

// buildDaemon compiles cmd/ze from THIS checkout's sources into dir, with the
// feature gates the shipped daemon carries, and answers the binary's path.
//
// A bin/ze somebody left behind was built at an unknown commit with unknown
// gates and would publish a reference for a daemon nobody ships, which is the
// staleness the site build exists to remove, so the build compiles its own.
// It compiles ONCE: the catalog and the tree used to `go run` the daemon each,
// under a bound that covered the compile as well as the command, and a compile
// on a loaded host outlasted the catalog's two minutes. The compile now has its
// own bound, the one the tree's `go run` already granted it, and each command's
// bound covers only that command.
func buildDaemon(repository, dir string) (string, error) {
	tags, err := repofeaturetags.DaemonBuildTags(repository, daemonCoreTag)
	if err != nil {
		return "", err
	}
	binary := filepath.Join(dir, "ze")

	ctx, cancel := context.WithTimeout(context.Background(), daemonBuildTimeout)
	defer cancel()
	// #nosec G204 -- the argv is fixed apart from the build tags, which are read
	// from the checkout's own feature-gates.txt, and the output path is ours.
	command := exec.CommandContext(ctx, "go", "build", "-tags", tags, "-o", binary, "./cmd/ze")
	command.Dir = repository
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("compile ./cmd/ze: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return binary, nil
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
