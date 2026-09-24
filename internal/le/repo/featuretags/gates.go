// Design: ai/rules/principles.md -- feature-gates.txt is declared once and
// every surface derives from it
// Related: daemontags.go -- the tag list this row parse feeds
//
// gates.go holds the manifest PARSE. Everything else derives from Gates.
//
// Six surfaces each carried their own walk over this one file. The four
// generated tag lists, the daemon tag set, the change selector's package map,
// the plugin-imports generator, the tier gate, the staticcheck matrix. A copy
// that nothing compares is what lets one build carry a feature another does
// not.

package repofeaturetags

import (
	"errors"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Gate is one row of feature-gates.txt: a build tag, and the package that tag
// gates. A tag appears on as many rows as it has packages.
type Gate struct {
	Tag     string
	Package string
}

// packagePathPattern accepts a relative Go import path. The manifest's second
// column is a package inside this module, so it never carries a host name, a
// leading slash, or a dot segment.
var packagePathPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)

// Gates answers every row of the manifest under root, in the order the file
// declares them.
//
// A line the reader cannot understand is an ERROR rather than a skipped row.
// A skipped row leaves the package it names ungated in every consumer at once.
// The linker keeps it, the tier gate finds no violation for it, and the change
// selector never widens for it, all without one message.
//
// An EMPTY manifest is not an error here. The caller decides whether "no gate
// is declared" is permitted. It is normal for a tree that gates nothing. It is
// fatal for a build, which is why DaemonTags refuses it.
func Gates(root string) ([]Gate, error) {
	path := filepath.Join(root, manifestFile)

	raw, err := os.ReadFile(path) //nolint:gosec // a build tool reads the checkout it was pointed at
	if err != nil {
		return nil, err
	}

	var gates []Gate
	for index, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, malformed(path, index+1, "want \"<tag> <package>\"")
		}
		if !validPackagePath(fields[1]) {
			return nil, malformed(path, index+1, "the package is not a clean relative import path")
		}

		gates = append(gates, Gate{Tag: fields[0], Package: fields[1]})
	}

	return gates, nil
}

// malformed names the file, the line and what the line owes.
func malformed(path string, line int, want string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(path).Byte(':').Int(int64(line)).Str(": ").Str(want).String())
}

// validPackagePath reports whether a manifest's second field is a clean
// relative Go import path.
func validPackagePath(packagePath string) bool {
	return packagePathPattern.MatchString(packagePath) &&
		pathpkg.Clean(packagePath) == packagePath &&
		!strings.Contains(packagePath, "//")
}
