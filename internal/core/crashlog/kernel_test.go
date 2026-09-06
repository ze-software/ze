// Design: docs/architecture/diagnostics/crash-capture.md -- kernel harvest and readiness tests
//
// VALIDATES: AC-1, AC-2, AC-3, AC-6, AC-7, AC-11, AC-13, AC-14 and risks R-8 and
//            R-9: the harvest writes an artifact, clears the source only after
//            the write succeeds, keeps the record when it cannot write, and
//            answers configured and armed from two different sources.
// PREVENTS:  losing the only copy of what the kernel said, writing two artifacts
//            for one fault, and reporting a box as armed when the running kernel
//            booted without the reservation.
//
// The goal is to prove the harvest never loses a record and never writes two
// artifacts for one fault, and that configured and armed are answered from
// different sources. The method is a fake KernelSource and a fake machine, so
// every branch is reachable without a reserved region, a mounted pstore, or a
// disk of a chosen size.

package crashlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSource is a KernelSource under the test's control. Cleared records the
// IDs the harvest asked it to drop, so a test can assert the ORDER of write and
// clear rather than only the end state.
type fakeSource struct {
	available     bool
	reason        string
	records       []KernelRecord
	pendingErr    error
	clearErr      error
	cleared       []string
	pendingCalled int
}

func (f *fakeSource) Available() (bool, string) { return f.available, f.reason }

func (f *fakeSource) Pending() ([]KernelRecord, error) {
	f.pendingCalled++
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return f.records, nil
}

func (f *fakeSource) Clear(id string) error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.cleared = append(f.cleared, id)
	kept := f.records[:0]
	for _, r := range f.records {
		if r.ID != id {
			kept = append(kept, r)
		}
	}
	f.records = kept
	return nil
}

func openFake(f *fakeSource) func() (KernelSource, error) {
	return func() (KernelSource, error) { return f, nil }
}

// record builds one pending kernel record. The time is fixed so the artifact
// name it produces is deterministic, which is what the idempotence tests turn on.
func record(id string) KernelRecord {
	return KernelRecord{
		ID:   id,
		When: time.Date(2026, 9, 5, 10, 1, 0, 0, time.UTC),
		Text: "Kernel panic - not syncing: " + id + "\nCall Trace:\n  ze_fault+0x0/0x1\n",
	}
}

func TestHarvestWritesKernelArtifact(t *testing.T) {
	// AC-3: a pending record becomes an artifact in the probed directory, and
	// the artifact carries the kernel's own text.
	dir := t.TempDir()
	source := &fakeSource{available: true, records: []KernelRecord{record("dmesg-ramoops-0")}}

	result := harvestKernelCrashes(openFake(source), dir, 5)

	if len(result.Written) != 1 {
		t.Fatalf("Written = %v, want one artifact (reason %q)", result.Written, result.Reason)
	}
	if result.Retained != 0 {
		t.Fatalf("Retained = %d, want 0", result.Retained)
	}
	if !strings.HasSuffix(result.Written[0], "-kernel.log") {
		t.Fatalf("artifact %q does not carry the kernel suffix", result.Written[0])
	}

	content, err := os.ReadFile(filepath.Join(dir, result.Written[0]))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.Contains(string(content), "Kernel panic - not syncing: dmesg-ramoops-0") {
		t.Fatalf("artifact does not carry the kernel text:\n%s", content)
	}
	if !strings.Contains(string(content), "=== Ze Crash Report ===") {
		t.Fatalf("artifact does not carry the crash report header:\n%s", content)
	}
}

func TestHarvestClearsSourceAfterSuccess(t *testing.T) {
	// R-8: the source is cleared, and only after the artifact exists. The
	// assertion is that the file is on disk AND the ID was cleared, because
	// clearing first would pass a test that only checked the clear.
	dir := t.TempDir()
	source := &fakeSource{available: true, records: []KernelRecord{record("dmesg-ramoops-0")}}

	result := harvestKernelCrashes(openFake(source), dir, 5)

	if len(source.cleared) != 1 || source.cleared[0] != "dmesg-ramoops-0" {
		t.Fatalf("cleared = %v, want [dmesg-ramoops-0]", source.cleared)
	}
	if _, err := os.Stat(filepath.Join(dir, result.Written[0])); err != nil {
		t.Fatalf("artifact missing after clear: %v", err)
	}
}

