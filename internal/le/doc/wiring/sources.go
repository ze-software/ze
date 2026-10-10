// Design: docs/architecture/core-design.md -- which changed file needs which gate
// Overview: docwiring.go -- the gate this selection feeds
//
// sources.go selects the checks that a change needs. Each predicate asks whether
// a changed path can alter one gate's answer. Their union uses a fixed order so
// repeated runs over one diff agree.
//
// A content predicate reads both the working tree and the base commit the
// change is judged against. A change can add or remove its marker.

package docwiring

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
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
	actionDigest,
	actionInventory,
	actionCommandList,
	actionPluginImportsCheck,
	"doc check/templ-output",
	"spec citation/anchors",
}

// selectedActions answers the checks this change needs, in actionOrder. base
// is the commit the change is judged against: a file the change deleted is
// classified by what it held there.
func selectedActions(root, base string, changed []string) ([]string, error) {
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
			match   func(string, string, string) (bool, error)
			targets []string
		}{
			{isCommandSource, []string{actionDocvalidCommandContract}},
			{isDocSource, []string{actionDocCheckVerify, actionDocsToCodeIndexCheck}},
			{isDigestSource, []string{actionDigest}},
			{isInventorySource, []string{actionInventory, actionCommandList, actionPluginImportsCheck}},
		} {
			hit, err := rule.match(root, base, path)
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
	return specpath.IsSpec(path) || strings.HasPrefix(path, "plan/learned/")
}

// isCommandOwnershipSource reports a change that must run the command ownership
// gate. Sources include the checker, registry, shim, owner register.go files,
// and the ze dispatch and central-registration files.
func isCommandOwnershipSource(path string) bool {
	if path == "internal/le/cli/ownership/register.go" || path == "cmd/ze/main.go" {
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
	"command.MustRegisterLocal",
	"command.MustRegisterLocalMeta",
	"pluginserver.RegisterRPCs",
	"ze:command",
}

// isCommandSource reports a change that must re-run the command contract gate.
func isCommandSource(root, base, path string) (bool, error) {
	switch path {
	case "internal/le/doc/yangcontract/actions.go", "internal/le/cli/list/register.go",
		"internal/component/config/yang/command.go",
		"internal/component/plugin/server/command.go":
		return true, nil
	}
	if strings.HasSuffix(path, "-cmd.yang") {
		return true, nil
	}
	if strings.HasSuffix(path, ".yang") {
		return fileOrBaseContains(root, base, path, "ze:command")
	}
	if !strings.HasSuffix(path, ".go") {
		return false, nil
	}
	return fileOrBaseContainsAny(root, base, path, commandMarkers[:])
}

// isDocSource reports a change that must re-run the documentation gates.
func isDocSource(root, base, path string) (bool, error) {
	switch path {
	case "internal/le/doc/yangcontract/actions.go",
		"internal/le/doc/index/codetodocs.go", "ai/CODE-TO-DOCS.md":
		return true, nil
	}
	if (strings.HasPrefix(path, "docs/") || path == "README.md") && strings.HasSuffix(path, ".md") {
		return fileOrBaseContains(root, base, path, "<!-- source:")
	}
	return false, nil
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
	slices.Sort(out)
	return out, nil
}

// isBaseSeparator reports the characters a digest-base header separates its
// subtrees with.
func isBaseSeparator(r rune) bool {
	return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
}

// isDigestSource reports a change that must validate digest anchors. Sources
// include a digest, the checker, or non-test Go under an anchored subtree.
func isDigestSource(root, base, path string) (bool, error) {
	if strings.HasPrefix(path, "ai/digests/") && strings.HasSuffix(path, ".md") {
		return true, nil
	}
	if path == "internal/le/ai/digest/register.go" {
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
func isInventorySource(root, base, path string) (bool, error) {
	switch path {
	case "internal/le/repo/inventory/inventory.go", "internal/le/plugin/imports/pluginimports.go",
		"internal/component/plugin/all/all.go":
		return true, nil
	}
	if strings.HasSuffix(path, ".yang") && strings.HasPrefix(path, "internal/") {
		return true, nil
	}
	if strings.HasSuffix(path, "register.go") && strings.HasPrefix(path, "internal/") {
		return fileOrBaseContainsAny(root, base, path, registryMarkers[:])
	}
	return false, nil
}

func fileOrBaseContains(root, base, path, needle string) (bool, error) {
	return fileOrBaseContainsAny(root, base, path, []string{needle})
}

// fileOrBaseContainsAny reports a needle in the path's current content or in
// its content at base, so a file the change deleted still selects its gates.
func fileOrBaseContainsAny(root, base, path string, needles []string) (bool, error) {
	text, err := readCurrentOrEmpty(root, path)
	if err != nil {
		return false, err
	}
	before, err := readBaseOrEmpty(root, base, path)
	if err != nil {
		return false, err
	}
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true, nil
		}
		if strings.Contains(before, needle) {
			return true, nil
		}
	}
	return false, nil
}

// readBaseOrEmpty answers a path's content at the base commit, or "" when the
// base tree does not hold the path. An absent path is a path this change ADDS,
// which is the case the caller is looking for rather than an error.
//
// A git failure is an error, never "": an empty base copy of a deleted file
// selects no gate for it, and the change is then judged by nobody.
func readBaseOrEmpty(root, base, path string) (string, error) {
	listing, err := gitOutput(root, "ls-tree", "-z", "--name-only", base, "--", path)
	if err != nil {
		return "", err
	}
	if listing == "" {
		return "", nil
	}
	var tb textbuf.Buffer
	return gitOutput(root, "show", tb.Str(base).Byte(':').Str(path).String())
}

// gitOutput runs one git query in root and answers its output. A failure names
// the query and git's message.
func gitOutput(root string, argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", argv...) //nolint:gosec // a fixed git query over a repository path the caller named
	cmd.Dir = root
	var errOut textbuf.Buffer
	cmd.Stderr = &errOut
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(argv, " "), err, strings.TrimSpace(errOut.String()))
	}
	return string(out), nil
}

// actionOrderList answers the native check order.
func actionOrderList() []string { return slices.Clone(actionOrder[:]) }

// The delegated action names this package both dispatches and wires sources for.
const (
	actionDigest                  = "digest"
	actionInventory               = "inventory"
	actionCommandList             = "command list"
	actionDocCheckVerify          = "doc check/verify"
	actionDocsToCodeIndexCheck    = "docs-to-code/index-check"
	actionDocvalidCommandContract = "docvalid/command-contract"
	actionPluginImportsCheck      = "plugin imports/check"
)
