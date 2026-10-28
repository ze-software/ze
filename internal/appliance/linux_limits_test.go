// Design: docs/architecture/linux-limitations.md

package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/linuxlimit"
)

// linuxLimitsPage is the register of Linux and library limitations that stop
// Ze implementing a feature. Each entry records the version its source was
// read at.
var linuxLimitsPage = filepath.Join("..", "..", "docs", "architecture", "linux-limitations.md")

// VALIDATES: the Linux limitations register records, for every entry, the
// kernel release Ze ships (kernel.version) or the module version go.mod
// requires, so a kernel or library bump cannot land without the register
// being re-read at the new version.
// PREVENTS: a register whose "checked at" versions silently fall behind the
// kernel Ze runs, which is the review the owner ordered on 2026-10-20.
func TestLinuxLimitsCheckedAtShippedVersions(t *testing.T) {
	data, err := os.ReadFile(linuxLimitsPage)
	if err != nil {
		t.Fatalf("read %s: %v", linuxLimitsPage, err)
	}
	entries, problems := linuxlimit.Parse(string(data))
	for _, p := range problems {
		t.Errorf("%s: %s", linuxLimitsPage, p)
	}
	if len(entries) == 0 {
		t.Fatalf("%s holds no LL- entry; the register is empty or its headings changed form", linuxLimitsPage)
	}
	modules := goModRequirements(t)
	for i := range entries {
		e := &entries[i]
		if e.Kind == "kernel" {
			if e.Kernel != defaultKernelVersion {
				t.Errorf("%s: entry %s was checked at Linux %q but Ze ships Linux %s (internal/appliance/kernel.version); "+
					"re-read the source the entry cites at %s, follow its Re-check step, then update its Checked line",
					linuxLimitsPage, e.ID, e.Kernel, defaultKernelVersion, defaultKernelVersion)
			}
			continue
		}
		if e.Kind != "library" {
			continue
		}
		required, ok := modules[e.Module]
		if !ok {
			t.Errorf("%s: entry %s names module %q, which go.mod does not require; correct the module path or remove the entry",
				linuxLimitsPage, e.ID, e.Module)
			continue
		}
		if e.Version != required {
			t.Errorf("%s: entry %s was checked at %s %s but go.mod requires %s; "+
				"re-read the library source the entry cites at %s, follow its Re-check step, then update its Checked line",
				linuxLimitsPage, e.ID, e.Module, e.Version, required, required)
		}
	}
}

// goModRequirements maps each module go.mod requires to its version.
func goModRequirements(t *testing.T) map[string]string {
	t.Helper()
	path := filepath.Join("..", "..", "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := map[string]string{}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "require "))
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "v") {
			continue
		}
		out[fields[0]] = fields[1]
	}
	return out
}
