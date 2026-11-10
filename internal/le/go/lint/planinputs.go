// Design: docs/contributing/running-commands.md -- the lint planning guard over its own population
// Related: verifylint.go -- the planner that grows the population at each flavor query
// Related: ../../job/inputsnapshot.go -- the content question this population is asked through
package golint

import (
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	"github.com/ze-software/ze/internal/le/job"
)

// toolInputs are the checkout paths a lint verdict depends on that no go list
// population names, because the Go command and golangci-lint read them for
// themselves on every query and every child. An entry ending in a slash is a
// tree.
//
//   - go.mod (with its toolchain pin), go.sum, go.work, go.work.sum and
//     vendor/modules.txt decide how every import resolves.
//   - The lint configuration and the ruleguard rules it loads decide which
//     findings exist.
//   - feature-gates.txt and the standard-library patches are the toolchain's
//     checkout inputs (gotoolchain.New, stdpatch).
//
// An absent entry costs nothing: a path that does not exist and never changes
// is never named as changed.
var toolInputs = []string{
	".golangci/",
	configName,
	"feature-gates.txt",
	"go.mod",
	"go.sum",
	"go.work",
	"go.work.sum",
	gen.PatchDir + "/",
	"vendor/modules.txt",
}

// sourceExtensions are the file name extensions go/build reads as package
// source (goodOSArchFile and the extension switch in go/build's Import). A
// file with another extension in a package directory (a page, a schema that is
// not embedded, an editor's temporary copy) is not something go list can
// start selecting.
var sourceExtensions = []string{
	".F", ".S", ".c", ".cc", ".cpp", ".cxx", ".f", ".f90", ".for", ".go", ".h",
	".hh", ".hpp", ".hxx", ".m", ".s", ".swig", ".swigcxx", ".sx", ".syso",
}

// planInputs is what a plan's verdict was measured against: the snapshot taken
// before the first flavor query, and the population of paths that can change
// the verdict. The planner refuses when a path the population reaches changed
// since the snapshot, at each flavor boundary and again at job admission.
type planInputs struct {
	start      job.InputSnapshot
	population lintPopulation
}

// queryInputs is what one flavor's go list read inside the checkout.
type queryInputs struct {
	// files are the compiler inputs: every source and embedded file of the
	// linted packages, their test files, and the dependency closure.
	files map[string]bool
	// linted are the directories of the packages the run lints.
	linted map[string]bool
	// deps are the directories of the packages the run only compiles.
	deps map[string]bool
	// embeds are the trees an embed pattern can select, each ending in a slash
	// (empty for the checkout root): a file created under one can join the
	// package's embedded files.
	embeds []string
}

func newQueryInputs() queryInputs {
	return queryInputs{files: make(map[string]bool), linted: make(map[string]bool), deps: make(map[string]bool)}
}

