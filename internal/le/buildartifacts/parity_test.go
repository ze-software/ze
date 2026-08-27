// VALIDATES: the ze-host-build and ze-installer-build-* producer contracts keep
// their argv, environment, artifact paths, writes metadata, and exit status.
// PREVENTS: a target GOARCH leaking into ze-host, or one installer architecture
// silently receiving the other architecture's compiler environment.

package buildartifacts

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leroot"
)

const (
	parityVersion   = "26.08.27"
	parityBuildDate = "2026-08-27T10:11:12Z"
	parityPin       = "go1.26.6"
)

func parityToolchain(root string) gotoolchain.Toolchain {
	return gotoolchain.Toolchain{
		Root:        root,
		GoToolchain: parityPin,
		Version:     parityVersion,
		BuildDate:   parityBuildDate,
	}
}

func TestInstallerPlansMatchBothPythonProducerBuilds(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "checkout")
	toolchain := parityToolchain(root)
	ldflags := "-X main.version=" + parityVersion + " -X main.buildDate=" + parityBuildDate

	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			plan := installerPlan(root, toolchain, arch)
			output := "bin/ze-installer-" + arch
			wantCommand := []string{
				"go", "build",
				"-tags", "ze_installer",
				"-ldflags", ldflags,
				"-o", output,
				"./cmd/ze-installer",
			}
			if !reflect.DeepEqual(plan.command, wantCommand) {
				t.Fatalf("command = %#v, want %#v", plan.command, wantCommand)
			}
			wantEnvironment := []string{
				"GOCACHE=" + filepath.Join(root, "cache", "go-cache"),
				"GOLANGCI_LINT_CACHE=" + filepath.Join(root, "tmp", "golangci-lint-cache"),
				"CGO_ENABLED=0",
				"GOTOOLCHAIN=" + parityPin,
				"GOOS=linux",
				"GOARCH=" + arch,
			}
			if got := toolchain.Overrides(plan.environ); !reflect.DeepEqual(got, wantEnvironment) {
				t.Fatalf("environment = %#v, want %#v", got, wantEnvironment)
			}
			if plan.gate != "ze-installer-build-"+arch || plan.output != output {
				t.Fatalf("plan identity = gate %q output %q", plan.gate, plan.output)
			}
		})
	}
}

func TestHostPlanUsesExactTagsVersionFlagsAndRootOutput(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "checkout")
	toolchain := parityToolchain(root)
	output := filepath.Join(root, "ze-host")
	want := []string{
		"go", "build",
		"-tags", "ze_core ze_setup",
		"-ldflags", "-X main.version=26.08.27 -X main.buildDate=2026-08-27T10:11:12Z",
		"-o", output,
		"./cmd/ze",
	}

	plan := hostPlan(root, toolchain)
	if !reflect.DeepEqual(plan.command, want) {
		t.Fatalf("command = %#v, want %#v", plan.command, want)
	}
	if plan.output != output {
		t.Fatalf("output = %q, want %q", plan.output, output)
	}
	if plan.gate != hostGate {
		t.Fatalf("gate = %q, want %q", plan.gate, hostGate)
	}
	wantEnvironment := []string{
		"GOCACHE=" + filepath.Join(root, "cache", "go-cache"),
		"GOLANGCI_LINT_CACHE=" + filepath.Join(root, "tmp", "golangci-lint-cache"),
		"CGO_ENABLED=0",
		"GOTOOLCHAIN=" + parityPin,
		"GOOS=" + runtime.GOOS,
		"GOARCH=" + runtime.GOARCH,
	}
	if got := toolchain.Overrides(plan.environ); !reflect.DeepEqual(got, wantEnvironment) {
		t.Fatalf("environment = %#v, want %#v", got, wantEnvironment)
	}
}

// The Python producer inherited GOOS and GOARCH for a host build. This test pins
// the corrected behavior: target values inherited from a cross build are
// replaced by this machine's values before the compiler starts.
func TestTargetArchitectureNeverLeaksIntoTheHostTool(t *testing.T) {
	t.Setenv("GOOS", "target-os")
	t.Setenv("GOARCH", "target-arch")
	root := t.TempDir()
	toolchain := parityToolchain(root)

	_ = installerPlan(root, toolchain, "arm64")
	host := hostPlan(root, toolchain)
	environment := effectiveEnvironment(toolchain.Environment(host.environ))
	if environment["GOOS"] != runtime.GOOS {
		t.Fatalf("host GOOS = %q, want %q", environment["GOOS"], runtime.GOOS)
	}
	if environment["GOARCH"] != runtime.GOARCH {
		t.Fatalf("host GOARCH = %q, want %q", environment["GOARCH"], runtime.GOARCH)
	}
	if environment["GOARCH"] == "target-arch" {
		t.Fatal("the target architecture reached the host build")
	}
}

func effectiveEnvironment(entries []string) map[string]string {
	environment := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			environment[key] = value
		}
	}
	return environment
}

