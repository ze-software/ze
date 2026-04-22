// Design: (none -- build tool)
//
// sync_web copies vendored web assets from third_party/web/{htmx,ze,uplot}/ to all
// consumer directories. Used by `make ze-sync-vendor-web`.
//
// Source of truth: third_party/web/. See third_party/web/MANIFEST.md.
//
// Usage: go run scripts/vendor/sync_web.go
//
// Replaces the previous bash implementation (scripts/sync-vendor-web.sh).

//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// asset describes one source-to-name mapping.
type asset struct {
	srcDir string // relative to repo root, e.g. "third_party/web/htmx"
	name   string // file name shared between source and consumer
}

var assets = []asset{
	{"third_party/web/htmx", "htmx.min.js"},
	{"third_party/web/htmx", "sse.js"},
	{"third_party/web/ze", "ze.svg"},
	{"third_party/web/uplot", "uPlot.min.js"},
	{"third_party/web/uplot", "uPlot.min.css"},
}

var consumers = []string{
	"internal/chaos/web/assets",
	"internal/component/lg/assets",
	"internal/component/web/assets",
}

func sync() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	changed := 0
	for _, dest := range consumers {
		destDir := filepath.Join(root, dest)
		info, err := os.Stat(destDir)
		if err != nil || !info.IsDir() {
			fmt.Fprintf(os.Stderr, "warning: consumer directory not found, skipping: %s\n", destDir)
			continue
		}

		for _, a := range assets {
			src := filepath.Join(root, a.srcDir, a.name)
			dst := filepath.Join(destDir, a.name)

			srcData, err := os.ReadFile(src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: vendor file not found: %s\n", src)
				continue
			}

			dstData, err := os.ReadFile(dst)
			if err == nil && bytes.Equal(srcData, dstData) {
				continue
			}

			if err := os.WriteFile(dst, srcData, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", dst, err)
			}
			fmt.Printf("synced: %s\n", dst)
			changed++
		}
	}

	if changed == 0 {
		fmt.Println("all consumer copies are up to date")
	} else {
		fmt.Printf("synced %d file(s)\n", changed)
	}
	return nil
}

// repoRoot returns the directory containing go.mod, walking up from the
// current working directory.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func main() {
	if err := sync(); err != nil {
		fmt.Fprintf(os.Stderr, "sync_web: %v\n", err)
		os.Exit(1)
	}
}
