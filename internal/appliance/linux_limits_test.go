package appliance

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// linuxLimitsPage is the register of Linux and library limitations that stop
// Ze implementing a feature. Each entry records the version its source was
// read at.
var linuxLimitsPage = filepath.Join("..", "..", "docs", "architecture", "linux-limitations.md")

var (
	limitHeading   = regexp.MustCompile(`^### (LL-[A-Z0-9-]+)$`)
	limitKernel    = regexp.MustCompile(`^- \*\*Checked:\*\* Linux (\d+\.\d+\.\d+), \d{4}-\d{2}-\d{2}$`)
	limitModule    = regexp.MustCompile(`^- \*\*Checked:\*\* module (\S+) (v\S+), \d{4}-\d{2}-\d{2}$`)
	limitChecked   = regexp.MustCompile(`^- \*\*Checked:\*\*`)
	limitKindField = regexp.MustCompile(`^- \*\*Kind:\*\* (kernel|library)$`)
)

// linuxLimit is one register entry as the test reads it.
type linuxLimit struct {
	id      string
	kind    string
	kernel  string // set when the entry was checked against a kernel
	module  string // set when the entry was checked against a Go module
	version string // the module version
}

// VALIDATES: the Linux limitations register records, for every entry, the
// kernel release Ze ships (kernel.version) or the module version go.mod
// requires, so a kernel or library bump cannot land without the register
// being re-read at the new version.
// PREVENTS: a register whose "checked at" versions silently fall behind the
// kernel Ze runs, which is the review the owner ordered on 2026-10-20.
func TestLinuxLimitsCheckedAtShippedVersions(t *testing.T) {
	entries := readLinuxLimits(t)
	if len(entries) == 0 {
		t.Fatalf("%s holds no LL- entry; the register is empty or its headings changed form", linuxLimitsPage)
	}
	modules := goModRequirements(t)
	for _, e := range entries {
		if e.kind == "" {
			t.Errorf("%s: entry %s has no `- **Kind:** kernel|library` line", linuxLimitsPage, e.id)
			continue
		}
		if e.kind == "kernel" {
			if e.kernel != defaultKernelVersion {
				t.Errorf("%s: entry %s was checked at Linux %q but Ze ships Linux %s (internal/appliance/kernel.version); "+
					"re-read the source the entry cites at %s, follow its Re-check step, then update its Checked line",
					linuxLimitsPage, e.id, e.kernel, defaultKernelVersion, defaultKernelVersion)
			}
			continue
		}
		required, ok := modules[e.module]
		if !ok {
			t.Errorf("%s: entry %s names module %q, which go.mod does not require; correct the module path or remove the entry",
				linuxLimitsPage, e.id, e.module)
			continue
		}
		if e.version != required {
			t.Errorf("%s: entry %s was checked at %s %s but go.mod requires %s; "+
				"re-read the library source the entry cites at %s, follow its Re-check step, then update its Checked line",
				linuxLimitsPage, e.id, e.module, e.version, required, required)
		}
	}
}

// readLinuxLimits parses the register into entries. A Checked line in a form
// the test does not recognize fails the test, so an entry cannot escape the
// comparison by a typo.
func readLinuxLimits(t *testing.T) []linuxLimit {
	t.Helper()
	data, err := os.ReadFile(linuxLimitsPage)
	if err != nil {
		t.Fatalf("read %s: %v", linuxLimitsPage, err)
	}
	var entries []linuxLimit
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if m := limitHeading.FindStringSubmatch(line); m != nil {
			entries = append(entries, linuxLimit{id: m[1]})
			continue
		}
		if len(entries) == 0 {
			continue
		}
		cur := &entries[len(entries)-1]
		if m := limitKindField.FindStringSubmatch(line); m != nil {
			cur.kind = m[1]
			continue
		}
		if !limitChecked.MatchString(line) {
			continue
		}
		if m := limitKernel.FindStringSubmatch(line); m != nil {
			cur.kernel = m[1]
			continue
		}
		if m := limitModule.FindStringSubmatch(line); m != nil {
			cur.module, cur.version = m[1], m[2]
			continue
		}
		t.Errorf("%s: entry %s has a Checked line in no recognized form: %q; "+
			"write `- **Checked:** Linux X.Y.Z, YYYY-MM-DD` or `- **Checked:** module <path> vX.Y.Z, YYYY-MM-DD`",
			linuxLimitsPage, cur.id, line)
	}
	return entries
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
