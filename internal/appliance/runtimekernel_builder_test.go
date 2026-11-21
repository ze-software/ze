// Design: docs/architecture/appliance/build-artifacts.md -- a test boots the cached runtime kernel and never builds one
// Related: runtimekernel_cached.go -- RuntimeKernelBuilder, unknownRuntimeKernelBuilder

package appliance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestRuntimeKernelBuilderReadsTheProvenance proves RuntimeKernelBuilder
// answers the backend a cache entry's provenance records, refuses a record that
// names no backend Ze has with a fix that removes the entry and then builds the
// kernel for the entry's arch, and refuses an entry with no record. Method: a
// temp entry whose kernel.version holds builder=docker, builder=qemu,
// builder=bogus, or is absent.
func TestRuntimeKernelBuilderReadsTheProvenance(t *testing.T) {
	const arch = archARM64
	build := func(arch string) string { return "build the runtime kernel for " + arch }
	cases := []struct {
		name    string
		record  string // "" writes no kernel.version
		want    string // the backend answered; "" means a refusal
		unknown bool   // the refusal is the unknown-backend operator error
	}{
		{name: "docker", record: "version=" + defaultKernelVersion + "\nbuilder=docker\n", want: builderDocker},
		{name: "qemu", record: "version=" + defaultKernelVersion + "\nbuilder=qemu\n", want: builderQEMU},
		{name: "bogus", record: "version=" + defaultKernelVersion + "\nbuilder=bogus\n", unknown: true},
		{name: "no record"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := t.TempDir()
			if tc.record != "" {
				if err := os.WriteFile(filepath.Join(tree, kernelbuilder.ProvenanceName), []byte(tc.record), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var asked []string
			got, err := RuntimeKernelBuilder(tree, arch, func(arch string) string {
				asked = append(asked, arch)
				return build(arch)
			})
			if tc.want != "" {
				if err != nil {
					t.Fatalf("refused a record naming %s: %v", tc.want, err)
				}
				if got != tc.want {
					t.Fatalf("builder = %q, want %q", got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("answered %q for a record that names no backend Ze has", got)
			}
			if got != "" {
				t.Errorf("a refusal also answered the builder %q", got)
			}
			if !tc.unknown {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("refusal for a missing record does not carry the missing file: %v", err)
				}
				return
			}
			var operr *opererr.Error
			if !errors.As(err, &operr) {
				t.Fatalf("refusal is not an operator error: %T %v", err, err)
			}
			for _, want := range []string{tree, `"bogus"`} {
				if !strings.Contains(operr.Problem(), want) {
					t.Errorf("problem does not name %s:\n%s", want, operr.Problem())
				}
			}
			wantFix := "rm -rf " + shellArg(tree) + "\n" + build(arch)
			if operr.Fix() != wantFix {
				t.Errorf("fix = %q, want %q: remove the entry, then build the kernel for %s", operr.Fix(), wantFix, arch)
			}
			if len(asked) != 1 || asked[0] != arch {
				t.Errorf("asked the build command for %v, want once for %s", asked, arch)
			}
		})
	}
}