// addPackage records one listed package's directory, relative to the checkout
// in slash form, and the trees its embed patterns can select.
func (q *queryInputs) addPackage(dir string, direct bool, embedPatterns []string) {
	if direct {
		q.linted[dir] = true
	} else {
		q.deps[dir] = true
	}
	for _, pattern := range embedPatterns {
		pattern = strings.TrimPrefix(pattern, "all:")
		base := pattern
		if meta := strings.IndexAny(pattern, `*?[\`); meta >= 0 {
			base = pattern[:meta]
			base = base[:strings.LastIndex(base, "/")+1]
		}
		q.embeds = append(q.embeds, treePrefix(path.Join(dir, base)))
	}
}

// checkoutDir answers the slash-form path of dir relative to root, and false
// for a directory outside the checkout (the module cache, GOROOT).
func checkoutDir(root, dir string) (string, bool) {
	relative, err := filepath.Rel(root, filepath.Clean(dir))
	if err != nil {
		return "", false
	}
	relative = filepath.ToSlash(relative)
	if relative == ".." {
		return "", false
	}
	if strings.HasPrefix(relative, "../") {
		return "", false
	}
	return relative, true
}

// treePrefix spells a directory as the prefix every path inside it starts
// with. The checkout root is the empty prefix, which every path starts with.
func treePrefix(dir string) string {
	if dir == "." {
		return ""
	}
	return dir + "/"
}

// lintPopulation answers which checkout paths can change one run's verdict.
// It grows as each flavor's go list names more of it, so a boundary judges
// every path some query so far has read.
//
// It is not safe for concurrent use.
type lintPopulation struct {
	// everything is set by a pattern that reaches the whole checkout: ./...,
	// or a wildcard this planner cannot bound to one tree. Such a run reads
	// every path the label reads.
	everything bool
	// trees are the directories a ./<dir>/... pattern reaches, each ending in a
	// slash: a package created under one during planning joins the run.
	trees []string
	// queried is everything the flavor queries so far read, and the
	// directories the patterns name.
	queried queryInputs
}

// newLintPopulation starts the population a set of package patterns reaches
// before any query has listed a package.
//
// A pattern without a wildcard names packages go list resolves, so the query
// adds their directories; one spelled from the checkout root also names its
// directory now. A ./<dir>/... wildcard reaches its tree. Any other wildcard
// (./..., one not spelled from the root, one inside a name such as
// ./internal/le/go...) reaches a tree this planner cannot bound below the
// whole checkout, so it reaches everything the label reads.
func newLintPopulation(patterns []string) lintPopulation {
	population := lintPopulation{queried: newQueryInputs()}
	for _, pattern := range patterns {
		relative, fromRoot := strings.CutPrefix(pattern, "./")
		if pattern == "." {
			relative, fromRoot = ".", true
		}
		if !strings.Contains(relative, "...") {
			if fromRoot {
				population.queried.linted[path.Clean(relative)] = true
			}
			continue
		}
		base, tree := strings.CutSuffix(relative, "/...")
		population.everything = population.everything || !fromRoot || !tree || strings.Contains(base, "...")
		if population.everything {
			continue
		}
		population.trees = append(population.trees, treePrefix(path.Clean(base)))
	}
	return population
}

// add records one query's reads.
func (p *lintPopulation) add(read queryInputs) {
	for rel := range read.files {
		p.queried.files[rel] = true
	}
	for dir := range read.linted {
		p.queried.linted[dir] = true
	}
	for dir := range read.deps {
		p.queried.deps[dir] = true
	}
	p.queried.embeds = append(p.queried.embeds, read.embeds...)
}

// reaches answers whether a change to rel can change this run's verdict.
//
// A file a query read always counts, and so does any file under an embed
// tree. A file a query did not read counts only when go list could start
// selecting it: a package source file, or a go.mod, in a linted directory, in
// a dependency's directory (where a _test.go never compiles, because -test
// builds tests for the linted packages only), or under a pattern's tree.
func (p *lintPopulation) reaches(rel string) bool {
	if p.everything {
		return true
	}
	if p.queried.files[rel] {
		return true
	}
	if underAny(rel, p.queried.embeds) {
		return true
	}
	if buildSource(rel) {
		dir := path.Dir(rel)
		if p.queried.linted[dir] {
			return true
		}
		if p.queried.deps[dir] && !strings.HasSuffix(rel, "_test.go") {
			return true
		}
		if underAny(rel, p.trees) {
			return true
		}
	}
	for _, input := range toolInputs {
		if rel == input {
			return true
		}
		if strings.HasSuffix(input, "/") && strings.HasPrefix(rel, input) {
			return true
		}
	}
	return false
}

// buildSource answers whether go/build reads a file of this name as package
// source, or as the module boundary that decides which package it is in.
func buildSource(rel string) bool {
	name := path.Base(rel)
	if name == "go.mod" {
		return true
	}
	return slices.Contains(sourceExtensions, path.Ext(name))
}

// underAny answers whether rel sits under one of the trees.
func underAny(rel string, trees []string) bool {
	for _, tree := range trees {
		if strings.HasPrefix(rel, tree) {
			return true
		}
	}
	return false
}
