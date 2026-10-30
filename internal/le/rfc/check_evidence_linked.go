// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Overview: check_evidence.go -- (*evidenceReader).layerPathGap, which holds go: and vendor/ paths to this set
// Related: cmd/ze/build_tag_appliance_test.go -- one of the shipped flavour files whose //go:build lines name the flavours
// Related: ../repo/featuretags/daemontags.go -- DaemonTags, the feature gates every shipped flavour carries
package rfc

import (
	"bytes"
	"go/build/constraint"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// shippedMain is the package every shipped flavour of ze is built from.
const shippedMain = "./cmd/ze"

// shippedFlavourGlob matches the files whose //go:build line declares one
// shipped flavour each: the tags it requires true are that flavour's build
// tags, as TestNormalZeLinksNoInternalLe reads them.
const shippedFlavourGlob = "cmd/ze/build_tag_*_test.go"

// linkedListFormat is the `go list -f` template: "std" or the module path,
// then the import path, one package per line.
const linkedListFormat = `{{if .Standard}}std{{else}}{{with .Module}}{{.Path}}{{end}}{{end}} {{.ImportPath}}`

// linkedStandard marks a standard library package in linkedListFormat's
// output.
const linkedStandard = "std"

// linkedPackages is what the shipped ze links, as the union over every
// shipped flavour: the standard library import paths, and the module paths
// of every other package. A {lower-layer} path names a component Ze relies
// on, and a component the binary does not link performs nothing for Ze.
type linkedPackages struct {
	standard map[string]bool
	modules  map[string]bool
}

// goListLinked asks the toolchain what each shipped flavour of ze links:
// one `go list -deps` over shippedMain per flavour, with the flavour's tags
// and every feature gate, for linux with cgo off as the appliance is built.
// Any failure is a refusal: a package cannot be told unlinked from a list
// nobody read.
func goListLinked(tree string) (linkedPackages, string) {
	flavours, refusal := shippedFlavours(tree)
	if refusal != "" {
		return linkedPackages{}, refusal
	}
	gates, err := repofeaturetags.DaemonTags(tree)
	if err != nil {
		return linkedPackages{}, "cannot read the feature gates (" + err.Error() +
			"), so no go: or vendor/ layer path can be checked against what ze links; restore feature-gates.txt from git"
	}
	linked := linkedPackages{standard: map[string]bool{}, modules: map[string]bool{}}
	for _, tags := range flavours {
		tagList := strings.Join(append(slices.Clone(tags), gates...), ",")
		cmd := exec.Command("go", "list", "-deps", "-tags", tagList, "-f", linkedListFormat, shippedMain) //nolint:gosec,noctx // this developer tool asks its own toolchain what the shipped binary links
		cmd.Dir = tree
		cmd.Env = append(os.Environ(), "GOOS=linux", "CGO_ENABLED=0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return linkedPackages{}, "`go list -deps -tags " + tagList + " " + shippedMain + "` failed (" +
				strings.TrimSpace(err.Error()+" "+stderr.String()) +
				"), so no go: or vendor/ layer path can be checked against what ze links; make that command answer, then rerun `./le rfc check`"
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			owner, pkg, found := strings.Cut(strings.TrimSpace(line), " ")
			if !found {
				continue
			}
			if owner == linkedStandard {
				linked.standard[pkg] = true
				continue
			}
			linked.modules[owner] = true
		}
	}
	return linked, ""
}

// shippedFlavours answers the tags each shipped flavour file requires true.
// No flavour file, or one whose constraint is not a conjunction of tags and
// negations, is a refusal: the set of flavours cannot be told from it.
func shippedFlavours(tree string) ([][]string, string) {
	files, err := filepath.Glob(filepath.Join(tree, filepath.FromSlash(shippedFlavourGlob)))
	if err != nil {
		return nil, "cannot list " + shippedFlavourGlob + " (" + err.Error() + "); restore cmd/ze from git"
	}
	if len(files) == 0 {
		return nil, "no " + shippedFlavourGlob + " file declares a shipped flavour, so what ze links cannot be listed; restore cmd/ze from git"
	}
	flavours := make([][]string, 0, len(files))
	for _, file := range files {
		tags, refusal := flavourTags(tree, file)
		if refusal != "" {
			return nil, refusal
		}
		flavours = append(flavours, tags)
	}
	return flavours, ""
}

// flavourTags answers the tags one flavour file's //go:build line requires
// true.
func flavourTags(tree, file string) ([]string, string) {
	rel := relTo(tree, file)
	body, err := os.ReadFile(file) //nolint:gosec // a flavour file of this checkout, found by glob
	if err != nil {
		return nil, "cannot read " + rel + " (" + err.Error() + "); restore it from git"
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			return nil, rel + ": its //go:build line does not parse (" + err.Error() + "); correct it"
		}
		var tags []string
		if !positiveTags(expr, &tags) {
			return nil, rel + ": its //go:build line is not a conjunction of tags and negations, so its flavour's tags cannot be read; write it as one"
		}
		return tags, ""
	}
	return nil, rel + " carries no //go:build line, so its flavour is unknown; add the line naming the flavour's tags"
}

// positiveTags appends the tags expr requires true, false when expr is not a
// conjunction of tags and negations. Recursion is bounded by the one
// //go:build line it reads.
func positiveTags(expr constraint.Expr, tags *[]string) bool {
	switch e := expr.(type) {
	case *constraint.AndExpr:
		return positiveTags(e.X, tags) && positiveTags(e.Y, tags)
	case *constraint.TagExpr:
		*tags = append(*tags, e.Tag)
		return true
	case *constraint.NotExpr:
		return true
	default:
		return false
	}
}
