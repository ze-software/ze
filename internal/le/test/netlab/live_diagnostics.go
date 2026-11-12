// Design: docs/guide/netlab.md -- preserve failed node state before cleanup.
package testnetlab

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// keepLiveDiagnostics saves inputs and bounded Docker output before teardown
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
		for _, capture := range []struct {
			file string
			args []string
		}{
			{name + ".inspect.json", []string{"docker", "inspect", node.Hostname}},
			{name + ".log", []string{"docker", "logs", "--tail", "200", node.Hostname}},
		} {
			probe, cancel := context.WithTimeout(ctx, 5*time.Second)
			result := c.Execute(Command{Context: probe, Argv: capture.args, Env: c.Env, Dir: directory})
			cancel()
			err = errors.Join(err, c.FS.WriteFile(filepath.Join(artifacts, capture.file), []byte(result.Stdout+result.Stderr), 0o600))
			if result.Err != nil {
				err = errors.Join(err, fmt.Errorf("capture %s for %s: %w: %s", capture.args[1], node.Hostname, result.Err, result.Stderr))
				continue
			}
			if result.Code != 0 {
				err = errors.Join(err, fmt.Errorf("capture %s for %s exited %d: %s", capture.args[1], node.Hostname, result.Code, result.Stderr))
			}
		}
	}
	return artifacts, err
}
