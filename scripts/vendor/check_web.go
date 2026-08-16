// Design: (none -- build tool)
//
// check_web is the read-only twin of scripts/vendor/sync_web.go. It compares
// every consumer copy of a vendored web asset against its third_party/web/
// source and exits non-zero when one differs.
//
// The gate needs no network. `--updates` adds an npm registry query that
// reports newer releases, and that query is the only part which does.
//
// Usage: go run scripts/vendor/check_web.go [--root DIR] [--updates]
//
// Run by `make ze-vendor-web-check`, which is a stage of `make ze-precommit-verify` and a
// prerequisite of ze-generated-files-check. `make ze-vendor-web-update-report`
// runs the registry query.
//
// Replaces the previous bash implementation (scripts/check-vendor-web.sh).
// The bash version used `grep -P` which is unavailable on macOS BSD grep --
// the Go version is portable.

//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// vendorDir is the source of truth, relative to the repository root.
const vendorDir = "third_party/web"

// consumerRoot is the tree walked for consumer asset directories.
const consumerRoot = "internal"

// consumerDirName is the directory name a consumer keeps its embedded assets
// in. Every row of MANIFEST.md's Consumers table uses it.
const consumerDirName = "assets"

// pkgVersion describes one vendored npm package.
type pkgVersion struct {
	pkg     string // npm package name
	current string // version recorded in MANIFEST.md
}

// extractVersionFromManifest scans the MANIFEST.md row that mentions the
// given file name and returns the first semver triple it finds (X.Y.Z).
var semverRE = regexp.MustCompile(`\d+\.\d+\.\d+`)

func extractVersionFromManifest(manifest, fileName string) string {
	for _, line := range bytes.Split([]byte(manifest), []byte("\n")) {
		if !bytes.Contains(line, []byte(fileName)) {
			continue
		}
		if m := semverRE.Find(line); m != nil {
			return string(m)
		}
	}
	return ""
}

// fetchLatestNpmVersion queries https://registry.npmjs.org/<pkg>/latest and
// returns the "version" field from the JSON response.
func fetchLatestNpmVersion(pkg string) (string, error) {
	var url strings.Builder
	url.WriteString("https://registry.npmjs.org/")
	url.WriteString(pkg)
	url.WriteString("/latest")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry returned %s", resp.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.Version, nil
}

func checkVersion(pv pkgVersion) {
	if pv.current == "" {
		fmt.Fprintf(os.Stdout, "  %s: version not found in MANIFEST.md\n", pv.pkg)
		return
	}
	latest, err := fetchLatestNpmVersion(pv.pkg)
	if err != nil || latest == "" {
		fmt.Fprintf(os.Stdout, "  %s: could not fetch latest version (%v)\n", pv.pkg, err)
		return
	}
	if pv.current == latest {
		fmt.Fprintf(os.Stdout, "  %s: %s (up to date)\n", pv.pkg, pv.current)
	} else {
		fmt.Fprintf(os.Stdout, "  %s: %s -> %s available\n", pv.pkg, pv.current, latest)
	}
}

// checkUpdates reports newer releases of the vendored packages. This is the one
// part of this program that uses the network, and `--updates` is what runs it.
func checkUpdates(root string) error {
	manifestPath := filepath.Join(root, vendorDir, "MANIFEST.md")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", manifestPath, err)
	}
	manifest := string(manifestBytes)

	fmt.Fprintln(os.Stdout, "checking vendored web assets against npm registry...")
	fmt.Fprintln(os.Stdout)

	// One row covers both htmx files. htmx 4 publishes its extensions inside
	// the core npm package, where htmx 2 published htmx-ext-sse beside it, so
	// hx-sse.min.js carries the version htmx.min.js does.
	pkgs := []pkgVersion{
		{pkg: "htmx.org", current: extractVersionFromManifest(manifest, "htmx.min.js")},
	}
	for _, p := range pkgs {
		checkVersion(p)
	}
	fmt.Fprintln(os.Stdout)

	return nil
}

// vendorPackage is one directory under third_party/web/ and the file names it
// holds. The directory is the unit a consumer subscribes to, so an asset that
// belongs to one consumer alone gets its own directory (swagger-ui/ is that
// case). third_party/web/MANIFEST.md states the same contract.
type vendorPackage struct {
	dir   string   // directory name, e.g. "htmx"
	files []string // file names, sorted
}

// vendorPackages reads the source of truth. It fails when the tree holds no
// package, and when two packages hold one file name: a consumer copy is matched
// by name, so an ambiguous name has no single source to compare against.
func vendorPackages(root string) ([]vendorPackage, error) {
	base := filepath.Join(root, vendorDir)

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", base, err)
	}

	var pkgs []vendorPackage
	owner := map[string]string{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // MANIFEST.md, and anything else at the top level
		}

		dir := filepath.Join(base, entry.Name())

		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}

		pkg := vendorPackage{dir: entry.Name()}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if first, seen := owner[file.Name()]; seen {
				return nil, fmt.Errorf("%s is vendored twice, in %s/ and %s/; a consumer copy of that name has no single source", file.Name(), first, entry.Name())
			}
			owner[file.Name()] = entry.Name()
			pkg.files = append(pkg.files, file.Name())
		}

		if len(pkg.files) == 0 {
			continue
		}

		sort.Strings(pkg.files)
		pkgs = append(pkgs, pkg)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("%s holds no vendored package, so nothing could be compared", base)
	}

	return pkgs, nil
}

