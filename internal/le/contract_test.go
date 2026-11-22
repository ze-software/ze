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

	lepath "github.com/ze-software/ze/internal/le/le/path"
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
		if err := developmentToolGoRun(body); err != nil {
			t.Errorf("%s: %v", rel, err)
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
		if isDisallowedScriptSource(slash) {
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
	for raw := range bytes.SplitSeq(out, []byte{0}) {
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

// examplePluginPythonDir holds the example external plugin written in Python,
// and the one-file library it is built on.
const examplePluginPythonDir = "examples/plugin/python/"

// examplePluginSources is the closed list of Python files
// TestNoPythonLeRemains does not refuse: the one-file plugin library and the
// example plugin built on it. A new example file is a deliberate edit of this
// list, in a reviewed commit.
var examplePluginSources = []string{
	examplePluginPythonDir + "ze_plugin.py",
	examplePluginPythonDir + "acme_monitor.py",
}

// isExamplePluginSource reports whether path is exactly one of the files in
// examplePluginSources. It matches nothing else: no other file of that
// directory, no file below it, and no file anywhere else.
//
// The two files are not development tooling: Ze serves the external plugin
// contract to programs in any language, and an example of that contract in
// another language is a subject Ze serves, the way the ExaBGP fixtures are a
// subject Ze interoperates with.
// TestPythonExamplePluginsRemainPluginContractSubjects pins the list and what
// the directory may hold.
func isExamplePluginSource(path string) bool {
	return slices.Contains(examplePluginSources, path)
}

// TestPythonExamplePluginsRemainPluginContractSubjects pins the exemption
// TestNoPythonLeRemains gives the Python example plugin. Method: a table over
// isExamplePluginSource, then the tracked files of the example directory.
//
// VALIDATES: the exemption is exactly the two listed files (a sibling `.py`, a
// nested `.py`, another kind of script and a `.py` elsewhere are each not
// exempt); both listed files are tracked and carry no shebang, so neither is a
// launcher; and the directory holds, at any depth, no other `.py`, `.pyc`,
// `.sh`, `.bash` or `.mk` file and no Makefile.
// PREVENTS: Python development tooling returning beside or below the example,
// the exemption widening without an edit of the list, and a list that outlives
// the files it names.
func TestPythonExamplePluginsRemainPluginContractSubjects(t *testing.T) {
	for path, want := range map[string]bool{
		examplePluginPythonDir + "ze_plugin.py":            true,
		examplePluginPythonDir + "acme_monitor.py":         true,
		examplePluginPythonDir + "gen.py":                  false,
		examplePluginPythonDir + "tools/gen.py":            false,
		examplePluginPythonDir + "tools/acme_monitor.py":   false,
		examplePluginPythonDir + "run.sh":                  false,
		examplePluginPythonDir + "Makefile":                false,
		examplePluginPythonDir + "acme_monitor.pyc":        false,
		"examples/plugin/acme_monitor.py":                  false,
		"examples/plugin/python-tools/acme_monitor.py":     false,
		"internal/le/" + examplePluginPythonDir + "gen.py": false,
		"internal/le/acme_monitor.py":                      false,
		"scripts/acme_monitor.py":                          false,
	} {
		if got := isExamplePluginSource(path); got != want {
			t.Errorf("isExamplePluginSource(%q) = %v, want %v", path, got, want)
		}
	}

	root := checkoutRoot(t)
	tracked := trackedExistingFiles(t, root)
	for _, listed := range examplePluginSources {
		if !slices.Contains(tracked, filepath.FromSlash(listed)) {
			t.Errorf("%s is listed in examplePluginSources and is not a tracked file: remove it from the list", listed)
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(listed)))
		if err != nil {
			t.Fatalf("read example plugin source %s: %v", listed, err)
		}
		if bytes.HasPrefix(body, []byte("#!")) {
			t.Errorf("%s carries a shebang: an example plugin is run by the program its config names, never as a launcher", listed)
		}
	}

	for _, rel := range tracked {
		slash := filepath.ToSlash(rel)
		if !strings.HasPrefix(slash, examplePluginPythonDir) || isExamplePluginSource(slash) {
			continue
		}
		switch filepath.Ext(slash) {
		case ".py", ".pyc", ".sh", ".bash", ".mk":
			t.Errorf("%s is interpreted or Make source in the example directory and is not in examplePluginSources", slash)
		}
		if filepath.Base(slash) == "Makefile" {
			t.Errorf("%s is a Makefile in the example directory", slash)
		}
	}
}

// contribNotifierDir holds the host-side systemd installer and leaf-guard
// shell scripts for the optional GitHub push notification unit.
const contribNotifierDir = "contrib/github-notifier/"

// contribNotifierScripts is the closed list of shell scripts under
// contrib/github-notifier/ that TestNoPythonLeRemains does not refuse. They
// install and exercise a Linux host systemd user unit rather than acting as le
// development tooling. Adding another script is a deliberate edit of this
// list, in a reviewed commit.
var contribNotifierScripts = []string{
	contribNotifierDir + "install.sh",
	contribNotifierDir + "leaf-guards.sh",
	contribNotifierDir + "test-leaf-guards.sh",
}

// isContribNotifierScript reports whether path is exactly one of the files in
// contribNotifierScripts. It matches nothing else.
func isContribNotifierScript(path string) bool {
	return slices.Contains(contribNotifierScripts, path)
}

// isDisallowedScriptSource reports whether path is subject to
// TestNoPythonLeRemains's ban on tracked interpreted and Make sources.
func isDisallowedScriptSource(path string) bool {
	if !firstPartySource(path) {
		return false
	}
	if isExamplePluginSource(path) {
		return false
	}
	if isContribNotifierScript(path) {
		return false
	}
	return true
}

// TestContribNotifierScriptsRemainHostInstallerSubjects pins the exemption
// TestNoPythonLeRemains gives the host systemd notifier scripts.
//
// VALIDATES: the exemption is exactly the three listed files (a sibling `.sh`,
// a nested `.sh`, a `.py` or Makefile there, and an `install.sh` elsewhere are
// each not exempt); all three listed files remain tracked; and the directory
// holds, at any depth, no other `.py`, `.pyc`, `.sh`, `.bash`, `.mk`, or
// Makefile.
// PREVENTS: interpreted development tooling creeping into contrib/ or the
// exemption widening without an explicit edit of contribNotifierScripts.
func TestContribNotifierScriptsRemainHostInstallerSubjects(t *testing.T) {
	for path, want := range map[string]bool{
		contribNotifierDir + "install.sh":            true,
		contribNotifierDir + "leaf-guards.sh":        true,
		contribNotifierDir + "test-leaf-guards.sh":   true,
		contribNotifierDir + "other.sh":              false,
		contribNotifierDir + "sub/install.sh":        false,
		contribNotifierDir + "notify.py":             false,
		contribNotifierDir + "Makefile":              false,
		"contrib/install.sh":                         false,
		"contrib/other-tool/install.sh":              false,
		"internal/le/" + contribNotifierDir + "a.sh": false,
		"scripts/install.sh":                         false,
	} {
		if got := isContribNotifierScript(path); got != want {
			t.Errorf("isContribNotifierScript(%q) = %v, want %v", path, got, want)
		}
	}

	root := checkoutRoot(t)
	tracked := trackedExistingFiles(t, root)
	for _, listed := range contribNotifierScripts {
		if !slices.Contains(tracked, filepath.FromSlash(listed)) {
			t.Errorf("%s is listed in contribNotifierScripts and is not a tracked file: remove it from the list", listed)
		}
	}

	for _, rel := range tracked {
		slash := filepath.ToSlash(rel)
		if !strings.HasPrefix(slash, contribNotifierDir) {
			continue
		}
		if isContribNotifierScript(slash) {
			continue
		}
		switch filepath.Ext(slash) {
		case ".py", ".pyc", ".sh", ".bash", ".mk":
			t.Errorf("%s is interpreted or Make source in %s and is not in contribNotifierScripts", slash, contribNotifierDir)
		}
		if filepath.Base(slash) == "Makefile" {
			t.Errorf("%s is a Makefile in %s", slash, contribNotifierDir)
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