func TestCompilerFailureIsTheActionExitAndTheStructuredReportCode(t *testing.T) {
	root := t.TempDir()
	toolchain := parityToolchain(root)
	plans := []buildPlan{
		hostPlan(root, toolchain),
		installerPlan(root, toolchain, "amd64"),
		installerPlan(root, toolchain, "arm64"),
	}

	for _, plan := range plans {
		t.Run(plan.gate, func(t *testing.T) {
			const compilerCode = 37
			var called bool
			run := func(gate string, argv []string, dir string, environ []string) int {
				called = true
				if gate != plan.gate || dir != root {
					t.Fatalf("runner got gate %q dir %q", gate, dir)
				}
				if !reflect.DeepEqual(argv, plan.command) {
					t.Fatalf("runner argv = %#v, want %#v", argv, plan.command)
				}
				if effectiveEnvironment(environ)["CGO_ENABLED"] != "0" {
					t.Fatalf("compiler environment did not force CGO_ENABLED=0: %#v", environ)
				}
				return compilerCode
			}

			report, code := execute(plan, toolchain, root, run)
			if !called {
				t.Fatal("the compiler runner was not called")
			}
			if code != compilerCode || report.Code != compilerCode {
				t.Fatalf("exit = %d report code = %d, want %d", code, report.Code, compilerCode)
			}
			if report.Output != plan.output || !report.Writes {
				t.Fatalf("write metadata = output %q writes %t", report.Output, report.Writes)
			}
			if !reflect.DeepEqual(report.Command, plan.command) {
				t.Fatalf("report command = %#v, want %#v", report.Command, plan.command)
			}
			if !reflect.DeepEqual(report.Environment, toolchain.Overrides(plan.environ)) {
				t.Fatalf("report environment = %#v", report.Environment)
			}
		})
	}
}

func TestSuccessfulBuildReportsTheArtifactAndZeroExit(t *testing.T) {
	root := t.TempDir()
	toolchain := parityToolchain(root)
	plans := []buildPlan{
		hostPlan(root, toolchain),
		installerPlan(root, toolchain, "amd64"),
		installerPlan(root, toolchain, "arm64"),
	}

	for _, plan := range plans {
		t.Run(plan.gate, func(t *testing.T) {
			report, code := execute(
				plan,
				toolchain,
				root,
				func(string, []string, string, []string) int { return 0 },
			)
			if code != 0 || report.Code != 0 {
				t.Fatalf("exit = %d report code = %d, want 0", code, report.Code)
			}
			if report.Output != plan.output || !report.Writes {
				t.Fatalf("write metadata = output %q writes %t", report.Output, report.Writes)
			}
		})
	}
}

func TestBuildAreaClaimsThreeWritingActions(t *testing.T) {
	list := Actions()
	wantGates := []string{
		"ze-host-build",
		"ze-installer-build-amd64",
		"ze-installer-build-arm64",
	}
	wantWhys := []string{
		"ze-host, the `ze appliance ...` driver that runs on the BUILD machine. " +
			"It owns the kernel cache key, so every QEMU target declares it as a " +
			"prerequisite before taking the staged-kernel guard",
		"the installer initrd PID 1 for amd64, at bin/ze-installer-amd64",
		"the installer initrd PID 1 for arm64, at bin/ze-installer-arm64",
	}
	if !reflect.DeepEqual(actions.Gates(), wantGates) {
		t.Fatalf("gates = %#v, want %#v", actions.Gates(), wantGates)
	}
	if len(list.Actions) != len(wantGates) {
		t.Fatalf("actions = %d, want %d", len(list.Actions), len(wantGates))
	}
	for index, row := range list.Actions {
		if row.Gate != wantGates[index] || row.Verb != wantGates[index] {
			t.Errorf("action %d identity = gate %q verb %q", index, row.Gate, row.Verb)
		}
		if !row.Writes {
			t.Errorf("action %s is not marked as writing", row.Gate)
		}
		if row.Why != wantWhys[index] {
			t.Errorf("action %s reason = %q, want %q", row.Gate, row.Why, wantWhys[index])
		}
		if len(row.Forks) != 0 {
			t.Errorf("action %s still forks %v", row.Gate, row.Forks)
		}
	}
	if !registry.HasLocal(leroot.CommandPath(area)) {
		t.Fatalf("importing buildartifacts did not register %q", area)
	}
}

func TestUnreadableToolchainFailsClosedBeforeTheCompiler(t *testing.T) {
	root := t.TempDir()
	var ran bool
	run := func(string, []string, string, []string) int {
		ran = true
		return 0
	}

	payload, code := runWithRoot(root, hostPlan, run)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if payload != nil {
		t.Fatalf("failure payload = %#v, want nil", payload)
	}
	if ran {
		t.Fatal("the compiler ran without a readable toolchain definition")
	}
}
