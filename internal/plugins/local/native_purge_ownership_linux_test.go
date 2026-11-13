//go:build linux

// Design: docs/architecture/cli/plugin-modes.md -- native removal respects package ownership.

package local

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

var nativePurgeRoot = flag.String("native-purge-root", "", "private native-removal worker filesystem")
var nativePurgeCaseName = flag.String("native-purge-case", "", "private native-removal worker case")

type nativePurgeCase struct {
	name           string
	prefix         string
	marker         string
	binaryPresent  bool
	install        bool
	explicitLocal  bool
	dryRun         bool
	packageBinLink bool
	purge          bool
	refused        bool
}

func nativePurgeCases() []nativePurgeCase {
	return append([]nativePurgeCase{
		{name: "shared-package-state-install-without-standalone", prefix: "/usr/local", marker: "/usr/share/ze/install-method", install: true, refused: true},
		{name: "shared-package-state-install-with-standalone", prefix: "/usr/local", marker: "/usr/share/ze/install-method", binaryPresent: true, install: true, refused: true},
		{name: "shared-package-state-install-relocated-package-bin", prefix: "/usr/local", marker: "/usr/share/ze/install-method", packageBinLink: true, install: true, refused: true},
		{name: "shared-package-state-without-standalone", prefix: "/usr/local", marker: "/usr/share/ze/install-method", purge: true, refused: true},
		{name: "shared-package-state-with-standalone", prefix: "/usr/local", marker: "/usr/share/ze/install-method", binaryPresent: true, purge: true, refused: true},
		{name: "shared-package-state-relocated-package-bin", prefix: "/usr/local", marker: "/usr/share/ze/install-method", packageBinLink: true, purge: true, refused: true},
		{name: "selected-package-prefix", prefix: "/usr/local", marker: "/usr/local/share/ze/install-method", binaryPresent: true, purge: true, refused: true},
		{name: "standalone-purge", prefix: "/usr/local", binaryPresent: true, purge: true},
		{name: "standalone-state-only-purge", prefix: "/usr/local", purge: true},
		{name: "custom-prefix-independent-of-package", prefix: "/opt/ze", marker: "/usr/share/ze/install-method", binaryPresent: true, purge: true},
		{name: "ordinary-removal-preserves-shared-state", prefix: "/usr/local", marker: "/usr/share/ze/install-method", binaryPresent: true},
	}, explicitLocalPurgeCases()...)
}

// TestNativePurgeRespectsSharedPackageState runs the actual default uninstall
// inside a child-only filesystem. The parent also checks every survivor, so an
// early refusal cannot stand in for a successful independent standalone purge.
func TestNativePurgeRespectsSharedPackageState(t *testing.T) {
	nativePurgeRunCases(t, false, false)
}

// TestNativeInstallRespectsSharedPackageState exercises the default installer
// before it can replace a binary or adopt the package installation's store.
func TestNativeInstallRespectsSharedPackageState(t *testing.T) {
	nativePurgeRunCases(t, true, false)
}

