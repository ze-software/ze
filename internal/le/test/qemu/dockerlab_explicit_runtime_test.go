// Design: docs/architecture/testing/qemu-integration.md -- explicit caller-owned runtime pairs.
// Related: dockerlab_kernel_acquisition_test.go -- private payload ownership at QEMU consumption.

package testqemu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// TestDockerLabCommandExplicitRuntimePairSkipsCurrentResolver supplies a named
// cached runtime and its real FAT image while the current source resolver fails.
// Valid explicit inputs must reach the command consumer without resolving or
// building today's runtime; invalid identity, modules or image must still refuse.
func TestDockerLabCommandExplicitRuntimePairSkipsCurrentResolver(t *testing.T) {
	for _, test := range []struct {
		name    string
		alter   func(*testing.T, string, *RunOptions)
		code    int
		refusal string
	}{
		{name: "valid older runtime pair", code: 5},
		{name: "tree outside runtime cache", code: 1, alter: func(t *testing.T, tree string, options *RunOptions) {
			outside := filepath.Join(t.TempDir(), filepath.Base(tree))
			writeDockerLabExplicitRuntime(t, outside, "amd64")
			options.Kernel = filepath.Join(outside, "vmlinuz")
		}},
		{name: "cache entry names another architecture", code: 1, alter: func(t *testing.T, tree string, options *RunOptions) {
			other := strings.Replace(tree, "-amd64-", "-arm64-", 1)
			writeDockerLabExplicitRuntime(t, other, "amd64")
			options.Kernel = filepath.Join(other, "vmlinuz")
		}},
		{name: "provenance names another architecture", code: 1, alter: func(t *testing.T, tree string, _ *RunOptions) {
			path := filepath.Join(tree, kernelbuilder.ProvenanceName)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			body = []byte(strings.Replace(string(body), "arch=amd64", "arch=arm64", 1))
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "provenance names another source digest", code: 1, refusal: "runtime provenance source-sha256=", alter: func(t *testing.T, tree string, _ *RunOptions) {
			path := filepath.Join(tree, kernelbuilder.ProvenanceName)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			digest, known := kernelbuilder.SourceDigest(appliance.KernelVersion())
			if !known {
				t.Fatal("the fixture's kernel release has no tracked source digest")
			}
			body = []byte(strings.Replace(string(body), "source-sha256="+digest,
				"source-sha256="+strings.Repeat("0", 64), 1))
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "provenance repeats an identical architecture", code: 1, refusal: `repeats "arch"`, alter: func(t *testing.T, tree string, _ *RunOptions) {
			path := filepath.Join(tree, kernelbuilder.ProvenanceName)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			body = append(body, []byte("arch=amd64\n")...)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "otherwise valid provenance exceeds its bound", code: 1, refusal: "exceeds 4096 bytes", alter: func(t *testing.T, tree string, _ *RunOptions) {
			path := filepath.Join(tree, kernelbuilder.ProvenanceName)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// One well-formed new key preserves every required value and
			// introduces no duplicate, isolating the completion-record bound.
			body = append(body, []byte("fixture-padding="+strings.Repeat("x", 4096)+"\n")...)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing module index", code: 1, alter: func(t *testing.T, tree string, _ *RunOptions) {
			if err := os.Remove(filepath.Join(tree, "lib", "modules", "7.3.0-ze", "modules.dep")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "image embeds another kernel", code: 1, alter: func(t *testing.T, _ string, options *RunOptions) {
			options.Image = dockerLabImageFixture(t, "foreign-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")
		}},
		{name: "missing named image", code: 1, alter: func(_ *testing.T, _ string, options *RunOptions) {
			options.Image += ".missing"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			tree := dockerLabExplicitRuntimeFixture(t, "amd64")
			options := RunOptions{Kernel: filepath.Join(tree, "vmlinuz"), Command: "exit 5",
				Image: dockerLabImageFixture(t, "runtime-kernel", "root=PARTUUID=fixture-02 ro init=/gokrazy/init\n")}
			if test.alter != nil {
				test.alter(t, tree, &options)
			}
			if test.refusal != "" {
				_, err := appliance.ExistingRuntimeKernelTree(options.Kernel, "amd64")
				if err == nil {
					t.Fatal("invalid completion metadata was accepted")
				}
				if !strings.Contains(err.Error(), test.refusal) {
					t.Fatalf("completion metadata refused for %v, want %q", err, test.refusal)
				}
			}
			canonicalTree, err := filepath.EvalSymlinks(tree)
			if err != nil {
				t.Fatal(err)
			}
			previous := dockerLabHost
			t.Cleanup(func() { dockerLabHost = previous })
			resolved, images, guests, boots, commands := 0, 0, 0, 0, 0
			dockerLabHost = dockerLabCommandHost{
				runtimeTree: func(string, string) (string, error) {
					resolved++
					return "", errors.New("current runtime source key is deliberately unavailable")
				},
				ensureImage: func(context.Context, string, string) (string, error) {
					images++
					return "", errors.New("an explicit pair must not build an image")
				},
				buildGuest: func(context.Context, string, string) error {
					guests++
					return errors.New("an ordinary command must not build a guest launcher")
				},
				boot: func(_ context.Context, run *Run, image string, kernel *dockerLabKernel) (DockerLabReport, error) {
					boots++
					report := DockerLabReport{Image: image, Kernel: kernel, CommandExit: 5}
					// Keep the real embedded-kernel check at the boot boundary.
					if _, err := dockerLabImageBoot(image, kernel); err != nil {
						return report, err
					}
					if image != options.Image {
						t.Errorf("consumer selected image %q, want caller's %q", image, options.Image)
					}
					if kernel.Runtime != canonicalTree {
						t.Errorf("consumer selected runtime %q, want named cache entry %q", kernel.Runtime, canonicalTree)
					}
					if kernel.Selected != options.Kernel || run.Options.Kernel != options.Kernel {
						t.Errorf("consumer lost the caller's kernel: identity=%+v, option=%q", kernel, run.Options.Kernel)
					}
					digest := sha256.Sum256([]byte("runtime-kernel"))
					if kernel.SHA256 != hex.EncodeToString(digest[:]) || kernel.Release != "7.3.0-ze" {
						t.Errorf("consumer lost validated kernel/modules identity: %+v", kernel)
					}
					commands++
					return report, nil
				},
			}
			_, code := runDockerLabCommand(t.Context(), t.TempDir(), "amd64", options)
			if code != test.code {
				t.Errorf("command exit=%d, want %d", code, test.code)
			}
			if resolved != 0 || images != 0 || guests != 0 {
				t.Errorf("explicit inputs reached current resolver/builders: runtime=%d, image=%d, guest=%d", resolved, images, guests)
			}
			if test.refusal != "" {
				if boots != 0 {
					t.Errorf("invalid completion metadata reached the boot consumer %d times", boots)
				}
			}
			wantCommands := 0
			if test.code == 5 {
				wantCommands = 1
			}
			if commands != wantCommands {
				t.Errorf("executed command consumers=%d, want %d", commands, wantCommands)
			}
		})
	}
}

// TestDockerLabCommandWithoutImageUsesCurrentResolver preserves acquisition for
// callers that supply no image. Its resolver failure must stop before building.
func TestDockerLabCommandWithoutImageUsesCurrentResolver(t *testing.T) {
	previous := dockerLabHost
	t.Cleanup(func() { dockerLabHost = previous })
	resolved := 0
	dockerLabHost = dockerLabCommandHost{
		runtimeTree: func(string, string) (string, error) {
			resolved++
			return "", errors.New("current runtime resolution deliberately stops here")
		},
		ensureImage: func(context.Context, string, string) (string, error) {
			t.Error("resolver failure reached image build")
			return "", errors.New("unexpected image build")
		},
		boot: func(context.Context, *Run, string, *dockerLabKernel) (DockerLabReport, error) {
			t.Error("resolver failure reached boot")
			return DockerLabReport{}, errors.New("unexpected boot")
		},
	}
	_, code := runDockerLabCommand(t.Context(), t.TempDir(), "amd64", RunOptions{Kernel: "/unused", Command: "true"})
	if code != 1 || resolved != 1 {
		t.Errorf("no-image call exit=%d, resolver calls=%d; want 1 and 1", code, resolved)
	}
}

// dockerLabExplicitRuntimeFixture owns one named cache entry for the caller's
// architecture. Its isolated namespace never reads or changes the host cache.
func dockerLabExplicitRuntimeFixture(t *testing.T, arch string) string {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	tree := filepath.Join(cache, "ze", "runtime-kernel",
		appliance.KernelVersion()+"-runtime-"+arch+"-runtime-docker-01234567-89abcdef")
	writeDockerLabExplicitRuntime(t, tree, arch)
	return tree
}

// writeDockerLabExplicitRuntime writes only the small named-cache inputs the
// consumer validates; no current recipe, image builder or kernel builder runs.
func writeDockerLabExplicitRuntime(t *testing.T, tree, arch string) {
	t.Helper()
	modules := filepath.Join(tree, "lib", "modules", "7.3.0-ze")
	if err := os.MkdirAll(modules, 0o700); err != nil {
		t.Fatal(err)
	}
	digest, known := kernelbuilder.SourceDigest(appliance.KernelVersion())
	if !known {
		t.Fatal("the shipped kernel release has no source digest")
	}
	provenance := "version=" + appliance.KernelVersion() + "\ntarget=runtime\nprofile=runtime\narch=" + arch + "\nmodules=yes\nbuilder=docker\n" +
		"source-url=https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-" + appliance.KernelVersion() + ".tar.xz\nsource-sha256=" + digest + "\n"
	for path, body := range map[string]string{
		filepath.Join(tree, "vmlinuz"):                    "runtime-kernel",
		filepath.Join(modules, "modules.dep"):             "kernel/fixture.ko:\n",
		filepath.Join(tree, kernelbuilder.ProvenanceName): provenance,
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
