// Design: docs/architecture/testing/ci-workflows.md -- tracked Go and canonical cache boundaries.
package codeql

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestSelectUsesTrackedFilesAndActualContexts proves new authored roots require
// no central list, while each real Go context selects only its buildable dirs.
func TestSelectUsesTrackedFilesAndActualContexts(t *testing.T) {
	root := fixtureRepository(t, map[string]string{
		"newroot/new.go":                  "package newroot\n",
		"gokrazy/authored/main.go":        "package authored\n",
		"gokrazy/modcache/cache/cache.go": "not Go source\n",
		"distro/distro.go":                "//go:build ze_distro\n\npackage distro\n",
		"appliance/appliance.go":          "//go:build ze_appliance\n\npackage appliance\n",
		"setup/setup.go":                  "//go:build ze_setup\n\npackage setup\n",
		"darwin/platform_darwin.go":       "package darwin\n",
		"arm/platform_arm64.go":           "package arm\n",
		"cgo/cgo.go":                      "package cgo\nimport \"C\"\n",
	})
	fixtureWrite(t, root, "untracked/untracked.go", "package untracked\n")
	for _, test := range []struct {
		name, goos, goarch, tags string
		want                     []string
	}{
		{"distro", "linux", "amd64", "ze_core ze_distro", []string{"./authored", "./distro", "./gokrazy/authored", "./newroot"}},
		{"appliance", "linux", "amd64", "ze_core ze_appliance", []string{"./appliance", "./authored", "./gokrazy/authored", "./newroot"}},
		{"setup", "linux", "amd64", "ze_setup", []string{"./authored", "./gokrazy/authored", "./newroot", "./setup"}},
		{"platform", "darwin", "arm64", "ze_setup", []string{"./arm", "./authored", "./darwin", "./gokrazy/authored", "./newroot", "./setup"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GOOS", test.goos)
			t.Setenv("GOARCH", test.goarch)
			t.Setenv("CGO_ENABLED", "0")
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			buildContext, err := BuildContext(ctx, root, test.tags)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Select(ctx, root, buildContext)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("selected %v, want %v", got, test.want)
			}
		})
	}
}

// TestGuardRefusesCanonicalCache exercises direct requests, package-directory
// links, imported links and individual source links using the actual Go loader.
func TestGuardRefusesCanonicalCache(t *testing.T) {
	for _, kind := range []string{"explicit", "directory", "dependency", "source"} {
		t.Run(kind, func(t *testing.T) {
			root := fixtureRepository(t, map[string]string{
				"gokrazy/modcache/dependency/dep.go": "package dependency\n",
			})
			packages := []string{"./authored"}
			switch kind {
			case "explicit":
				packages = []string{"./gokrazy/modcache/dependency"}
			case "directory":
				if err := os.Symlink("gokrazy/modcache/dependency", filepath.Join(root, "bridge")); err != nil {
					t.Fatal(err)
				}
				packages = []string{"./bridge"}
			case "dependency":
				fixtureWrite(t, root, "authored/import.go", "package authored\nimport _ \"example.test/codeql/bridge\"\n")
				if err := os.Symlink("gokrazy/modcache/dependency", filepath.Join(root, "bridge")); err != nil {
					t.Fatal(err)
				}
			case "source":
				fixtureWrite(t, root, "gokrazy/modcache/source.go", "package authored\n")
				if err := os.Symlink("../gokrazy/modcache/source.go", filepath.Join(root, "authored", "linked.go")); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			t.Setenv("CGO_ENABLED", "0")
			buildContext, err := BuildContext(ctx, root, "ze_setup")
			if err != nil {
				t.Fatal(err)
			}
			err = Guard(ctx, root, buildContext, packages)
			if err == nil {
				t.Fatal("cache request was admitted")
			}
			if !strings.Contains(err.Error(), "gokrazy/modcache") {
				t.Fatalf("wrong refusal: %v", err)
			}
		})
	}
}

// TestSelectRefusesUnclassifiedAndBrokenInputs prevents a silent drop when a
// tracked form, nested module, missing source or dependency cannot be admitted.
func TestSelectRefusesUnclassifiedAndBrokenInputs(t *testing.T) {
	for _, test := range []struct {
		name   string
		files  map[string]string
		remove string
		want   string
	}{
		{"hidden", map[string]string{"newroot/.hidden.go": "package hidden\n"}, "", "unclassified tracked Go"},
		{"nested module", map[string]string{"nested/go.mod": "module example.test/nested\n", "nested/pkg.go": "package nested\n"}, "", "unclassified tracked Go"},
		{"missing tracked file", map[string]string{"newroot/main.go": "package newroot\n"}, "newroot/main.go", "tracked Go"},
		{"broken dependency", map[string]string{"authored/import.go": "package authored\nimport _ \"example.test/codeql/missing\"\n"}, "", "go list"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureRepository(t, test.files)
			if test.remove != "" {
				if err := os.Remove(filepath.Join(root, test.remove)); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			t.Setenv("CGO_ENABLED", "0")
			buildContext, err := BuildContext(ctx, root, "ze_setup")
			if err != nil {
				t.Fatal(err)
			}
			_, err = Select(ctx, root, buildContext)
			if err == nil {
				t.Fatal("invalid tracked population was admitted")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("wrong refusal: %v, want %q", err, test.want)
			}
		})
	}
}
