// Design: website/AI.md -- website sources and the Pages artifact are separate trees
package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Paths is the filesystem contract shared by every site action.
type Paths struct {
	Repository string
	Source     string
	Output     string
}

// resolvePaths derives the site source and output roots from one checkout.
func resolvePaths(repository, output string) (Paths, error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return Paths{}, err
	}
	root = filepath.Clean(root)
	if output == "" {
		output = filepath.Join(filepath.Dir(root), "gh-pages")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return Paths{}, err
	}
	source := filepath.Join(root, "website")
	if output == source || pathWithin(source, output) {
		return Paths{}, fmt.Errorf("site output must be outside the website source tree: %s", output)
	}
	if info, statErr := os.Stat(source); statErr != nil || !info.IsDir() {
		if statErr == nil {
			statErr = fmt.Errorf("not a directory")
		}
		return Paths{}, fmt.Errorf("website source %s: %w", source, statErr)
	}
	return Paths{Repository: root, Source: source, Output: filepath.Clean(output)}, nil
}

func pathWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// sourceOnlyDirectories are directories under website/ that must not be
// published. The list is DEFENSIVE: an entry naming nothing costs nothing and
// catches the directory the day it appears, which is why `.claude` and
// `.github` are here while website/ holds neither today. Do not prune an entry
// merely because it currently matches nothing.
//
// Three entries match nothing in website/ today: `.claude`, `.github` and
// `presentations/tools`. The first two are ordinary defence. The third names a
// directory that was RETIRED -- website/presentations went with eae282592
// ("make le a ze personality and retire make and scripts") -- and it is kept
// anyway, because TestSourceOnlyBoundary pins presentations/tools/bundle.go
// and dropping the entry would mean deleting that case to make the list tidy.
var sourceOnlyDirectories = []string{
	".claude", ".github", "assets/css", "assets/js", "blog/posts",
	"changes/posts", "presentations/tools", "tools",
}

var sourceOnlyFiles = map[string]bool{
	".gitignore": true, "AI.md": true, "assets/vendor/README.md": true,
	"assets/vendor/fonts/README.md": true, "CACHEDIR.TAG": true,
	"compare/bgp.md": true, "compare/comparison.md": true, "compare/nos.md": true,
	"contribute/contribute.md": true, "contribute/guide.md": true,
	"docs/docs.md": true, "faq/faq.md": true, "license/license.md": true,
	"quality/browser-editor.md": true, "quality/functional-ci.md": true,
	"quality/qemu-interop-release.md": true, "quality/quality.md": true,
	"quality/unit-fuzz-mutation.md": true, "quality/verify-debugging.md": true,
	"roadmap/roadmap.md": true,
}

// isSourceOnly reports whether a source-relative path must not be deployed.
func isSourceOnly(name string) bool {
	name = strings.Trim(strings.ReplaceAll(filepath.ToSlash(name), "//", "/"), "/")
	if sourceOnlyFiles[name] {
		return true
	}
	for _, directory := range sourceOnlyDirectories {
		if name == directory || strings.HasPrefix(name, directory+"/") {
			return true
		}
	}
	return false
}