// consumerDirs returns every consumer asset directory, relative to root.
//
// The list is DERIVED, not written down. A hand-written list is the failure
// this program exists to catch, one level up: a consumer added to sync_web.go
// and forgotten here would hold copies that nothing gates.
func consumerDirs(root string) ([]string, error) {
	base := filepath.Join(root, consumerRoot)

	var dirs []string

	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if name == "testdata" || name == "node_modules" || (len(name) > 1 && name[0] == '.') {
			return fs.SkipDir
		}
		if name != consumerDirName {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		dirs = append(dirs, rel)

		return fs.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", base, err)
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("%s holds no %s/ directory, so nothing could be compared", base, consumerDirName)
	}

	sort.Strings(dirs)

	return dirs, nil
}

// driftCheck compares each consumer copy of a vendored asset against its
// source. It writes one line per problem and returns the number of problems
// with the number of copies it read.
//
// A consumer subscribes to a vendor directory by holding one file of it, and it
// then owes a matching copy of EVERY file of that directory. A file the sync
// never wrote is therefore a problem, not an absence nobody looks at.
func driftCheck(root string) (problems, compared int, err error) {
	pkgs, err := vendorPackages(root)
	if err != nil {
		return 0, 0, err
	}

	consumers, err := consumerDirs(root)
	if err != nil {
		return 0, 0, err
	}

	subscribers := map[string]int{}

	for _, consumer := range consumers {
		for _, pkg := range pkgs {
			if !subscribes(root, consumer, pkg) {
				continue
			}

			subscribers[pkg.dir]++

			for _, name := range pkg.files {
				source := filepath.Join(vendorDir, pkg.dir, name)
				copied := filepath.Join(consumer, name)

				sourceData, sourceErr := os.ReadFile(filepath.Join(root, source))
				if sourceErr != nil {
					fmt.Fprintf(os.Stdout, "  UNREADABLE: %s (%v)\n", source, sourceErr)
					problems++
					continue
				}

				copiedData, copiedErr := os.ReadFile(filepath.Join(root, copied))
				if copiedErr != nil {
					fmt.Fprintf(os.Stdout, "  MISSING: %s\n", copied)
					problems++
					continue
				}

				compared++

				if !bytes.Equal(sourceData, copiedData) {
					fmt.Fprintf(os.Stdout, "  DRIFT: %s differs from %s\n", copied, source)
					problems++
				}
			}
		}
	}

	// A vendored package that reaches no consumer is the same defect seen from
	// the source side: the sync was never told to copy it. Without this, the
	// subscription rule would report nothing at all about a new directory.
	for _, pkg := range pkgs {
		if subscribers[pkg.dir] == 0 {
			fmt.Fprintf(os.Stdout, "  UNSYNCED: %s reaches no consumer; add it to scripts/vendor/sync_web.go\n", filepath.Join(vendorDir, pkg.dir))
			problems++
		}
	}

	return problems, compared, nil
}

// subscribes reports whether a consumer holds any file of one vendor package.
func subscribes(root, consumer string, pkg vendorPackage) bool {
	for _, name := range pkg.files {
		if _, err := os.Stat(filepath.Join(root, consumer, name)); err == nil {
			return true
		}
	}

	return false
}

func run(root string, updates bool) error {
	if updates {
		if err := checkUpdates(root); err != nil {
			return err
		}
	}

	fmt.Fprintln(os.Stdout, "checking consumer copies...")

	problems, compared, err := driftCheck(root)
	if err != nil {
		return err
	}

	// FAIL CLOSED. A run that read nothing has proven nothing, so it must not
	// report what a run that read every copy reports.
	if compared == 0 && problems == 0 {
		return fmt.Errorf("no consumer copy of a vendored asset was found under %s/, so this check proved nothing", consumerRoot)
	}

	if problems > 0 {
		return fmt.Errorf("%d consumer asset copy problem(s); run `make ze-vendor-web-sync` and commit the result", problems)
	}

	fmt.Fprintf(os.Stdout, "  all %d consumer copies match their %s/ source\n", compared, vendorDir)

	return nil
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func main() {
	rootFlag := flag.String("root", "", "repository root to check (default: the tree holding the working directory)")
	updates := flag.Bool("updates", false, "also query the npm registry for newer releases; this is the only part that uses the network")
	flag.Parse()

	root := *rootFlag
	if root == "" {
		found, err := repoRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "check_web: %v\n", err)
			os.Exit(1)
		}
		root = found
	}

	if err := run(root, *updates); err != nil {
		fmt.Fprintf(os.Stderr, "check_web: %v\n", err)
		os.Exit(1)
	}
}
