// Design: docs/architecture/testing/verify-freshness-scope.md -- what a scoped run covers
// Related: actions.go -- the verbs that reach this selection
// Related: scope.go -- the scoped-verify question this area also answers
// Related: report.go -- the two renderings of one selection
//
// Package changed answers "what did I edit" for two callers. The changed-group
// race pass uses the answer to size its test run. Every scoped verify stage
// does the same.
//
// The answer is a GUARD that fails closed. A run can fail to read the checkout
// or resolve a package directory. In either case, it MUST NOT answer "nothing
// changed." Both callers treat an empty answer as permission to run no tests
// and report success. internal/le/changed/actions.go currently returns that
// answer when git or `go list` fails. This port closes that defect
// (plan/journal/zero-value-as-valid-answer.md, 2026-08-26).
package changed

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the word this command is typed as, and the first word of every verb
// under it.
const area = "changed"

// gitCommand is the program every query in this file starts.
const gitCommand = "git"

// goFiles is the pathspec every git query is bounded by. Both callers size a GO
// test run, so a changed rule page or a changed .ci is not their business.
const goFiles = "*.go"

// restGroup is the name the group listing prints when a changed file belongs to
// no mapped group.
const restGroup = "rest"

// relativePrefix is what a package directory is spelled with, so `go test` and
// `golangci-lint` read it as a directory rather than as a module path.
const relativePrefix = "./"

// Group is one mapped test group. It contains the short name the native unit
// action reports and the Go package pattern that name represents.
type Group struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

// groups maps a directory prefix to the test group that owns it.
//
// FIRST MATCH WINS, but the answer must not depend on match order. NO PREFIX
// HERE MAY SIT INSIDE ANOTHER. TestNoGroupPrefixSitsInsideAnother enforces this
// rule. A group whose prefix is a subtree of another group can only NARROW the
// parent pattern. Thus, it removes tests instead of selecting better tests.
//
// The shell half declares such a row. It maps internal/component/l2tp/ppp/ to a
// `ppp` group under a comment that says "first match wins (most specific
// first)." It iterates a bash associative array in hash order. That order puts
// internal/component/l2tp/ first, so the ppp row is unreachable.
//
// If reachable, the row would reduce a ppp change's run from all of l2tp to
// only ppp. No Make
// target named a ppp group. Therefore, the port drops the row instead of
// reordering it (plan/journal/gate-excludes-part-of-its-population.md,
// 2026-08-26).
var groups = []struct {
	Prefix string
	Group  Group
}{
	{"internal/component/l2tp/", Group{Name: "l2tp", Pattern: "./internal/component/l2tp/..."}},
	{"internal/component/bgp/", Group{Name: "bgp", Pattern: "./internal/component/bgp/..."}},
	{"internal/component/config/", Group{Name: "config", Pattern: "./internal/component/config/..."}},
	{"internal/component/cli/", Group{Name: "cli", Pattern: "./internal/component/cli/..."}},
	{"internal/component/web/", Group{Name: "web", Pattern: "./internal/component/web/..."}},
	{"internal/component/api/", Group{Name: "api", Pattern: "./internal/component/api/..."}},
	{"internal/core/", Group{Name: "core", Pattern: "./internal/core/..."}},
	{"internal/plugins/", Group{Name: "plugins", Pattern: "./internal/plugins/..."}},
	{"internal/test/", Group{Name: "test", Pattern: "./internal/test/..."}},
	{"cmd/", Group{Name: "cmd", Pattern: "./cmd/..."}},
}

// groupOf answers the group a changed path belongs to, and whether one claimed
// it. A path no prefix claims is a package of its own.
func groupOf(file string) (Group, bool) {
	for _, entry := range groups {
		if strings.HasPrefix(file, entry.Prefix) {
			return entry.Group, true
		}
	}
	return Group{}, false
}

// ErrNoCommand says this area was asked to run an empty command line.
var ErrNoCommand = errors.New("changed: no command to run")

// Run defines how this area runs one command. A field replaces a direct call so
// that a test can drive fail-closed paths. The test does not need a checkout
// that fails in a specific way.
type Run func(dir string, argv []string) (string, error)

// RunCommand runs argv in dir and answers its stdout.
//
// RunCommand uses context.Background with no deadline. A git query over a local
// checkout waits on neither the network nor a lock that this process holds. A
// deadline would describe a slow filesystem as no changes. This package exists
// to prevent that failure.
func RunCommand(dir string, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", ErrNoCommand
	}
	//nolint:gosec // argv is this package's own table; le is a build-host tool
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Selection is what changed, in the two shapes its callers size a run by.
//
// Groups are the mapped test groups in table order. Rest contains the package
// directory of each changed file that no group claimed. The directories have
// an `./` prefix and are sorted. One payload carries both lists because they
// describe one fact. A caller that selects one list chooses a rendering, not a
// second question.
type Selection struct {
	Groups []Group  `json:"groups"`
	Rest   []string `json:"rest-packages"`
}

