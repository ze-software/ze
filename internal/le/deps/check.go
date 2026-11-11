// Design: docs/contributing/dependency-approval.md -- the whole-tree comparison of imports with the ledger
// Overview: deps.go -- the ledger and the import reader this applies

package deps

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Use is one third-party import: a package and the Ze directory importing it.
type Use struct {
	Package  string `json:"package"`
	Importer string `json:"importer"`
	File     string `json:"file"`
}

// Report is what `deps check` answers: the imports neither declared modules
// nor the ledger permit, and the rows no Go file uses any more.
type Report struct {
	Files      int   `json:"files"`
	Unapproved []Use `json:"unapproved"`
	Stale      []Row `json:"stale"`
}

// Check judges every Go file of the Ze module, tracked or not ignored, against
// the current module declarations and exact-ledger approvals. An unreadable
// file, ledger or go.mod is an error: skipping one would leave it unjudged.
func Check(root string) (Report, error) {
	var report Report
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // the checkout root this command was pointed at
	if err != nil {
		return report, err
	}
	module, err := ModulePath(string(goMod))
	if err != nil {
		return report, err
	}
	text, err := os.ReadFile(filepath.Join(root, LedgerRel)) //nolint:gosec // a checkout-relative constant path
	if err != nil {
		return report, errors.New("read the ledger " + LedgerRel + ": " + err.Error())
	}
	ledger, err := Parse(string(text))
	if err != nil {
		return report, err
	}
	vendorModules, err := os.ReadFile(filepath.Join(root, VendorModulesRel)) //nolint:gosec // a checkout-relative constant path
	// refuse-by-default: guards ParsePolicy -- an absent inventory grants no ordinary-module reuse; exact-ledger approvals remain available
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return report, err
		}
	}
	policy, err := ParsePolicy(string(goMod), string(vendorModules), ledger)
	if err != nil {
		return report, err
	}
	files, err := goFiles(root)
	if err != nil {
		return report, err
	}
	used := make(map[key]bool)
	for _, rel := range files {
		judged, err := Judged(root, rel)
		if err != nil {
			return report, err
		}
		if !judged {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a path git listed in this checkout
		// refuse-by-default: guards goFiles -- a tracked file deleted in the working tree imports nothing the build links
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return report, err
		}
		imports, err := Imports(rel, src)
		if err != nil {
			return report, err
		}
		report.Files++
		importer := Importer(rel)
		for _, pkg := range Added(module, nil, imports) {
			used[key{pkg, importer}] = true
			if policy.Allows(pkg, importer) {
				continue
			}
			report.Unapproved = append(report.Unapproved, Use{Package: pkg, Importer: importer, File: rel})
		}
	}
	for _, row := range ledger.Rows() {
		if !used[key{row.Package, row.Importer}] {
			report.Stale = append(report.Stale, row)
		}
	}
	slices.SortFunc(report.Unapproved, func(a, b Use) int {
		return strings.Compare(a.File+"\x00"+a.Package, b.File+"\x00"+b.Package)
	})
	return report, nil
}

// goFiles lists the checkout's Go files git tracks or would track: ignored
// trees (tmp/, bin/) are not Ze source.
func goFiles(root string) ([]string, error) {
	command := exec.CommandContext(context.Background(), "git", "-C", root, "ls-files", "-z",
		"--cached", "--others", "--exclude-standard", "--", "*.go")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, errors.New("list the checkout's Go files: " + err.Error() + ": " + strings.TrimSpace(stderr.String()))
	}
	var files []string
	for name := range strings.SplitSeq(string(output), "\x00") {
		if name == "" {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}

// splitLines answers text's lines with no trailing carriage return.
func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}
