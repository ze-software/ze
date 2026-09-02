// Design: docs/architecture/core-design.md -- which changed file needs which gate
// Overview: docwiring.go -- the gate this selection feeds
//
// sources.go selects the checks that a diff needs. Each predicate asks whether
// a changed path can alter one gate's answer. Their union uses a fixed order so
// repeated runs over one diff agree.
//
// A content predicate reads both the working tree and HEAD. A change can add or
// remove its marker.

package docwiring

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/discoveryindex"
)

// wiringTarget is the one selected check implemented directly by this package.
// Every other name in the order below is the stable target identity of a
// linked Go callback.
const wiringTarget = "wiring"

// gitTimeout bounds one git query. Each one lists paths and forks nothing, so a
// run past this bound is a hung index lock rather than a slow query.
const gitTimeout = 2 * time.Minute

// templCheckerSource is the Go owner of the orphan and freshness scope checks.
const templCheckerSource = "internal/le/doc/wiring/templ.go"

// actionOrder is the deterministic order selected checks run in.
var actionOrder = [...]string{
	wiringTarget,
	actionDocvalidCommandContract,
	"command ownership",
	actionDocCheckVerify,
	actionDocsToCodeIndexCheck,
	actionDiscoveryIndexCheck,
	actionDigest,
	actionInventory,
	actionCommandList,
	actionPluginImportsCheck,
	"doc check/templ-output",
	"spec citation/anchors",
}

// selectedActions answers the checks this diff needs, in actionOrder.
func selectedActions(root string, changed []string) ([]string, error) {
	selected := make(map[string]bool)
	for _, path := range changed {
		if isWiringSource(path) {
			selected[wiringTarget] = true
		}
		if isTemplSource(path) {
			selected["doc check/templ-output"] = true
		}
		if isPlanSource(path) {
			selected["spec citation/anchors"] = true
		}
		if isCommandOwnershipSource(path) {
			selected["command ownership"] = true
		}

		for _, rule := range []struct {
			match   func(string, string) (bool, error)
			targets []string
		}{
			{isCommandSource, []string{actionDocvalidCommandContract}},
			{isDocSource, []string{actionDocCheckVerify, actionDocsToCodeIndexCheck}},
			{isDiscoverySource, []string{actionDiscoveryIndexCheck}},
			{isDigestSource, []string{actionDigest}},
			{isInventorySource, []string{actionInventory, actionCommandList, actionPluginImportsCheck}},
		} {
			hit, err := rule.match(root, path)
			if err != nil {
				return nil, err
			}
			if !hit {
				continue
			}
			for _, target := range rule.targets {
				selected[target] = true
			}
		}
	}

	out := make([]string, 0, len(selected))
	for _, target := range actionOrder {
		if selected[target] {
			out = append(out, target)
		}
	}
	return out, nil
}

