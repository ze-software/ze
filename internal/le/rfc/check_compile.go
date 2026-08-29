// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Related: check.go -- the centralized check driver that treats compilation as evidence admissibility
//
// check_compile.go type-checks every Go package that carries an RFC tag. A tag
// in a package the compiler rejects is not evidence because no test can run.
package rfc

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

const quotedCompilerMessages = 5
const vetTimeout = 15 * time.Minute

// buildTags answers the `-tags` value the type-check compiles with.
//
// gotoolchain reads feature-gates.txt and refuses an empty gate set, so a
// manifest this check cannot classify stops the run. A reduced tag set would
// drop every gated file out of the type-check, which then reports clean over
// code it never read.
func buildTags(tree string) (string, error) {
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return "", baselineParseError("derive native Go test tags: " + err.Error())
	}
	return toolchain.TestTags(), nil
}

func modulePath(tree string) (string, error) {
	const rel = "go.mod"
	var tb textbuf.Buffer
	raw, err := os.ReadFile(treePath(tree, rel)) // #nosec G304 -- the module under the checkout
	if err != nil {
		return "", baselineParseError(tb.Str(rel).Str(": cannot read the module declaration: ").Err(err).String())
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", baselineParseError(tb.Reset().Str(rel).Str(": no `module <path>` line found, so a `go vet` package header cannot be resolved to a directory").String())
}

func goTagPackages(tree string, tags []Tag, carriers []Carrier) []string {
	dirs := map[string]bool{}
	var tb textbuf.Buffer
	for _, tag := range tags {
		carrier, held := carrierFor(tag.File, carriers)
		if !held || carrier.Reader != "go" {
			continue
		}
		underRoot := false
		for _, root := range testRoots {
			if strings.HasPrefix(tag.File, tb.Reset().Str(root).Byte('/').Slice()) {
				underRoot = true
			}
		}
		if !underRoot {
			continue
		}
		if _, err := os.Stat(treePath(tree, tag.File)); err != nil {
			continue
		}
		dirs[filepath.ToSlash(filepath.Dir(tag.File))] = true
	}
	out := sortedSet(dirs)
	for index, dir := range out {
		if dir == "." {
			continue
		}
		out[index] = tb.Reset().Str("./").Str(dir).String()
	}
	return out
}

func vetFailures(text, module string) map[string][]string {
	out := map[string][]string{}
	current := ""
	var tb textbuf.Buffer
	modulePrefix := tb.Str(module).Byte('/').String()
	for line := range strings.SplitSeq(text, "\n") {
		if body, found := strings.CutPrefix(line, "#"); found {
			parts := strings.Fields(body)
			if len(parts) == 0 {
				continue
			}
			pkg := strings.Trim(parts[0], "[]")
			switch {
			case pkg == module:
				pkg = "."
			case strings.HasPrefix(pkg, modulePrefix):
				pkg = strings.TrimPrefix(pkg, modulePrefix)
			}
			current = pkg
			if _, held := out[current]; !held {
				out[current] = nil
			}
			continue
		}
		message := strings.TrimSpace(line)
		if message == "" || current == "" {
			continue
		}
		message = strings.TrimPrefix(message, "vet: ")
		out[current] = append(out[current], message)
	}
	return out
}

func quoteCompiler(messages []string) string {
	if len(messages) == 0 {
		return "nothing"
	}
	shown := messages
	if len(shown) > quotedCompilerMessages {
		shown = shown[:quotedCompilerMessages]
	}
	text := strings.Join(shown, " / ")
	if len(messages) > quotedCompilerMessages {
		var tb textbuf.Buffer
		return tb.Str(text).Str(" (and ").Int(int64(len(messages) - quotedCompilerMessages)).Str(" more)").String()
	}
	return text
}

func checkTagPackagesCompile(tree string, tags []Tag, carriers []Carrier) ([]string, error) {
	packages := goTagPackages(tree, tags, carriers)
	if len(packages) == 0 {
		return nil, nil
	}
	tagsArg, err := buildTags(tree)
	if err != nil {
		return nil, err
	}
	args := append([]string{"vet", "-framepointer", "-tags", tagsArg}, packages...)
	ctx, cancel := context.WithTimeout(context.Background(), vetTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...) //nolint:gosec // package paths are derived from files in the checkout
	cmd.Dir = tree
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr == nil {
		return nil, nil
	}
	if ctx.Err() != nil {
		var tb textbuf.Buffer
		return nil, baselineParseError(tb.Str("cannot run `go vet` over the ").Int(int64(len(packages))).Str(" package(s) that hold RFC requirement tags, so whether those tests compile is unknown: ").Err(ctx.Err()).Str(". A tag is evidence only when the test can run, and an unmeasured answer is the one thing this gate must never report as clean. Install a Go toolchain, or run this from the repository root").String())
	}
	module, err := modulePath(tree)
	if err != nil {
		return nil, err
	}
	text := stdout.String() + stderr.String()
	failures := vetFailures(text, module)
	if len(failures) == 0 {
		var raw []string
		for line := range strings.SplitSeq(text, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				raw = append(raw, line)
			}
		}
		var tb textbuf.Buffer
		return nil, baselineParseError(tb.Str("`go vet` failed over the ").Int(int64(len(packages))).Str(" package(s) that hold RFC requirement tags without naming a package, so whether those tests compile is unknown. go vet said: ").Str(quoteCompiler(raw)).String())
	}
	held := map[string][]Tag{}
	for _, tag := range tags {
		carrier, found := carrierFor(tag.File, carriers)
		if found && carrier.Reader == "go" {
			held[filepath.ToSlash(filepath.Dir(tag.File))] = append(held[filepath.ToSlash(filepath.Dir(tag.File))], tag)
		}
	}
	var errs []string
	for _, pkg := range sortedKeysOf(failures) {
		rids := map[string]bool{}
		for _, tag := range held[pkg] {
			rids[tag.RID] = true
		}
		stake := "and a package holding RFC requirement tags depends on it, so those tests cannot run either"
		if len(rids) > 0 {
			list := sortedSet(rids)
			shown := list
			if len(shown) > 4 {
				shown = shown[:4]
			}
			text := strings.Join(shown, ", ")
			if len(list) > 4 {
				text += ", ..."
			}
			var tb textbuf.Buffer
			stake = tb.Str("so the ").Int(int64(len(list))).Str(" RFC requirement(s) tagged in it are not evidence: no test here can run. Tagged here: ").Str(text).String()
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(pkg).Str(": `go vet` cannot type-check this package, ").Str(stake).Str(". go vet said: ").Str(quoteCompiler(failures[pkg])).Str(". Fix the package so `./le verify-deps unit-cached` compiles it, then re-run `./le rfc check`").String())
	}
	sort.Strings(errs)
	return errs, nil
}
