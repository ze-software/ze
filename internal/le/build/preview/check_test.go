// VALIDATES: portable verification checks actual ELF strings and every file.
// PREVENTS: a declared version masking a different binary or an extra payload.
package buildpreview

import (
	"debug/elf"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func TestELFStringReadsActualGoVariable(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		t.Fatal(err)
	}
	toolchain.Version = "preview-actual-linked-go-variables"
	toolchain.BuildDate = "2026-11-11T00:00:00Z"
	// go test may strip its own transient executable. Build a tiny fixture
	// with the same pinned patched compiler and linker stamps as a preview;
	// inspect both target formats without executing either target binary.
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			fixture := t.TempDir()
			put(t, fixture, "main.go", "package main\nvar version = \"dev\"\nvar buildDate = \"unknown\"\nfunc main() { println(version, buildDate) }\n")
			binary := filepath.Join(fixture, "ze")
			tags := []string{"ze_core", "ze_distro", "ze_bgp"}
			build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-tags", strings.Join(tags, " "), "-ldflags", toolchain.LDFlags(), "-o", binary, "main.go") //nolint:gosec // the test owns its fixture, output and compiler arguments
			build.Dir = fixture
			build.Env = append(toolchain.EnvironmentContext(t.Context(), gotoolchain.EnvOptions{GOOS: "linux", GOARCH: arch}), "GO111MODULE=off", "GOWORK=off", "GOENV=off", "GOFLAGS=-trimpath", "GOEXPERIMENT=", "GOAMD64=v1", "GOARM64=v8.0")
			if output, err := build.CombinedOutput(); err != nil {
				t.Fatalf("compile the stamped %s fixture: %v\n%s", arch, err, output)
			}
			file, err := elf.Open(binary)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			for name, want := range map[string]string{"main.version": toolchain.Version, "main.buildDate": toolchain.BuildDate} {
				got, err := elfString(file, name)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("ELF %s = %q, actual %q", name, got, want)
				}
			}
			if _, err := elfString(file, "main.absentPreviewStamp"); err == nil {
				t.Fatal("missing stamp accepted")
			}
			source := Source{Toolchain: toolchain.GoToolchain, Tags: tags, BuildDate: toolchain.BuildDate}
			if err := inspectBinary(binary, arch, source, toolchain.Version); err != nil {
				t.Fatal(err)
			}
			if err := inspectBinary(binary, arch, source, "preview-false-version"); err == nil {
				t.Fatal("incorrect declared binary version accepted")
			}
		})
	}
}

func rewriteManifest(t *testing.T, output string, manifest Manifest) {
	t.Helper()
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	put(t, output, "manifest.json", string(body))
	var sums strings.Builder
	for _, artifact := range manifest.Artifacts {
		fmt.Fprintf(&sums, "%s  %s\n", artifact.SHA256, artifact.File)
	}
	sum, err := fileSum(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&sums, "%s  manifest.json\n", sum)
	put(t, output, "SHA256SUMS", sums.String())
}

func TestCheckRefusesPayloadAndInventoryChanges(t *testing.T) {
	for _, attack := range []string{"extra-file", "input-inventory", "duplicate-target", "symlink", "checksum-traversal"} {
		t.Run(attack, func(t *testing.T) {
			root, deps := fixture(t)
			output := filepath.Join(t.TempDir(), "preview")
			manifest, err := prepare(t.Context(), root, output, false, deps)
			if err != nil {
				t.Fatal(err)
			}
			switch attack {
			case "extra-file":
				put(t, output, "private-password", "do not publish")
			case "input-inventory":
				manifest.Source.Inputs[0].SHA256 = strings.Repeat("f", 64)
				rewriteManifest(t, output, manifest)
			case "duplicate-target":
				manifest.Artifacts[2].Arch = "amd64"
				rewriteManifest(t, output, manifest)
			case "symlink":
				file := filepath.Join(output, manifest.Artifacts[0].File)
				if err := os.Rename(file, filepath.Join(t.TempDir(), "original")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("/etc/passwd", file); err != nil {
					t.Fatal(err)
				}
			case "checksum-traversal":
				put(t, output, "SHA256SUMS", strings.Repeat("a", 64)+"  ../private\n")
			}
			if _, err := check(output, deps.inspect); err == nil {
				t.Fatalf("accepted %s", attack)
			}
		})
	}
}