// isWiringSource reports a non-test Go file under a tree the wiring check
// judges.
func isWiringSource(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") &&
		(strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/"))
}

// isTemplSource reports a change that must re-run the templ generated-output
// freshness gate.
//
// A .templ file is a source, and a *_templ.go file is its output. The Go
// checker is included because its scope decides which pairs are judged.
func isTemplSource(path string) bool {
	if path == templCheckerSource {
		return true
	}
	if strings.HasSuffix(path, ".templ") {
		return true
	}
	return strings.HasSuffix(path, "_templ.go")
}

// isPlanSource reports a change that must run the spec citation freshness gate.
// This includes specs, learned summaries, the checker, and the citation
// baseline. Spec closure removes a spec file, so the gate detects a dangling
// citation from a sibling spec.
func isPlanSource(path string) bool {
	if path == "internal/le/spec/citation/anchors.go" || path == "plan/.citation-baseline" {
		return true
	}
	if !strings.HasSuffix(path, ".md") {
		return false
	}
	return strings.HasPrefix(path, "plan/spec-") || strings.HasPrefix(path, "plan/learned/")
}

// isCommandOwnershipSource reports a change that must run the command ownership
// gate. Sources include the checker, registry, shim, owner register.go files,
// and the ze dispatch and central-registration files.
func isCommandOwnershipSource(path string) bool {
	if path == "internal/le/command/ownership/register.go" || path == "cmd/ze/main.go" {
		return true
	}
	if strings.HasPrefix(path, "internal/component/command/registry/") {
		return true
	}
	if strings.HasPrefix(path, "cmd/ze/internal/cmdregistry/") {
		return true
	}
	return strings.HasSuffix(path, "register.go") &&
		(strings.Contains(path, "/cli/") || strings.Contains(path, "/client/") ||
			strings.HasPrefix(path, "cmd/ze/"))
}

// commandMarkers are the spellings that make a Go file part of the command
// surface.
var commandMarkers = [...]string{
	"cmdregistry.MustRegisterLocal",
	"cmdregistry.MustRegisterLocalMeta",
	"pluginserver.RegisterRPCs",
	"ze:command",
}

// isCommandSource reports a change that must re-run the command contract gate.
func isCommandSource(root, path string) (bool, error) {
	switch path {
	case "internal/le/docvalid/actions.go", "internal/le/command/list/register.go",
		"internal/component/config/yang/command.go",
		"internal/component/plugin/server/command.go":
		return true, nil
	}
	if strings.HasSuffix(path, "-cmd.yang") {
		return true, nil
	}
	if strings.HasSuffix(path, ".yang") {
		return fileOrHeadContains(root, path, "ze:command")
	}
	if !strings.HasSuffix(path, ".go") {
		return false, nil
	}
	return fileOrHeadContainsAny(root, path, commandMarkers[:])
}

// isDocSource reports a change that must re-run the documentation gates.
func isDocSource(root, path string) (bool, error) {
	switch path {
	case "internal/le/docvalid/actions.go",
		"internal/le/docstocode/codetodocs.go", "ai/CODE-TO-DOCS.md":
		return true, nil
	}
	if (strings.HasPrefix(path, "docs/") || path == "README.md") && strings.HasSuffix(path, ".md") {
		return fileOrHeadContains(root, path, "<!-- source:")
	}
	return false, nil
}

// isDiscoverySource reports a change that can drift a generated discovery
// index.
//
// The path rules are the generator's own (internal/le/discoveryindex), so the
// router and the index cannot disagree about what feeds it. Here the header
// marker is matched against the working tree PLUS head, because a change either
// adds such a header or removes one.
func isDiscoverySource(root, path string) (bool, error) {
	header := ""
	if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
		current, err := readCurrentOrEmpty(root, path)
		if err != nil {
			return false, err
		}
		var tb textbuf.Buffer
		header = tb.Str(current).Byte('\n').Str(readHeadOrEmpty(root, path)).String()
	}
	return discoveryindex.IsSource(path, header), nil
}

var digestBaseRe = regexp.MustCompile(`<!--\s*digest-base:\s*(.+?)\s*-->`)

// digestBases answers the subtrees the digests anchor into, read from their own
// headers. A Go edit under one of these can shift the line numbers a digest
// cites.
func digestBases(root string) ([]string, error) {
	bases := make(map[string]bool)
	dir := filepath.Join(root, "ai", "digests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// A tree with no digests anchors nothing, which is a fact about the
			// tree rather than a read that fell short.
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // a digest of the tree the caller named
		if err != nil {
			// A digest this scan cannot read anchors into subtrees nobody then
			// routes, so a Go edit under one of them skips the anchor check.
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		for _, m := range digestBaseRe.FindAllStringSubmatch(string(raw), -1) {
			for _, token := range strings.FieldsFunc(strings.TrimSpace(m[1]), isBaseSeparator) {
				if token != "" {
					bases[token] = true
				}
			}
		}
	}

	out := make([]string, 0, len(bases))
	for base := range bases {
		out = append(out, base)
	}
	sort.Strings(out)
	return out, nil
}

// isBaseSeparator reports the characters a digest-base header separates its
// subtrees with.
func isBaseSeparator(r rune) bool {
	return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
}

// isDigestSource reports a change that must validate digest anchors. Sources
// include a digest, the checker, or non-test Go under an anchored subtree.
func isDigestSource(root, path string) (bool, error) {
	if strings.HasPrefix(path, "ai/digests/") && strings.HasSuffix(path, ".md") {
		return true, nil
	}
	if path == "internal/le/digest/register.go" {
		return true, nil
	}
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false, nil
	}
	bases, err := digestBases(root)
	if err != nil {
		return false, err
	}
	var tb textbuf.Buffer
	for _, base := range bases {
		if path == base {
			return true, nil
		}
		if strings.HasPrefix(path, tb.Reset().Str(base).Byte('/').String()) {
			return true, nil
		}
	}
	return false, nil
}

