// Design: (none -- build tool)
//
// check_web checks the npm registry for newer versions of vendored web assets
// and reports any drift between vendor copies and consumer copies.
//
// Reads versions from third_party/web/MANIFEST.md, fetches the latest version
// for each package from registry.npmjs.org, and compares.
//
// Usage: go run scripts/vendor/check_web.go
//
// Replaces the previous bash implementation (scripts/check-vendor-web.sh).
// The bash version used `grep -P` which is unavailable on macOS BSD grep --
// the Go version is portable.

//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

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
	url := fmt.Sprintf("https://registry.npmjs.org/%s/latest", pkg)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
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
		fmt.Printf("  %s: version not found in MANIFEST.md\n", pv.pkg)
		return
	}
	latest, err := fetchLatestNpmVersion(pv.pkg)
	if err != nil || latest == "" {
		fmt.Printf("  %s: could not fetch latest version (%v)\n", pv.pkg, err)
		return
	}
	if pv.current == latest {
		fmt.Printf("  %s: %s (up to date)\n", pv.pkg, pv.current)
	} else {
		fmt.Printf("  %s: %s -> %s available\n", pv.pkg, pv.current, latest)
	}
}

// driftCheck compares each consumer's vendor file against the source.
func driftCheck(root string) bool {
	vendor := filepath.Join(root, "third_party/web/htmx")
	consumers := []string{
		filepath.Join(root, "internal/chaos/web/assets"),
		filepath.Join(root, "internal/component/web/assets"),
	}
	files := []string{"htmx.min.js", "sse.js"}

	drift := false
	for _, dest := range consumers {
		for _, f := range files {
			src := filepath.Join(vendor, f)
			dst := filepath.Join(dest, f)
			srcData, errSrc := os.ReadFile(src)
			dstData, errDst := os.ReadFile(dst)
			if errDst != nil {
				fmt.Printf("  MISSING: %s\n", dst)
				drift = true
				continue
			}
			if errSrc != nil {
				continue
			}
			if !bytes.Equal(srcData, dstData) {
				fmt.Printf("  DRIFT: %s differs from vendor copy\n", dst)
				drift = true
			}
		}
	}
	return drift
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(root, "third_party/web/MANIFEST.md")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", manifestPath, err)
	}
	manifest := string(manifestBytes)

	fmt.Println("checking vendored web assets against npm registry...")
	fmt.Println()

	pkgs := []pkgVersion{
		{pkg: "htmx.org", current: extractVersionFromManifest(manifest, "htmx.min.js")},
		{pkg: "htmx-ext-sse", current: extractVersionFromManifest(manifest, "sse.js")},
	}
	for _, p := range pkgs {
		checkVersion(p)
	}

	fmt.Println()
	fmt.Println("checking consumer copies...")
	if !driftCheck(root) {
		fmt.Println("  all consumer copies match vendor")
	}
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
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "check_web: %v\n", err)
		os.Exit(1)
	}
}
