// Design: docs/architecture/diagnostics/crash-capture.md -- crash listing tests
//
// VALIDATES: AC-4 and AC-14: one listing carries both artifact kinds, each row
//            says which kind it is, and the rows are structured data with
//            lower kebab-case keys.
// PREVENTS:  a kernel report being invisible in `show crashes`, or arriving
//            with no field that tells it apart from a Go panic report.

package crashlog

import (
	"os"
	"path/filepath"
	"testing"
)

// writeArtifacts puts named files in dir and points the package at it. The
// listing reads the resolved crash directory, so the test sets that rather than
// threading a path through every call.
func writeArtifacts(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("report "+name+"\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	previous := crashDir
	crashDir = dir
	t.Cleanup(func() { crashDir = previous })
}

func TestListCrashesCarriesKind(t *testing.T) {
	writeArtifacts(t,
		"crash-20260905-100000.log",
		"crash-20260905-110000-dmesg-ramoops-0-kernel.log",
	)

	summaries := ListCrashes()
	if len(summaries) != 2 {
		t.Fatalf("ListCrashes returned %d entries, want 2", len(summaries))
	}

	// Newest first, so the kernel artifact leads.
	if summaries[0].Kind != KindKernel {
		t.Fatalf("newest entry kind = %q, want %q", summaries[0].Kind, KindKernel)
	}
	if summaries[1].Kind != KindPanic {
		t.Fatalf("older entry kind = %q, want %q", summaries[1].Kind, KindPanic)
	}
}

func TestCrashKindReadsTheName(t *testing.T) {
	// The kind is derived from the name, so there is no sidecar file to lose and
	// no second store to keep in step. A name with "kernel" anywhere other than
	// the suffix is still a Go panic report.
	cases := []struct{ name, want string }{
		{"crash-20260905-100000.log", KindPanic},
		{"crash-20260905-100000-dmesg-ramoops-0-kernel.log", KindKernel},
		{"crash-20260905-100000-kernel.log", KindKernel},
		{"crash-kernel-20260905-100000.log", KindPanic},
	}
	for _, tc := range cases {
		if got := CrashKind(tc.name); got != tc.want {
			t.Fatalf("CrashKind(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCrashListFieldsCarryKebabKeys(t *testing.T) {
	writeArtifacts(t, "crash-20260905-110000-dmesg-ramoops-0-kernel.log")

	rows := CrashListFields()
	if len(rows) != 1 {
		t.Fatalf("CrashListFields returned %d rows, want 1", len(rows))
	}
	for _, key := range []string{"name", "size", "kind"} {
		if _, ok := rows[0][key]; !ok {
			t.Fatalf("row has no %q key: %v", key, rows[0])
		}
	}
	if rows[0]["kind"] != KindKernel {
		t.Fatalf("kind = %v, want %q", rows[0]["kind"], KindKernel)
	}
}

func TestReadCrashReadsAKernelArtifact(t *testing.T) {
	// AC-5: `show crashes name <kernel artifact>` prints the report in full.
	// The name check in ReadCrash is prefix and suffix based, so a kernel
	// artifact has to pass it as well as a Go panic report.
	name := "crash-20260905-110000-dmesg-ramoops-0-kernel.log"
	writeArtifacts(t, name)

	if got := ReadCrash(name); got != "report "+name+"\n" {
		t.Fatalf("ReadCrash(%q) = %q, want the file content", name, got)
	}
}