// registryMarkers are the spellings that make a register.go part of the runtime
// inventory.
var registryMarkers = [...]string{
	"registry.Register",
	"MustRegister",
	"RegisterNamespace",
	"RegisterBackend",
	"yang.MustRegister",
}

// isInventorySource reports a change that must re-run the inventory gates.
func isInventorySource(root, path string) (bool, error) {
	switch path {
	case "internal/le/inventory/inventory.go", "internal/le/plugin/imports/pluginimports.go",
		"internal/component/plugin/all/all.go":
		return true, nil
	}
	if strings.HasSuffix(path, ".yang") && strings.HasPrefix(path, "internal/") {
		return true, nil
	}
	if strings.HasSuffix(path, "register.go") && strings.HasPrefix(path, "internal/") {
		return fileOrHeadContainsAny(root, path, registryMarkers[:])
	}
	return false, nil
}

func fileOrHeadContains(root, path, needle string) (bool, error) {
	return fileOrHeadContainsAny(root, path, []string{needle})
}

func fileOrHeadContainsAny(root, path string, needles []string) (bool, error) {
	text, err := readCurrentOrEmpty(root, path)
	if err != nil {
		return false, err
	}
	head := readHeadOrEmpty(root, path)
	for _, needle := range needles {
		if strings.Contains(text, needle) || strings.Contains(head, needle) {
			return true, nil
		}
	}
	return false, nil
}

// readHeadOrEmpty answers a path's content at HEAD, or "" when git does not
// hold it there. A path git cannot show is a path this change ADDS, which is
// the case the caller is looking for rather than an error.
func readHeadOrEmpty(root, path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	var tb textbuf.Buffer
	cmd := exec.CommandContext(ctx, "git", "show", tb.Str("HEAD:").Str(path).String()) //nolint:gosec // a repository path the caller named, handed to git as one argument
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// ChangedFiles answers every path this working tree has changed against HEAD,
// staged or not, tracked or not.
//
// A failed git command means that no check judged the tree. The caller reports
// the run failure without a group.
func ChangedFiles(root string) ([]string, error) {
	files := make(map[string]bool)
	for _, argv := range [][]string{
		{gitDiff, "--name-only"},
		{gitDiff, "--cached", "--name-only"},
		{"ls-files", "--others", "--exclude-standard"},
	} {
		out, err := gitLines(root, argv)
		if err != nil {
			return nil, err
		}
		for _, line := range out {
			files[line] = true
		}
	}

	out := make([]string, 0, len(files))
	for path := range files {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

// gitFailure names the failed query and git's message. A command failure means
// that no check judged the tree. A caller can distinguish it from a check
// finding.
func gitFailure(argv []string, stderr string) error {
	var tb textbuf.Buffer
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = tb.Str("git ").Join(argv, " ").Str(" failed").String()
	}
	return errors.New(message)
}

// gitLines runs one git query and answers its non-blank output lines.
func gitLines(root string, argv []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", argv...) //nolint:gosec // one of three fixed queries declared above
	cmd.Dir = root
	var errOut textbuf.Buffer
	cmd.Stderr = &errOut
	out, err := cmd.Output()
	if err != nil {
		return nil, gitFailure(argv, errOut.String())
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// actionOrderList answers the native check order.
func actionOrderList() []string { return slices.Clone(actionOrder[:]) }

// The delegated action names this package both dispatches and wires sources for.
const (
	actionDiscoveryIndexCheck     = "discovery-index/check"
	actionDigest                  = "digest"
	actionInventory               = "inventory"
	actionCommandList             = "command list"
	actionDocCheckVerify          = "doc check/verify"
	actionDocsToCodeIndexCheck    = "docs-to-code/index-check"
	actionDocvalidCommandContract = "docvalid/command-contract"
	actionPluginImportsCheck      = "plugin imports/check"
)
