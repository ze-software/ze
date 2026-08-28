// VALIDATES: AC-4, AC-5, and AC-12 leave only compiled native development tools.
// PREVENTS: build-ignored tools, script code, Python launchers, deleted producer
// paths, or subprocess-driven go-run tests returning.
package le

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/le/lepath"
)

func walkLeGo(t *testing.T, visit func(rel string, body string)) {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	dir := filepath.Join(root, "internal", "le")
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // path is inside this checkout
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		visit(rel, string(body))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
}

func TestNoDevelopmentToolIsBuildIgnored(t *testing.T) {
	root := checkoutRoot(t)
	for _, rel := range trackedExistingFiles(t, root) {
		if filepath.Ext(rel) != ".go" || strings.HasPrefix(filepath.ToSlash(rel), "vendor/") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		header, _, _ := strings.Cut(string(body), "\npackage ")
		if strings.Contains(header, "//go:build "+"ignore") {
			t.Errorf("%s is constrained out of every build", rel)
		}
	}
}

func TestNoDevelopmentToolTestShellsOutToGoRun(t *testing.T) {
	walkLeGo(t, func(rel, body string) {
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}
		forms := []string{`"go", ` + `"run"`, `"go",` + `"run"`, `go ` + `run `}
		for _, form := range forms {
			if strings.Contains(body, form) {
				t.Errorf("%s invokes a compiled tool with %s", rel, form)
			}
		}
	})
}

func TestNoPythonLeRemains(t *testing.T) {
	root := checkoutRoot(t)
	for _, legacy := range []string{
		"scripts", "mk", "Makefile",
		"gokrazy/kernel/Makefile", "tools/installer-kernel/Makefile",
	} {
		path := filepath.Join(root, filepath.FromSlash(legacy))
		if info, err := os.Stat(path); err == nil {
			t.Fatalf("%s remains (%s)", legacy, info.Mode())
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy path %s: %v", legacy, err)
		}
	}
	for _, launcher := range []string{"le", "ze"} {
		path := filepath.Join(root, launcher)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read root %s launcher: %v", launcher, err)
		}
		first, _, _ := bytes.Cut(body, []byte{'\n'})
		if bytes.Contains(bytes.ToLower(first), []byte("python")) ||
			bytes.Contains(bytes.ToLower(body), []byte("python")) {
			t.Errorf("root %s is still a Python launcher", launcher)
		}
		if info, statErr := os.Stat(path); statErr != nil {
			t.Errorf("stat root %s launcher: %v", launcher, statErr)
		} else if info.Mode()&0o111 == 0 {
			t.Errorf("root %s launcher is not executable", launcher)
		}
	}

	var stale []string
	for _, rel := range trackedExistingFiles(t, root) {
		slash := filepath.ToSlash(rel)
		if strings.HasPrefix(slash, "scripts/") && isScriptCode(slash) {
			t.Errorf("tracked script code remains: %s", slash)
		}
		if firstPartySource(slash) {
			switch filepath.Ext(slash) {
			case ".py", ".pyc", ".sh", ".bash", ".mk":
				t.Errorf("tracked interpreted or Make source remains: %s", slash)
			}
			if filepath.Base(slash) == "Makefile" {
				t.Errorf("tracked Makefile remains: %s", slash)
			}
		}
		if !isSwapReferenceSource(slash) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read tracked source %s: %v", slash, err)
		}
		if bytes.IndexByte(body, 0) >= 0 || !utf8.Valid(body) {
			continue
		}
		if strings.HasSuffix(slash, ".ci") || strings.HasSuffix(slash, ".et") || strings.HasSuffix(slash, ".wb") {
			lower := bytes.ToLower(body)
			for _, marker := range [][]byte{
				[]byte("#!/usr/bin/env python"), []byte("#!/usr/bin/python"),
				[]byte("#!/bin/sh"), []byte("#!/bin/bash"),
				[]byte("tmpfs=run.py"), []byte("tmpfs=run.sh"),
				[]byte("exec=python"), []byte("exec=sh"), []byte("exec=bash"),
			} {
				if bytes.Contains(lower, marker) {
					stale = append(stale, slash+": embedded interpreted helper "+string(marker))
				}
			}
		}
		for _, reference := range staleLeReferences(string(body)) {
			stale = append(stale, slash+": "+reference)
		}
	}
	slices.Sort(stale)
	if len(stale) != 0 {
		t.Fatalf("tracked non-plan source still names deleted le producers:\n%s", strings.Join(stale, "\n"))
	}
}

func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	return root
}

func trackedExistingFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "ls-files", "-z", "--cached")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list tracked files: %v", err)
	}
	var files []string
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := filepath.Clean(string(raw))
		info, statErr := os.Stat(filepath.Join(root, rel))
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			t.Fatalf("stat tracked path %s: %v", rel, statErr)
		}
		if !info.IsDir() {
			files = append(files, rel)
		}
	}
	slices.Sort(files)
	return files
}

// firstPartySource excludes vendored source and the predecessor's compatibility
// harness. The ExaBGP tree is an external interoperability subject: its Python
// process fixtures exercise the API Ze claims to accept, so converting or
// deleting them would remove the behavior under test rather than port Ze
// development tooling.
func firstPartySource(path string) bool {
	return !strings.HasPrefix(path, "vendor/") &&
		!strings.HasPrefix(path, "third_party/") &&
		!strings.HasPrefix(path, "test/exabgp-compat/")
}

func TestExaBGPCompatibilityProcessFixturesRemainExternalSubjects(t *testing.T) {
	root := checkoutRoot(t)
	for _, rel := range []string{
		"test/exabgp-compat/bin/functional",
		"test/exabgp-compat/bin/exabgp",
		"test/exabgp-compat/etc/run/api-health.run",
	} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read external ExaBGP fixture %s: %v", rel, err)
		}
		if !bytes.Contains(body, []byte("python")) {
			t.Errorf("external ExaBGP fixture %s no longer exercises its Python contract", rel)
		}
		if firstPartySource(rel) {
			t.Errorf("external ExaBGP fixture %s was classified as first-party tooling", rel)
		}
	}
}

func isScriptCode(path string) bool {
	switch filepath.Ext(path) {
	case ".py", ".sh", ".go":
		return true
	default:
		return false
	}
}

func isSwapReferenceSource(path string) bool {
	if path == "internal/le/contract_test.go" ||
		strings.HasPrefix(path, "plan/") ||
		strings.HasPrefix(path, "vendor/") ||
		strings.Contains(path, "/testdata/") ||
		strings.Contains(path, "/fixtures/") ||
		strings.HasSuffix(path, ".patch") {
		return false
	}
	return true
}

func staleLeReferences(body string) []string {
	legacyDir := "scripts" + "/le"
	formerImport := "github.com/ze-software/ze" + "/le/"
	var references []string
	if strings.Contains(body, legacyDir) {
		references = append(references, legacyDir)
	}
	if strings.Contains(body, formerImport) {
		references = append(references, formerImport)
	}
	slices.Sort(references)
	return slices.Compact(references)
}
