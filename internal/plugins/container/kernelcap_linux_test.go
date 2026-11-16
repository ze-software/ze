// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier

//go:build linux

package container

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestContainerKernelCapabilitiesAreEnrolled proves every kernel capability
// the plugin declares is enrolled by the package init(), refuses a start
// when absent, is in use whenever the config holds a container to run (one
// whose enabled leaf is not false), and names its own pair of registered
// codes, spelled from the capability's name.
//
// Method: the declaration, the live enrolment and the code registry.
//
// MUTATION: remove the loop of register_kernelcap_linux.go and no capability
// is enrolled.
func TestContainerKernelCapabilitiesAreEnrolled(t *testing.T) {
	capabilities := containerKernelCapabilities()
	if len(capabilities) == 0 {
		t.Fatal("the plugin declares no kernel capability")
	}
	symbols := make([]string, 0, len(capabilities))
	for i := range capabilities {
		capability := &capabilities[i]
		symbols = append(symbols, capability.Kernel)

		name, prefixed := strings.CutPrefix(capability.Subsystem, "container-")
		if !prefixed {
			t.Fatalf("capability %s is not named container-NAME", capability.Subsystem)
		}
		if want := "doctor-container-kernel-" + name + "-unavailable"; capability.CodeAbsent != want {
			t.Errorf("capability %s names the absent code %s, want %s", capability.Subsystem, capability.CodeAbsent, want)
		}
		if want := "doctor-container-kernel-" + name + "-unknown"; capability.CodeUnknown != want {
			t.Errorf("capability %s names the unknown code %s, want %s", capability.Subsystem, capability.CodeUnknown, want)
		}
		for _, code := range []string{capability.CodeAbsent, capability.CodeUnknown} {
			if diagnostic.Lookup(code) == nil {
				t.Errorf("capability %s names code %s, which is not in the code registry", capability.Subsystem, code)
			}
		}
		if capability.Degrades != "" {
			t.Errorf("capability %s only degrades, want a refusal: a container without its limit must not run", capability.Subsystem)
		}
		if !slices.Contains(kernelcap.Enrolled(), capability.Subsystem) {
			t.Errorf("capability %s is not enrolled by the package init()", capability.Subsystem)
		}

		if capability.InUse(nil) {
			t.Errorf("capability %s is in use with a nil tree", capability.Subsystem)
		}
		if capability.InUse(config.NewTree()) {
			t.Errorf("capability %s is in use with no container root", capability.Subsystem)
		}
		empty := config.NewTree()
		empty.GetOrCreateContainer(configRoot)
		if capability.InUse(empty) {
			t.Errorf("capability %s is in use with a container root that holds no instance", capability.Subsystem)
		}
		configured := config.NewTree()
		configured.GetOrCreateContainer(configRoot).AddListEntry(listInstance, "web", config.NewTree())
		if !capability.InUse(configured) {
			t.Errorf("capability %s is not in use with a container whose enabled leaf is absent, which defaults to true", capability.Subsystem)
		}
		disabled := config.NewTree()
		stopped := config.NewTree()
		stopped.Set(leafEnabled, "false")
		disabled.GetOrCreateContainer(configRoot).AddListEntry(listInstance, "web", stopped)
		if capability.InUse(disabled) {
			t.Errorf("capability %s is in use with only a disabled container, which runs nothing", capability.Subsystem)
		}
		mixed := config.NewTree()
		running := config.NewTree()
		running.Set(leafEnabled, "true")
		mixedRoot := mixed.GetOrCreateContainer(configRoot)
		mixedRoot.AddListEntry(listInstance, "db", stopped)
		mixedRoot.AddListEntry(listInstance, "web", running)
		if !capability.InUse(mixed) {
			t.Errorf("capability %s is not in use with one enabled container beside a disabled one", capability.Subsystem)
		}
	}
	if !slices.Contains(symbols, "CONFIG_CFS_BANDWIDTH") {
		t.Errorf("the plugin enrolls %v, want the hard CPU limit's CONFIG_CFS_BANDWIDTH among them", symbols)
	}
}

// TestSysctlPresenceTellsAbsentFromUnknown proves the probe answers present
// for a setting that exists, absent for one that does not, and unknown, never
// absent, when the directory that would hold it cannot be read.
//
// Method: a temporary directory standing for /proc/sys/kernel.
func TestSysctlPresenceTellsAbsentFromUnknown(t *testing.T) {
	directory := t.TempDir()

	if got := sysctlPresence(directory); got.State != kernelcap.StateAbsent || got.Reason == nil {
		t.Errorf("a directory without the setting answered %s (reason %v), want absent with a reason", got.State, got.Reason)
	}

	if err := os.WriteFile(filepath.Join(directory, cpuBandwidthSysctl), []byte("5000\n"), 0o600); err != nil {
		t.Fatalf("writing the fixture setting failed: %v", err)
	}
	if got := sysctlPresence(directory); got.State != kernelcap.StatePresent || got.Reason != nil {
		t.Errorf("a directory with the setting answered %s (reason %v), want present with no reason", got.State, got.Reason)
	}

	missing := filepath.Join(directory, "no-such-directory")
	if got := sysctlPresence(missing); got.State != kernelcap.StateUnknown || got.Reason == nil {
		t.Errorf("a directory that does not exist answered %s (reason %v), want unknown with a reason", got.State, got.Reason)
	}
}
