// Design: docs/guide/netlab.md -- preserve failed node state before cleanup.
package testnetlab

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type liveCapture struct {
	file string
	args []string
}

// keepLiveDiagnostics saves inputs and bounded node state before teardown
// removes them. Each failure gets a private directory because these files can
// contain the lab password, and a later attempt must retain earlier evidence.
func (c *Checker) keepLiveDiagnostics(ctx context.Context, directory, startupOutput string, snapshot []byte, nodes map[string]liveNode) (string, error) {
	artifacts, err := c.FS.MkdirTemp(directory, "diagnostics-")
	if err != nil {
		return "", fmt.Errorf("create private netlab diagnostics: %w", err)
	}
	err = c.FS.WriteFile(filepath.Join(artifacts, "netlab-up.log"), []byte(startupOutput), 0o600)
	if len(snapshot) > 0 {
		err = errors.Join(err, c.FS.WriteFile(filepath.Join(artifacts, "netlab.snapshot.yml"), snapshot, 0o600))
	}
	private := c.FS
	private.MkdirAll = func(path string, _ fs.FileMode) error {
		return c.FS.MkdirAll(path, 0o700)
	}
	private.WriteFile = func(path string, body []byte, _ fs.FileMode) error {
		return c.FS.WriteFile(path, body, 0o600)
	}
	generated := filepath.Join(directory, "node_files")
	if _, statErr := c.FS.Stat(generated); statErr == nil {
		err = errors.Join(err, copyTree(private, generated, filepath.Join(artifacts, "node_files")))
	} else if !errors.Is(statErr, os.ErrNotExist) {
		err = errors.Join(err, statErr)
	}
	for _, name := range sortedNames(nodes) {
		node := nodes[name]
		if node.Hostname == "" {
			continue
		}
		for _, capture := range []liveCapture{
			{name + ".inspect.json", []string{"docker", "inspect", node.Hostname}},
			{name + ".log", []string{"docker", "logs", "--tail", "200", node.Hostname}},
		} {
			err = errors.Join(err, c.keepLiveCommand(ctx, directory, artifacts, capture.file, capture.args))
		}
	}
	captures := []liveCapture{
		{"addresses.json", []string{"ip", "-j", "address", "show"}},
		{"routes.json", []string{"ip", "-j", "route", "show", "table", "main"}},
	}
	for _, proof := range nodeProofs(liveNode{}) {
		captures = append(captures, liveCapture{proof.file, []string{"ze", "cli", "-c", proof.command + " | json compact"}})
	}
	for _, detail := range []struct{ file, command string }{
		{"rib.json", "show rib"},
		{"plugins.json", "show plugin list"},
		{"ospf-interface.json", "show ospf interface"},
		{"isis-interface.json", "show isis interface"},
		{"ospf-route.json", "show ospf route"},
		{"isis-route.json", "show isis route"},
		{"ospf-database-router.json", "show ospf database router"},
		{"isis-database-detail.json", "show isis database detail"},
	} {
		captures = append(captures, liveCapture{detail.file, []string{"ze", "cli", "-c", detail.command + " | json compact"}})
	}
	// Capture each essential query across every node before collecting detail;
	// an unavailable node must not consume the whole budget before its peers.
	for _, capture := range captures {
		for _, name := range sortedNames(nodes) {
			node := nodes[name]
			if node.Hostname == "" {
				continue
			}
			argv := append([]string{"docker", "exec", node.Hostname}, capture.args...)
			err = errors.Join(err, c.keepLiveCommand(ctx, directory, artifacts, name+"."+capture.file, argv))
		}
	}
	return artifacts, err
}

// keepLiveCommand leaves a failure record instead of an empty success artifact.
// Both the command deadline and the caller's total capture deadline apply.
func (c *Checker) keepLiveCommand(ctx context.Context, directory, artifacts, file string, argv []string) error {
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := c.Execute(Command{Context: probe, Argv: argv, Env: c.Env, Dir: directory})
	var cause error
	if result.Err != nil {
		cause = fmt.Errorf("%w: %s", result.Err, result.Stderr)
	} else if result.Code != 0 {
		cause = fmt.Errorf("exited %d: %s", result.Code, result.Stderr)
	}
	// The executor represents a killed process as an exit code, so retain the
	// deadline or cancellation that caused it alongside the process result.
	cause = errors.Join(cause, probe.Err())
	if cause != nil {
		captureErr := opererr.Wrap(cause, "cannot retain netlab diagnostic "+file,
			"the failed query leaves this node's state unknown", "./le test netlab run")
		var body textbuf.Buffer
		body.Err(captureErr).Str("\nCommand: ").Str(strings.Join(argv, " "))
		body.Str("\nstdout:\n").Str(result.Stdout).Str("\nstderr:\n").Str(result.Stderr)
		return errors.Join(captureErr, c.FS.WriteFile(filepath.Join(artifacts, file+".error.txt"), body.Bytes(), 0o600))
	}
	path := filepath.Join(artifacts, file)
	if strings.HasSuffix(file, ".json") {
		err := c.FS.WriteFile(path, []byte(result.Stdout), 0o600)
		if result.Stderr != "" {
			err = errors.Join(err, c.FS.WriteFile(path+".stderr.txt", []byte(result.Stderr), 0o600))
		}
		return err
	}
	return c.FS.WriteFile(path, []byte(result.Stdout+result.Stderr), 0o600)
}
