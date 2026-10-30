// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Overview: check_evidence.go -- (*evidenceReader).layerPathGap, which holds go: and vendor/ paths to this set
// Related: ../../test/buildflavor/buildflavor.go -- Shipped, the one reader of the shipped flavors
// Related: ../repo/featuretags/daemontags.go -- DaemonTags, the feature gates every shipped flavor carries
package rfc

import (
	"bytes"
	"os"
	"os/exec"
	"slices"
	"strings"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	"github.com/ze-software/ze/internal/test/buildflavor"
)

// shippedMain is the package every shipped flavor of ze is built from.
const shippedMain = "./cmd/ze"

// linkedListFormat is the `go list -f` template: "std" or the module path,
// then the import path, one package per line.
const linkedListFormat = `{{if .Standard}}std{{else}}{{with .Module}}{{.Path}}{{end}}{{end}} {{.ImportPath}}`

// linkedStandard marks a standard library package in linkedListFormat's
// output.
const linkedStandard = "std"

// linkedPackages is what the shipped ze links, as the union over every
// shipped flavor: the standard library import paths, and the module paths
// of every other package. A {lower-layer} path names a component Ze relies
// on, and a component the binary does not link performs nothing for Ze.
type linkedPackages struct {
	standard map[string]bool
	modules  map[string]bool
}

// goListLinked asks the toolchain what each shipped flavor of ze links:
// one `go list -deps` over shippedMain per flavor, with the flavor's tags
// and every feature gate, for linux with cgo off as the appliance is built.
// Any failure is a refusal: a package cannot be told unlinked from a list
// nobody read.
func goListLinked(tree string) (linkedPackages, string) {
	flavors, refusal := shippedFlavors(tree)
	if refusal != "" {
		return linkedPackages{}, refusal
	}
	gates, err := repofeaturetags.DaemonTags(tree)
	if err != nil {
		return linkedPackages{}, "cannot read the feature gates (" + err.Error() +
			"), so no go: or vendor/ layer path can be checked against what ze links; restore feature-gates.txt from git"
	}
	linked := linkedPackages{standard: map[string]bool{}, modules: map[string]bool{}}
	for _, tags := range flavors {
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

// shippedFlavors answers the tags each shipped flavor requires true, read by
// buildflavor.Shipped, the parser TestNormalZeLinksNoInternalLe reads them
// with. No flavor file, or one whose constraint is not a conjunction of tags
// and negations its own tags satisfy, is a refusal: the set of flavors cannot
// be told from it.
func shippedFlavors(tree string) ([][]string, string) {
	shipped, err := buildflavor.Shipped(tree)
	if err != nil {
		// The error names the file, or the glob when none matches, and its one
		// fix; this adds only what the refusal costs.
		return nil, "the shipped flavors cannot be read, so what ze links cannot be listed: " + err.Error()
	}
	flavors := make([][]string, 0, len(shipped))
	for _, flavor := range shipped {
		flavors = append(flavors, flavor.Tags)
	}
	return flavors, ""
}