func nativePurgeRunCases(t *testing.T, install, explicitLocal bool) {
	t.Helper()
	for _, test := range nativePurgeCases() {
		if test.install != install {
			continue
		}
		if test.explicitLocal != explicitLocal {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			nativePurgeSeed(t, root, "/download/ze", "new-binary", 0o755)
			target, err := selectNativeTarget(test.prefix)
			if err != nil {
				t.Fatal(err)
			}
			state := filepath.Join(target.configDir, "database", "credentials")
			nativePurgeSeed(t, root, state, "selected-state", 0o600)
			if test.binaryPresent {
				nativePurgeSeed(t, root, target.binary, "standalone-binary", 0o755)
			}
			if test.marker != "" {
				nativePurgeSeed(t, root, test.marker, "deb\n", 0o644)
				if test.packageBinLink {
					nativePurgeSeed(t, root, "/package/bin/ze", "packaged-binary", 0o755)
					if err := os.Symlink("../package/bin", nativePurgePath(root, "/usr/bin")); err != nil {
						t.Fatal(err)
					}
				} else {
					nativePurgeSeed(t, root, "/usr/bin/ze", "packaged-binary", 0o755)
				}
			}
			if test.prefix != "/usr/local" {
				nativePurgeSeed(t, root, "/etc/ze/database/credentials", "package-state", 0o600)
			}

			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestNativePurgeWorker$", "-test.v", "-native-purge-root="+root, "-native-purge-case="+test.name)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Cloneflags:                 syscall.CLONE_NEWUSER,
				UidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
				GidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
				GidMappingsEnableSetgroups: false,
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("isolated native operation: %v\n%s", err, output)
			} else {
				t.Logf("isolated native operation:\n%s", output)
			}

			if test.refused || test.dryRun {
				nativePurgeRetained(t, root, state, "selected-state")
				if test.binaryPresent {
					nativePurgeRetained(t, root, target.binary, "standalone-binary")
				} else {
					if _, err := os.Lstat(nativePurgePath(root, target.binary)); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("refusal created a standalone binary: %v", err)
					}
				}
			} else {
				if _, err := os.Lstat(nativePurgePath(root, target.binary)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("standalone binary was not removed: %v", err)
				}
				if test.purge {
					if _, err := os.Lstat(nativePurgePath(root, target.configDir)); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("selected state was not purged: %v", err)
					}
				} else {
					nativePurgeRetained(t, root, state, "selected-state")
				}
			}
			if test.marker != "" {
				nativePurgeRetained(t, root, test.marker, "deb\n")
				nativePurgeRetained(t, root, "/usr/bin/ze", "packaged-binary")
			}
			if test.prefix != "/usr/local" {
				nativePurgeRetained(t, root, "/etc/ze/database/credentials", "package-state")
			}
		})
	}
}

// TestNativePurgeWorker enters only the parent-created private root, then calls
// the same flag parser, target selection, preflight and operation as ze install
// and ze uninstall.
// No path mutation takes place before both chroot and chdir have succeeded.
func TestNativePurgeWorker(t *testing.T) {
	if *nativePurgeRoot == "" {
		t.Skip("only the private native-removal subprocess runs this worker")
	}
	if err := unix.Chroot(*nativePurgeRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("/"); err != nil {
		t.Fatal(err)
	}
	for _, test := range nativePurgeCases() {
		if test.name != *nativePurgeCaseName {
			continue
		}
		var output bytes.Buffer
		rt := &nativeRuntime{source: "/download/ze", stdin: strings.NewReader(""), stdout: &output, stderr: &output}
		args := []string{"--no-service"}
		if !test.install {
			args = append(args, "--yes")
		}
		if test.prefix != "/usr/local" {
			args = append(args, "--prefix", test.prefix)
		}
		if test.purge {
			args = append(args, "--purge")
		}
		if test.dryRun {
			args = append(args, "--dry-run")
		}
		var code int
		if test.explicitLocal {
			args = []string{"--prefix", test.prefix, "--yes"}
			if test.purge {
				args = append(args, "--purge")
			}
			if test.dryRun {
				args = append(args, "--dry-run")
			}
			var text string
			code, text = explicitLocalPurgeOutput(t, args)
			output.WriteString(text)
			if test.refused {
				explicitLocalPurgeRefusal(t, text, test.marker)
			}
		} else if test.install {
			code = rt.install(args)
		} else {
			code = rt.uninstall(args)
		}
		if test.refused {
			if code == exitOK {
				t.Fatalf("package-owned shared state mutation succeeded: %s", &output)
			}
			if !strings.Contains(output.String(), test.marker) {
				t.Fatalf("refusal did not name package ownership %s: %s", test.marker, &output)
			}
			return
		}
		if code != exitOK {
			t.Fatalf("standalone operation=%d: %s", code, &output)
		}
		return
	}
	t.Fatalf("unknown private native-removal case %q", *nativePurgeCaseName)
}

func nativePurgePath(root, path string) string {
	return filepath.Join(root, strings.TrimPrefix(path, "/"))
}

func nativePurgeSeed(t *testing.T, root, path, content string, mode os.FileMode) {
	t.Helper()
	path = nativePurgePath(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func nativePurgeRetained(t *testing.T, root, path, want string) {
	t.Helper()
	got, err := os.ReadFile(nativePurgePath(root, path))
	if err != nil {
		t.Errorf("retained %s: %v", path, err)
		return
	}
	if string(got) != want {
		t.Errorf("retained %s=%q, want %q", path, got, want)
	}
}