// Empty reports whether nothing changed. Stated once, so no caller has to
// decide for itself what an empty selection looks like.
func (s Selection) Empty() bool { return len(s.Groups) == 0 && len(s.Rest) == 0 }

// grouping resolves a file list against the group table. It contains the
// matched groups and the package directories that no group claimed. The
// toolchain must still determine which directories contain a package. Thus,
// grouping is not yet a Selection.
type grouping struct {
	Groups   []Group
	unmapped []string
}

// groupFiles maps a changed file list onto the group table.
//
// The groups appear in TABLE order instead of file order. Thus, two runs over
// one file set answer the same bytes.
func groupFiles(files []string) grouping {
	hit := make(map[string]bool, len(groups))
	dirs := make(map[string]bool)
	for _, file := range files {
		group, mapped := groupOf(file)
		if mapped {
			hit[group.Name] = true
			continue
		}
		var tb textbuf.Buffer
		dirs[tb.Str(relativePrefix).Str(path.Dir(file)).String()] = true
	}

	var result grouping
	for _, entry := range groups {
		if hit[entry.Group.Name] {
			result.Groups = append(result.Groups, entry.Group)
		}
	}
	result.unmapped = make([]string, 0, len(dirs))
	for dir := range dirs {
		result.unmapped = append(result.unmapped, dir)
	}
	sort.Strings(result.unmapped)
	return result
}

// Selector answers what changed in one checkout.
type Selector struct {
	// Root is the checkout the answer is about.
	Root string
	// Run runs one command. The zero value means RunCommand.
	Run Run
}

// run answers through the selector's command runner, defaulted.
func (s Selector) run(dir string, argv []string) (string, error) {
	if s.Run == nil {
		return RunCommand(dir, argv)
	}
	return s.Run(dir, argv)
}

// ChangedFiles answers every .go file that differs from HEAD. It includes
// unstaged changes, staged changes, and untracked files that git does not
// ignore.
//
// EVERY query must succeed. A git failure is an error, not an empty file list.
// The caller treats an empty list as "no test to run" and reports success.
func (s Selector) ChangedFiles() ([]string, error) {
	queries := [][]string{
		{gitCommand, "diff", "--name-only", "--", goFiles},
		{gitCommand, "diff", "--cached", "--name-only", "--", goFiles},
		{gitCommand, "ls-files", "--others", "--exclude-standard", "--", goFiles},
	}

	seen := make(map[string]bool)
	for _, argv := range queries {
		out, err := s.run(s.Root, argv)
		if err != nil {
			return nil, err
		}
		for line := range strings.SplitSeq(out, "\n") {
			file := strings.TrimSpace(line)
			if file != "" {
				seen[file] = true
			}
		}
	}

	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}

// Packages answers which dirs the toolchain recognizes as buildable packages.
// It returns the directories relative to the checkout with an `./` prefix.
//
// It DROPS a directory whose only Go files use `//go:build ignore` because `go
// test` refuses that directory with "build constraints exclude all Go files".
// A toolchain failure is an ERROR. Otherwise, a broken module would produce a
// test run over nothing.
func (s Selector) Packages(dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	argv := make([]string, 0, len(dirs)+4)
	argv = append(argv, "go", "list", "-e", "-f", "{{if not .Error}}{{.Dir}}{{end}}")
	argv = append(argv, dirs...)

	out, err := s.run(s.Root, argv)
	if err != nil {
		return nil, err
	}

	found := make(map[string]bool, len(dirs))
	for line := range strings.SplitSeq(out, "\n") {
		dir := strings.TrimSpace(line)
		if dir == "" {
			continue
		}
		relative, inside := s.relative(dir)
		if inside {
			found[relative] = true
		}
	}

	packages := make([]string, 0, len(found))
	for pkg := range found {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages, nil
}

// relative spells an absolute package directory as a path of this checkout, and
// reports whether it is inside it at all. A directory outside the root is not
// this run's business, which is what the shell's awk guard says too.
func (s Selector) relative(dir string) (string, bool) {
	if dir == s.Root {
		return ".", true
	}
	var tb textbuf.Buffer
	prefix := tb.Str(s.Root).Byte('/').String()
	if !strings.HasPrefix(dir, prefix) {
		return "", false
	}
	var out textbuf.Buffer
	return out.Str(relativePrefix).Str(strings.TrimPrefix(dir, prefix)).String(), true
}

// Select answers the whole selection for this checkout.
func (s Selector) Select() (Selection, error) {
	files, err := s.ChangedFiles()
	if err != nil {
		return Selection{}, err
	}

	grouped := groupFiles(files)
	rest, err := s.Packages(grouped.unmapped)
	if err != nil {
		return Selection{}, err
	}
	return Selection{Groups: grouped.Groups, Rest: rest}, nil
}
