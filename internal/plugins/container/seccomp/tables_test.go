// Design: docs/architecture/container/runtime.md -- the system-call filter

//go:build linux

package seccomp

import (
	"maps"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// vendoredNumbers parses a vendored golang.org/x/sys/unix zsysnum file into
// call numbers by lower-case name, the form the tables hold.
func vendoredNumbers(t *testing.T, file string) map[string]uint32 {
	t.Helper()
	// The test runs in the package directory; the vendor tree is at the root.
	source, err := os.ReadFile("../../../../vendor/golang.org/x/sys/unix/" + file)
	if err != nil {
		t.Fatalf("reading the vendored %s: %v", file, err)
	}
	constant := regexp.MustCompile(`(?m)^\s+SYS_([A-Z0-9_]+)\s+= (\d+)$`)
	numbers := make(map[string]uint32)
	for _, match := range constant.FindAllStringSubmatch(string(source), -1) {
		number, err := strconv.ParseUint(match[2], 10, 32)
		if err != nil {
			t.Fatalf("the vendored %s gives SYS_%s the number %q: %v", file, match[1], match[2], err)
		}
		numbers[strings.ToLower(match[1])] = uint32(number)
	}
	// A parse that matched nothing would make the comparison below vacuous.
	if len(numbers) < 300 {
		t.Fatalf("the vendored %s yields %d call numbers, fewer than any Linux architecture has", file, len(numbers))
	}
	return numbers
}

// TestSyscallTablesMatchVendoredNumbers proves each table equals the
// vendored system-call number file of its architecture, in both
// directions: no call is missing, none is invented, none is renumbered. A
// bump of golang.org/x/sys that changes a file fails here until the table is
// regenerated.
//
// MUTATION: delete one entry of a table, or change one number; that table's
// comparison fails.
func TestSyscallTablesMatchVendoredNumbers(t *testing.T) {
	for _, row := range []struct {
		table *syscallTable
		file  string
	}{
		{&amd64Table, "zsysnum_linux_amd64.go"},
		{&arm64Table, "zsysnum_linux_arm64.go"},
	} {
		vendored := vendoredNumbers(t, row.file)
		for name, number := range vendored {
			held, found := row.table.numbers[name]
			if !found {
				t.Errorf("the %s table lacks %s (%d) from %s", row.table.goarch, name, number, row.file)
				continue
			}
			if held != number {
				t.Errorf("the %s table numbers %s %d, and %s numbers it %d", row.table.goarch, name, held, row.file, number)
			}
		}
		for name := range row.table.numbers {
			if _, found := vendored[name]; !found {
				t.Errorf("the %s table holds %s, which %s does not", row.table.goarch, name, row.file)
			}
		}
		if !maps.Equal(vendored, row.table.numbers) {
			t.Errorf("the %s table differs from %s", row.table.goarch, row.file)
		}
	}
}
