// Design: docs/architecture/diagnostics/crash-capture.md -- crash-dump config extraction tests
//
// VALIDATES: AC-1 at the config layer: `set system crash-dump enabled true`
//            reaches the intent readiness is computed from, with the schema
//            defaults for anything the operator left alone.
// PREVENTS:  the shape that ends every one of these subtrees silently -- a
//            reader asserting .(bool) on a delivered value that arrives as a
//            string, so the operator's setting is discarded with no message.

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang so a text config parses
	"github.com/ze-software/ze/internal/core/crashlog"
)

// treeFrom parses config TEXT rather than building a tree by hand, because the
// leaf names and the container nesting are half of what these tests assert: a
// tree built with GetOrCreateContainer would pass with a schema that has no
// crash-dump container at all.
func treeFrom(t *testing.T, text string) *config.Tree {
	t.Helper()
	result, err := config.LoadConfig(text, "test.conf", nil)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return result.Tree
}

func TestExtractCrashDumpReadsTheSubtree(t *testing.T) {
	tree := treeFrom(t, `
system {
    crash-dump {
        enabled true
        reserve 32
        memory-image {
            enabled true
            reserve 512
        }
    }
}
`)
	cd := extractCrashDump(tree.GetContainer("system"))

	if !cd.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if cd.ReserveMegabytes != 32 {
		t.Fatalf("ReserveMegabytes = %d, want 32", cd.ReserveMegabytes)
	}
	if !cd.MemoryImage {
		t.Fatal("MemoryImage = false, want true")
	}
	if cd.MemoryImageMegabytes != 512 {
		t.Fatalf("MemoryImageMegabytes = %d, want 512", cd.MemoryImageMegabytes)
	}
}

func TestExtractCrashDumpUsesSchemaDefaults(t *testing.T) {
	// A subtree the operator only half wrote reads the schema defaults for the
	// rest, so an enabled box gets the documented 16 MiB rather than zero.
	tree := treeFrom(t, `
system {
    crash-dump {
        enabled true
    }
}
`)
	cd := extractCrashDump(tree.GetContainer("system"))

	if cd.ReserveMegabytes != crashDumpReserveDefault {
		t.Fatalf("ReserveMegabytes = %d, want the default %d", cd.ReserveMegabytes, crashDumpReserveDefault)
	}
	if cd.MemoryImageMegabytes != crashDumpMemoryImageDefault {
		t.Fatalf("MemoryImageMegabytes = %d, want the default %d", cd.MemoryImageMegabytes, crashDumpMemoryImageDefault)
	}
	if cd.MemoryImage {
		t.Fatal("MemoryImage = true, want the default false")
	}
}

func TestExtractCrashDumpAbsentSubtree(t *testing.T) {
	tree := treeFrom(t, "system {\n    host box\n}\n")
	cd := extractCrashDump(tree.GetContainer("system"))

	if cd.Enabled {
		t.Fatal("Enabled = true with no crash-dump container")
	}
	if cd.ReserveMegabytes != crashDumpReserveDefault {
		t.Fatalf("ReserveMegabytes = %d, want the default %d", cd.ReserveMegabytes, crashDumpReserveDefault)
	}
}

func TestCrashDumpReserveBoundsAreEnforcedAtParse(t *testing.T) {
	// The YANG range is what refuses an out-of-range reservation, and the
	// refusal names the bound so the operator does not have to look it up.
	cases := []struct {
		name  string
		text  string
		ok    bool
		bound string
	}{
		{name: "smallest reservation", text: "reserve 4", ok: true},
		{name: "largest reservation", text: "reserve 256", ok: true},
		{name: "one below the floor", text: "reserve 3", bound: "4..256"},
		{name: "one above the ceiling", text: "reserve 257", bound: "4..256"},
		{name: "smallest image reservation", text: "memory-image {\n            reserve 64\n        }", ok: true},
		{name: "largest image reservation", text: "memory-image {\n            reserve 1024\n        }", ok: true},
		{name: "one below the image floor", text: "memory-image {\n            reserve 63\n        }", bound: "64..1024"},
		{name: "one above the image ceiling", text: "memory-image {\n            reserve 1025\n        }", bound: "64..1024"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := "system {\n    crash-dump {\n        enabled true\n        " + tc.text + "\n    }\n}\n"
			_, err := config.LoadConfig(text, "test.conf", nil)
			if tc.ok {
				if err != nil {
					t.Fatalf("%s: %v, want accepted", tc.text, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s was accepted, want a refusal", tc.text)
			}
			if !strings.Contains(err.Error(), tc.bound) {
				t.Fatalf("error = %q, want the bound %q named", err, tc.bound)
			}
		})
	}
}

func TestExtractCrashDumpKeepsDefaultOutsideTheRange(t *testing.T) {
	// A value outside the range can only reach the extractor from a tree nobody
	// validated, because the parse above refuses it. The default is the safe
	// answer there: a reservation the kernel would refuse takes RAM and stores
	// nothing, so falling back beats passing the value through.
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	crash := sys.GetOrCreateContainer("crash-dump")
	crash.Set("enabled", "true")
	crash.Set("reserve", "4096")

	cd := extractCrashDump(tree.GetContainer("system"))

	if !cd.Enabled {
		t.Fatal("Enabled = false, want the leaf that IS in range to still be read")
	}
	if cd.ReserveMegabytes != crashDumpReserveDefault {
		t.Fatalf("ReserveMegabytes = %d, want the default %d", cd.ReserveMegabytes, crashDumpReserveDefault)
	}
}

func TestCrashDumpIntentCarriesEveryLeaf(t *testing.T) {
	// The intent is the one value that crosses from the config tree into the
	// crash subsystem, so a leaf missing from it is a leaf readiness cannot see.
	cd := CrashDumpConfig{Enabled: true, ReserveMegabytes: 32, MemoryImage: true, MemoryImageMegabytes: 512}
	want := crashlog.Intent{Enabled: true, ReserveMegabytes: 32, MemoryImage: true, MemoryImageMegabytes: 512}
	if got := cd.Intent(); got != want {
		t.Fatalf("Intent() = %+v, want %+v", got, want)
	}
}

func TestLoadCrashDumpIntentReadsANamedFile(t *testing.T) {
	// `ze support --config` and the offline `show crashes` both name a file, and
	// that path resolves no config store at all.
	path := filepath.Join(t.TempDir(), "ze.conf")
	text := "system {\n    crash-dump {\n        enabled true\n        reserve 64\n    }\n}\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	intent, err := LoadCrashDumpIntent(path)
	if err != nil {
		t.Fatalf("LoadCrashDumpIntent: %v", err)
	}
	if !intent.Enabled || intent.ReserveMegabytes != 64 {
		t.Fatalf("intent = %+v, want enabled with 64 megabytes", intent)
	}
}

func TestLoadCrashDumpIntentReportsAnUnreadableConfig(t *testing.T) {
	// A config that cannot be read is not a config that says capture is off.
	// Returning a zero intent would report a configured box as unconfigured.
	if _, err := LoadCrashDumpIntent(filepath.Join(t.TempDir(), "absent.conf")); err == nil {
		t.Fatal("LoadCrashDumpIntent on an absent file returned no error")
	}
}