func TestHarvestRetriesWhenDirectoryUnwritable(t *testing.T) {
	// R-9 and AC-7: no crash directory resolved, so the record is the only copy
	// of what the kernel said. It stays in the source for the next boot, and the
	// reason says so.
	source := &fakeSource{available: true, records: []KernelRecord{record("dmesg-ramoops-0")}}

	result := harvestKernelCrashes(openFake(source), "", 5)

	if result.Retained != 1 {
		t.Fatalf("Retained = %d, want 1", result.Retained)
	}
	if len(result.Written) != 0 {
		t.Fatalf("Written = %v, want nothing written", result.Written)
	}
	if len(source.cleared) != 0 {
		t.Fatalf("cleared = %v, want the record left in place", source.cleared)
	}
	if !strings.Contains(result.Reason, "not writable") {
		t.Fatalf("Reason = %q, want it to name the unwritable directory", result.Reason)
	}
}

func TestHarvestKeepsRecordWhenWriteFails(t *testing.T) {
	// The other half of R-9: the directory resolved but the write failed. The
	// record is still the only copy, so it stays.
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := &fakeSource{available: true, records: []KernelRecord{record("dmesg-ramoops-0")}}

	result := harvestKernelCrashes(openFake(source), dir, 5)

	if result.Retained != 1 {
		t.Fatalf("Retained = %d, want 1 (reason %q)", result.Retained, result.Reason)
	}
	if len(source.cleared) != 0 {
		t.Fatalf("cleared = %v, want the record left in place", source.cleared)
	}
}

func TestHarvestTwiceProducesOneArtifact(t *testing.T) {
	// AC-6: the same harvest run twice with no new fault leaves exactly one
	// artifact, and the second run finds nothing to do.
	dir := t.TempDir()
	source := &fakeSource{available: true, records: []KernelRecord{record("dmesg-ramoops-0")}}

	first := harvestKernelCrashes(openFake(source), dir, 5)
	second := harvestKernelCrashes(openFake(source), dir, 5)

	if len(first.Written) != 1 || len(second.Written) != 0 {
		t.Fatalf("first wrote %v, second wrote %v; want one then none", first.Written, second.Written)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want 1", len(entries))
	}
}

func TestHarvestRewritesOneArtifactWhenClearFails(t *testing.T) {
	// R-8 again, from the other side: a source that refuses to clear leaves the
	// record for the next boot. The artifact name is derived from the record, so
	// the next harvest overwrites the same file rather than adding a second one.
	dir := t.TempDir()
	source := &fakeSource{
		available: true,
		records:   []KernelRecord{record("dmesg-ramoops-0")},
		clearErr:  errors.New("read-only pstore"),
	}

	harvestKernelCrashes(openFake(source), dir, 5)
	harvestKernelCrashes(openFake(source), dir, 5)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want 1", len(entries))
	}
}

func TestWatchdogRebootProducesNoArtifact(t *testing.T) {
	// AC-11: a watchdog reboots a wedged box without a kernel fault, so the
	// kernel wrote no record and pstore is empty. Nothing is produced, and the
	// harvest reports no reason: an empty store is a normal, healthy answer.
	dir := t.TempDir()
	source := &fakeSource{available: true}

	result := harvestKernelCrashes(openFake(source), dir, 5)

	if len(result.Written) != 0 || result.Retained != 0 || result.Reason != "" {
		t.Fatalf("result = %+v, want an empty harvest", result)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("directory holds %d entries, want 0", len(entries))
	}
}

func TestHarvestWritesOneArtifactPerRecord(t *testing.T) {
	// Two faults in one boot are two records, and the artifact name carries the
	// record identity so the second cannot land on the first one's file.
	dir := t.TempDir()
	source := &fakeSource{available: true, records: []KernelRecord{
		record("dmesg-ramoops-0"),
		record("dmesg-ramoops-1"),
	}}

	result := harvestKernelCrashes(openFake(source), dir, 5)

	if len(result.Written) != 2 {
		t.Fatalf("Written = %v, want two artifacts (reason %q)", result.Written, result.Reason)
	}
	if result.Written[0] == result.Written[1] {
		t.Fatalf("both records landed on %q", result.Written[0])
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("directory holds %d entries, want 2", len(entries))
	}
}

func TestHarvestReportsUnavailableSource(t *testing.T) {
	// A source that cannot answer must say so. Reading its silence as "no fault
	// happened" is the failure this asserts against.
	dir := t.TempDir()
	source := &fakeSource{available: false, reason: "/sys/fs/pstore is absent"}

	result := harvestKernelCrashes(openFake(source), dir, 5)

	if result.Reason != "/sys/fs/pstore is absent" {
		t.Fatalf("Reason = %q, want the source's own reason", result.Reason)
	}
	if source.pendingCalled != 0 {
		t.Fatalf("Pending called %d times on an unavailable source, want 0", source.pendingCalled)
	}
}

