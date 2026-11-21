// Design: docs/architecture/testing/qemu-integration.md -- Docker labs on the docker-host image
// Related: dockerlab.go -- dockerLabKernelProvenance, dockerLabKernelBuilder

package testqemu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// TestDockerLabReportNamesTheKernelBuilder proves the lab report names the
// backend the booted kernel's provenance records, and in place of a backend it
// cannot name says "unknown: " with the refusal, whose fix removes the entry and
// then builds the kernel for the lab's arch with the host driver. Method: a temp
// entry whose kernel.version holds builder=docker, builder=qemu, builder=bogus,
// or is absent.
func TestDockerLabReportNamesTheKernelBuilder(t *testing.T) {
	const goarch = "arm64"
	cases := []struct {
		name   string
		record string   // "" writes no kernel.version
		want   string   // the exact report value; "" means an "unknown: " report
		holds  []string // what an "unknown: " report must carry
	}{
		{name: "docker", record: "builder=docker\n", want: "docker"},
		{name: "qemu", record: "builder=qemu\n", want: "qemu"},
		{name: "bogus", record: "builder=bogus\n", holds: []string{
			`"bogus"`, "rm -rf ", runtimeKernelBuildCommand(goarch),
			"ze-host appliance kernel --target runtime --arch " + goarch,
		}},
		{name: "no record", holds: []string{"read which backend built the runtime kernel", kernelbuilder.ProvenanceName}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kernel := t.TempDir()
			if tc.record != "" {
				if err := os.WriteFile(filepath.Join(kernel, kernelbuilder.ProvenanceName), []byte(tc.record), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got := dockerLabKernelBuilder(kernel, goarch)
			if tc.want != "" {
				if got != tc.want {
					t.Fatalf("report builder = %q, want %q", got, tc.want)
				}
				return
			}
			if !strings.HasPrefix(got, "unknown: ") {
				t.Fatalf("report builder = %q, want an \"unknown: \" report", got)
			}
			for _, want := range append(tc.holds, kernel) {
				if !strings.Contains(got, want) {
					t.Errorf("report does not carry %q:\n%s", want, got)
				}
			}
		})
	}
}
