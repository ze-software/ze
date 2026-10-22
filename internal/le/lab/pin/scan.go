// Design: docs/architecture/testing/interop.md -- lab peer pins and their upstream
// Overview: actions.go -- the check that compares these pins with upstream.
//
// scan.go derives the pin population from the files that carry the pins. No
// list of peers exists here: a new lab image, a new `go install` or a new
// source-built peer is found because the file that builds or pulls it names it.

package pin

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// Kind says which upstream answers a pin.
type Kind uint8

const (
	// KindUnspecified is never answered by Scan.
	KindUnspecified Kind = iota
	// KindImage is a container image tag, answered by its registry's tag list.
	KindImage
	// KindGoModule is a `go install path@version`, answered by proxy.golang.org.
	KindGoModule
	// KindGitTag is a GitHub tag a build clones or downloads, answered by the
	// repository's latest release.
	KindGitTag
	// KindSourceCommit is a source-built peer's pinned commit, answered by the
	// repository's latest default-branch commit.
	KindSourceCommit
)

// String is the kind's name in a report row.
func (k Kind) String() string {
	switch k {
	case KindImage:
		return "image"
	case KindGoModule:
		return "go-module"
	case KindGitTag:
		return "git-tag"
	case KindSourceCommit:
		return "source-commit"
	case KindUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

// Pin is one pinned version as the tree states it.
type Pin struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Kind    Kind   `json:"-"`
	Name    string `json:"name"`
	Version string `json:"pinned"`
}

// Scanned file shapes. Dockerfiles carry every kind; Go sources and the
// scenario `*-image` files carry image references in string literals.
var (
	imageLiteral = regexp.MustCompile(`"((?:[a-z0-9-]+(?:\.[a-z0-9-]+)+(?::[0-9]+)?/)?[a-z0-9._-]+(?:/[a-z0-9._-]+)+):([A-Za-z0-9._-]+)(?:@sha256:[0-9a-f]{64})?"`)
	imageBare    = regexp.MustCompile(`^((?:[a-z0-9-]+(?:\.[a-z0-9-]+)+(?::[0-9]+)?/)?[a-z0-9._-]+(?:/[a-z0-9._-]+)*):([A-Za-z0-9._-]+)(?:@sha256:[0-9a-f]{64})?$`)
	goInstall    = regexp.MustCompile(`go install\s+(?:-[^\s]+\s+)*([a-z0-9.-]+\.[a-z]+/[^\s@]+)@(v[0-9][^\s;&|]*)`)
	gitBranch    = regexp.MustCompile(`git clone[^\n]*--branch[ =]+([^\s]+)[^\n]*?https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:\.git)?(?:\s|$)`)
	codeloadTag  = regexp.MustCompile(`https://(?:codeload\.)?github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)/(?:tar\.gz|archive)/(?:refs/tags/)?(v?[0-9]+\.[0-9][A-Za-z0-9._-]*?)(?:\.tar\.gz)?(?:["\s]|$)`)
)

// skippedDirectories are never walked: vendored code pins nothing a lab runs,
// and a testdata tree holds fixtures, including this package's own.
var skippedDirectories = []string{"vendor", "testdata", "node_modules", ".git"}

// Scan answers every pin under test/ and internal/ of the tree, in file order.
func Scan(tree string) ([]Pin, error) {
	var pins []Pin
	for _, root := range []string{"test", "internal"} {
		err := filepath.WalkDir(filepath.Join(tree, root), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if slices.Contains(skippedDirectories, entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			reader := readerOf(entry.Name())
			if reader == nil {
				return nil
			}
			body, err := os.ReadFile(path) //nolint:gosec // a file the walk found under the checkout
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(tree, path)
			if err != nil {
				return err
			}
			pins = append(pins, reader(filepath.ToSlash(relative), string(body))...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return pins, nil
}

// readerOf answers the reader for a file name, or nil for a file that
// carries no pin this check reads.
func readerOf(name string) func(string, string) []Pin {
	if name == "Dockerfile" || strings.HasPrefix(name, "Dockerfile.") {
		return dockerfilePins
	}
	if strings.HasSuffix(name, ".go") {
		return literalPins
	}
	if strings.HasSuffix(name, "-image") {
		return barePins
	}
	return nil
}

// dockerfilePins reads FROM images, go install versions, cloned and
// downloaded tags, and the source pin of a source-built peer.
func dockerfilePins(file, body string) []Pin {
	var pins []Pin
	stages := map[string]bool{}
	for index, line := range strings.Split(body, "\n") {
		number := index + 1
		words := strings.Fields(line)
		if len(words) >= 2 && strings.EqualFold(words[0], "FROM") {
			image := fromImage(words[1:])
			if len(words) >= 4 && strings.EqualFold(words[len(words)-2], "AS") {
				stages[strings.ToLower(words[len(words)-1])] = true
			}
			if image == "" || stages[strings.ToLower(image)] || strings.Contains(image, "$") {
				continue
			}
			if match := imageBare.FindStringSubmatch(image); match != nil {
				pins = append(pins, Pin{File: file, Line: number, Kind: KindImage, Name: match[1], Version: match[2]})
			}
			continue
		}
		for _, match := range goInstall.FindAllStringSubmatch(line, -1) {
			pins = append(pins, Pin{File: file, Line: number, Kind: KindGoModule, Name: match[1], Version: match[2]})
		}
		for _, match := range gitBranch.FindAllStringSubmatch(line, -1) {
			pins = append(pins, Pin{File: file, Line: number, Kind: KindGitTag, Name: match[2], Version: match[1]})
		}
		for _, match := range codeloadTag.FindAllStringSubmatch(line, -1) {
			pins = append(pins, Pin{File: file, Line: number, Kind: KindGitTag, Name: match[1], Version: match[2]})
		}
	}
	if source, declared, err := interoplab.ParseSourcePin(file, body); declared && err == nil && source.Commit != "" {
		pins = append(pins, Pin{File: file, Line: lineOf(body, "ARG "+interoplab.SourceCommitArg+"="), Kind: KindSourceCommit,
			Name: source.Repository, Version: source.Commit})
	}
	return pins
}

// fromImage answers the image a FROM names, past its flags.
func fromImage(words []string) string {
	for _, word := range words {
		if strings.HasPrefix(word, "--") {
			continue
		}
		if word == "scratch" {
			return ""
		}
		return word
	}
	return ""
}

// literalPins reads image references out of Go string literals. A reference
// needs a repository path (a slash), so a local `ze-*:test` tag is not read.
func literalPins(file, body string) []Pin {
	var pins []Pin
	for index, line := range strings.Split(body, "\n") {
		for _, match := range imageLiteral.FindAllStringSubmatch(line, -1) {
			if !plausibleImage(match[1], match[2]) {
				continue
			}
			pins = append(pins, Pin{File: file, Line: index + 1, Kind: KindImage, Name: match[1], Version: match[2]})
		}
	}
	return pins
}

// literalTag is the tag of an image a Go literal names: a version or latest.
// A literal like "docs/page.md:4" or "../dir:x" has the image shape and is a
// path with a line or a label, never an image.
var literalTag = regexp.MustCompile(`^(?:v?\d[\w.-]*|latest)$`)

// fileSuffix ends a path segment that names a file rather than a repository.
var fileSuffix = regexp.MustCompile(`\.(?:go|md|txt|json|ya?ml|toml|conf|ci)$`)

// plausibleImage reports whether a Go literal's name and tag read as an image
// reference: a name that starts with a letter, whose last segment is not a
// file, and a tag that is a version or latest.
func plausibleImage(name, tag string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	if fileSuffix.MatchString(name) {
		return false
	}
	return literalTag.MatchString(tag)
}

// barePins reads a file that holds one image reference, such as a scenario's
// frr-image.
func barePins(file, body string) []Pin {
	reference := strings.TrimSpace(body)
	match := imageBare.FindStringSubmatch(reference)
	if match == nil {
		return nil
	}
	return []Pin{{File: file, Line: 1, Kind: KindImage, Name: match[1], Version: match[2]}}
}

// lineOf answers the one-based line holding prefix, or 0.
func lineOf(body, prefix string) int {
	for index, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return index + 1
		}
	}
	return 0
}