// armedMachine is a box whose kernel booted with a valid reservation and whose
// pstore is readable.
func armedMachine() machine {
	return machine{
		Reservation: func() bootReservation {
			return bootReservation{Name: ReserveRegionName, Megabytes: 16, Present: true}
		},
		Pstore:      func() (bool, string) { return true, "" },
		FreeBytes:   func(string) (int64, error) { return 64 << 30, nil },
		TotalMemory: func() int64 { return 8 << 30 },
		ArchSupport: func() (bool, string) { return true, "" },
	}
}

// unarmedMachine is a box that was configured and has not been rebooted yet.
func unarmedMachine() machine {
	box := armedMachine()
	box.Reservation = func() bootReservation { return bootReservation{} }
	return box
}

func TestReadinessConfiguredVersusArmed(t *testing.T) {
	// AC-1 and AC-2: the two states are reported separately and come from
	// different sources. The intent decides configured; the running kernel
	// decides armed; neither is derived from the other.
	cases := []struct {
		name       string
		intent     Intent
		box        machine
		configured bool
		armed      bool
		wantReason string
	}{
		{
			name:       "configured on a kernel that has not been rebooted",
			intent:     Intent{Enabled: true, ReserveMegabytes: 16},
			box:        unarmedMachine(),
			configured: true,
			armed:      false,
			wantReason: "reboot to arm it",
		},
		{
			name:       "configured on a kernel that booted with the reservation",
			intent:     Intent{Enabled: true, ReserveMegabytes: 16},
			box:        armedMachine(),
			configured: true,
			armed:      true,
		},
		{
			name:       "not configured on a kernel that carries a reservation",
			intent:     Intent{},
			box:        armedMachine(),
			configured: false,
			armed:      true,
			wantReason: "enabled is not set",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			readiness := crashReadiness(tc.intent, t.TempDir(), tc.box)
			if readiness.Configured != tc.configured {
				t.Fatalf("Configured = %v, want %v", readiness.Configured, tc.configured)
			}
			if readiness.Armed != tc.armed {
				t.Fatalf("Armed = %v, want %v", readiness.Armed, tc.armed)
			}
			if tc.wantReason != "" && !strings.Contains(readiness.Reason, tc.wantReason) {
				t.Fatalf("Reason = %q, want it to contain %q", readiness.Reason, tc.wantReason)
			}
		})
	}
}

func TestReadinessReportsPstoreUnreadable(t *testing.T) {
	// The reservation is present, so the RAM is spent, and nothing can read the
	// record back. Armed must be false and the reason must name the store.
	box := armedMachine()
	box.Pstore = func() (bool, string) { return false, "mount pstore on /sys/fs/pstore: operation not permitted" }

	readiness := crashReadiness(Intent{Enabled: true, ReserveMegabytes: 16}, t.TempDir(), box)

	if readiness.Armed {
		t.Fatal("Armed = true with an unreadable record store")
	}
	if !strings.Contains(readiness.Reason, "operation not permitted") {
		t.Fatalf("Reason = %q, want the mount failure named", readiness.Reason)
	}
}

func TestReadinessReportsSpaceShortfall(t *testing.T) {
	// AC-13: the image is sized to RAM plus headroom. With less free space than
	// that, the memory image is configured and not armed, and the shortfall is
	// reported in bytes rather than discovered at the moment of a fault.
	box := armedMachine()
	box.TotalMemory = func() int64 { return 8 << 30 }
	box.FreeBytes = func(string) (int64, error) { return 1 << 30, nil }

	readiness := crashReadiness(Intent{
		Enabled:              true,
		ReserveMegabytes:     16,
		MemoryImage:          true,
		MemoryImageMegabytes: 256,
	}, t.TempDir(), box)

	image := readiness.MemoryImage
	if !image.Configured {
		t.Fatal("MemoryImage.Configured = false, want true")
	}
	if image.Armed {
		t.Fatal("MemoryImage.Armed = true with less free space than the image needs")
	}
	want := (8 << 30) + memoryImageHeadroomBytes - (1 << 30)
	if image.ShortfallBytes != want {
		t.Fatalf("ShortfallBytes = %d, want %d", image.ShortfallBytes, want)
	}
}

