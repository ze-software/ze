package appliance

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// linuxLimitsPage is the register of Linux and library limitations that stop
// Ze implementing a feature. Each entry records the version its source was
// read at.
var linuxLimitsPage = filepath.Join("..", "..", "docs", "architecture", "linux-limitations.md")

var (
	limitHeading = regexp.MustCompile(`^### (LL-[A-Z0-9-]+)$`)
	limitField   = regexp.MustCompile(`^- \*\*(Kind|Effect|Status|Checked):\*\* (.*)$`)
	limitKernel  = regexp.MustCompile(`^Linux (\d+\.\d+(?:\.\d+)?), (\S+)$`)
	limitModule  = regexp.MustCompile(`^module (\S+) (v\S+), (\S+)$`)
)

// limitFieldValues lists the values each closed field accepts. Checked is
// open: limitKernel and limitModule judge it.
var limitFieldValues = map[string][]string{
	"Kind":   {"kernel", "library"},
	"Effect": {"blocks", "constrains design"},
	"Status": {"verified", "unverified"},
}

// limitRequiredFields are the fields every entry carries exactly once.
var limitRequiredFields = []string{"Kind", "Effect", "Status", "Checked"}

// linuxLimit is one register entry as the test reads it.
type linuxLimit struct {
	id      string
	kind    string
	kernel  string          // set when the entry was checked against a kernel
	module  string          // set when the entry was checked against a Go module
	version string          // the module version
	fields  map[string]bool // the fields seen, to refuse a repeat or an absence
}

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
	entries, problems := parseLinuxLimits(string(data))
	for _, p := range problems {
		t.Errorf("%s: %s", linuxLimitsPage, p)
	}
	if len(entries) == 0 {
		t.Fatalf("%s holds no LL- entry; the register is empty or its headings changed form", linuxLimitsPage)
	}
	modules := goModRequirements(t)
	for _, e := range entries {
		if e.kind == "kernel" {
			if e.kernel != defaultKernelVersion {
				t.Errorf("%s: entry %s was checked at Linux %q but Ze ships Linux %s (internal/appliance/kernel.version); "+
					"re-read the source the entry cites at %s, follow its Re-check step, then update its Checked line",
					linuxLimitsPage, e.id, e.kernel, defaultKernelVersion, defaultKernelVersion)
			}
			continue
		}
		if e.kind != "library" {
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

// VALIDATES: each form check of parseLinuxLimits refuses the malformed page
// it exists for, and accepts the well-formed variants.
// PREVENTS: an entry escaping the version comparison through a stray heading,
// a repeated field, a missing Status or Effect, or an impossible date.
func TestParseLinuxLimitsForm(t *testing.T) {
	const head = "# Register\n\n## Entries\n\n"
	const good = "### LL-GOOD\n\n- **Kind:** kernel\n- **Effect:** blocks\n" +
		"- **Status:** verified\n- **Checked:** Linux 7.2.9, 2026-10-20\n"
	cases := []struct {
		name string
		page string
		want string // a substring of the one problem expected, or "" for none
	}{
		{"well formed", head + good, ""},
		{"kernel pin X.Y", head + strings.Replace(good, "7.2.9", "7.3", 1), ""},
		{"heading after the entries", head + good + "\n## Left out\n\n### Anything\n", ""},
		{"stray heading", head + good + "\n### Not an id\n", "heading"},
		{"duplicate Kind", head + strings.Replace(good, "- **Effect:**", "- **Kind:** kernel\n- **Effect:**", 1), "more than one Kind"},
		{"duplicate Checked", head + good + "- **Checked:** Linux 7.2.9, 2026-10-21\n", "more than one Checked"},
		{"missing Status", head + strings.Replace(good, "- **Status:** verified\n", "", 1), "no Status"},
		{"bad Status", head + strings.Replace(good, "verified", "maybe", 1), "Status"},
		{"missing Effect", head + strings.Replace(good, "- **Effect:** blocks\n", "", 1), "no Effect"},
		{"bad Effect", head + strings.Replace(good, "blocks", "hinders", 1), "Effect"},
		{"impossible date", head + strings.Replace(good, "2026-10-20", "2026-13-45", 1), "date"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := parseLinuxLimits(tc.page)
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("want no problem, got %q", problems)
				}
				return
			}
			if len(problems) != 1 || !strings.Contains(problems[0], tc.want) {
				t.Fatalf("want one problem containing %q, got %q", tc.want, problems)
			}
		})
	}
}

// parseLinuxLimits reads the register text into entries, and returns one
// problem for each line or field in a form the test does not recognize, so an
// entry cannot escape the comparison by a typo. Entries are the `###` headings
// between `## Entries` and the next `## ` heading.
func parseLinuxLimits(page string) ([]linuxLimit, []string) {
	var entries []linuxLimit
	var problems []string
	inEntries := false
	for line := range strings.SplitSeq(page, "\n") {
		if strings.HasPrefix(line, "## ") {
			inEntries = line == "## Entries"
			continue
		}
		if !inEntries {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			m := limitHeading.FindStringSubmatch(line)
			if m == nil {
				problems = append(problems, "heading "+strconv.Quote(line)+
					" under ## Entries is not an entry id; write `### LL-<NAME>` in capitals, digits and dashes, "+
					"or move the section after the entries")
				continue
			}
			entries = append(entries, linuxLimit{id: m[1], fields: map[string]bool{}})
			continue
		}
		if len(entries) == 0 {
			continue
		}
		m := limitField.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		problems = append(problems, readLimitField(&entries[len(entries)-1], m[1], m[2])...)
	}
	for i := range entries {
		for _, field := range limitRequiredFields {
			if !entries[i].fields[field] {
				problems = append(problems, "entry "+entries[i].id+" has no "+field+
					" line; add `- **"+field+":** ...` in the form the page header gives")
			}
		}
	}
	return entries, problems
}

// readLimitField records one field line of entry e and returns its problems.
func readLimitField(e *linuxLimit, field, value string) []string {
	if e.fields[field] {
		return []string{"entry " + e.id + " has more than one " + field +
			" line; keep the one that is true and delete the others"}
	}
	e.fields[field] = true
	if field == "Checked" {
		return readLimitChecked(e, value)
	}
	if !slices.Contains(limitFieldValues[field], value) {
		return []string{"entry " + e.id + " has " + field + " " + strconv.Quote(value) +
			", which is not one of " + strings.Join(limitFieldValues[field], ", ")}
	}
	if field == "Kind" {
		e.kind = value
	}
	return nil
}

// readLimitChecked records a Checked value and returns its problems.
func readLimitChecked(e *linuxLimit, value string) []string {
	date := ""
	if m := limitKernel.FindStringSubmatch(value); m != nil {
		e.kernel, date = m[1], m[2]
	}
	if m := limitModule.FindStringSubmatch(value); m != nil {
		e.module, e.version, date = m[1], m[2], m[3]
	}
	if date == "" {
		return []string{"entry " + e.id + " has a Checked line in no recognized form: " + strconv.Quote(value) +
			"; write `Linux X.Y[.Z], YYYY-MM-DD` or `module <path> vX.Y.Z, YYYY-MM-DD`"}
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return []string{"entry " + e.id + " has a Checked date " + strconv.Quote(date) +
			" that is not a calendar date (YYYY-MM-DD): " + err.Error()}
	}
	return nil
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