func TestReadinessRefusesMemoryImageOnUnsupportedArch(t *testing.T) {
	// AC-12 at the readiness layer. The config validator refuses the commit;
	// this asserts the report never claims armed on a build that stages no
	// capture kernel, whatever a config already on disk says.
	box := armedMachine()
	box.ArchSupport = func() (bool, string) { return false, "only amd64 stages one" }

	readiness := crashReadiness(Intent{Enabled: true, MemoryImage: true, MemoryImageMegabytes: 256}, t.TempDir(), box)

	if readiness.MemoryImage.Armed {
		t.Fatal("MemoryImage.Armed = true on an architecture that stages no capture kernel")
	}
	if !strings.Contains(readiness.MemoryImage.Reason, "amd64") {
		t.Fatalf("Reason = %q, want the architecture limit named", readiness.MemoryImage.Reason)
	}
}

func TestReadinessFieldsCarryKebabKeys(t *testing.T) {
	// AC-14: the payload every surface renders is structured data with
	// lower kebab-case keys, so `| json`, `| yaml` and `| table` each read it.
	fields := crashReadiness(Intent{Enabled: true, ReserveMegabytes: 16}, "/var/lib/ze/crash", armedMachine()).Fields()

	for _, key := range []string{"configured", "armed", "directory", "directory-writable", "pstore-available", "region", "reserve-megabytes", "memory-image"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("readiness fields have no %q key: %v", key, fields)
		}
	}
	image, ok := fields["memory-image"].(map[string]any)
	if !ok {
		t.Fatalf("memory-image is %T, want a nested block", fields["memory-image"])
	}
	if _, ok := image["configured"]; !ok {
		t.Fatalf("memory-image block has no configured key: %v", image)
	}
}

func TestParseBootReservation(t *testing.T) {
	// The reservation is only real when BOTH tokens are present and name the
	// same region: a region nothing binds to captures nothing, and a binding
	// with no region has nowhere to write.
	cases := []struct {
		name      string
		cmdline   string
		present   bool
		megabytes uint16
	}{
		{
			name:      "both tokens naming one region",
			cmdline:   "console=ttyS0 reserve_mem=16M:4096:zecrash ramoops.mem_name=zecrash ro",
			present:   true,
			megabytes: 16,
		},
		{name: "reservation with nothing bound to it", cmdline: "reserve_mem=16M:4096:zecrash", present: false},
		{name: "binding with no reservation", cmdline: "ramoops.mem_name=zecrash", present: false},
		{name: "tokens naming different regions", cmdline: "reserve_mem=16M:4096:other ramoops.mem_name=zecrash", present: false},
		{name: "size in an unsupported unit", cmdline: "reserve_mem=16K:4096:zecrash ramoops.mem_name=zecrash", present: false},
		{name: "value with no region name", cmdline: "reserve_mem=16M:4096 ramoops.mem_name=zecrash", present: false},
		{name: "empty command line", cmdline: "", present: false},
		{
			name:      "smallest reservation the schema allows",
			cmdline:   "reserve_mem=4M:4096:zecrash ramoops.mem_name=zecrash",
			present:   true,
			megabytes: ReserveMegabytesMin,
		},
		{
			name:      "largest reservation the schema allows",
			cmdline:   "reserve_mem=256M:4096:zecrash ramoops.mem_name=zecrash",
			present:   true,
			megabytes: ReserveMegabytesMax,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseBootReservation(tc.cmdline)
			if got.Present != tc.present {
				t.Fatalf("Present = %v, want %v (got %+v)", got.Present, tc.present, got)
			}
			if tc.present && got.Megabytes != tc.megabytes {
				t.Fatalf("Megabytes = %d, want %d", got.Megabytes, tc.megabytes)
			}
			if tc.present && got.Name != ReserveRegionName {
				t.Fatalf("Name = %q, want %q", got.Name, ReserveRegionName)
			}
		})
	}
}

func TestSanitizeRecordID(t *testing.T) {
	// The record name comes from the kernel's own filesystem, so it is data. It
	// must never reach a file path with a separator or a parent reference in it.
	cases := []struct{ in, want string }{
		{"dmesg-ramoops-0", "dmesg-ramoops-0"},
		{"../../etc/passwd", "etc-passwd"},
		{"DMESG-EFI-0", "dmesg-efi-0"},
		{"", ""},
		{"///", ""},
		{strings.Repeat("a", 64), strings.Repeat("a", recordIDMax)},
	}
	for _, tc := range cases {
		if got := sanitizeRecordID(tc.in); got != tc.want {
			t.Fatalf("sanitizeRecordID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
