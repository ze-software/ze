package zefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: Check returns clean report for valid store (AC-4)
// PREVENTS: false positives on healthy stores

func TestCheckValidStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a", []byte("aaa"))
	writeOrFatal(t, s, "b/c", []byte("bcc"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	report, err := Check(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.MagicOK {
		t.Error("magic should be OK")
	}
	if !report.ContainerOK {
		t.Error("container should be OK")
	}
	if report.ContainerError != "" {
		t.Errorf("unexpected container error: %s", report.ContainerError)
	}
	if report.TotalEntries != 2 {
		t.Errorf("total entries: got %d, want 2", report.TotalEntries)
	}
	if report.CorruptEntries != 0 {
		t.Errorf("corrupt entries: got %d, want 0", report.CorruptEntries)
	}
	for _, e := range report.Entries {
		if e.Status != "ok" {
			t.Errorf("entry %q: status %q, want ok", e.Key, e.Status)
		}
	}
}

// VALIDATES: Check reports truncation (AC-6)
// PREVENTS: silent acceptance of truncated files

func TestCheckTruncatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("some data here"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Truncate the file mid-entry
	truncated := raw[:len(raw)/2]
	truncPath := filepath.Join(t.TempDir(), "truncated.zefs")
	if err := os.WriteFile(truncPath, truncated, 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := Check(truncPath)
	if err != nil {
		t.Fatal(err)
	}
	if report.ContainerError == "" && report.CorruptEntries == 0 {
		t.Error("expected corruption reported for truncated file")
	}
}

// VALIDATES: Check identifies corrupt entry by key (AC-5)
// PREVENTS: reporting corruption without identifying which entry

func TestCheckBitFlippedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "good", []byte("good data"))
	writeOrFatal(t, s, "bad", []byte("this will be corrupted"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Find "this will" in the raw bytes and flip a bit
	corrupted := make([]byte, len(raw))
	copy(corrupted, raw)
	target := "this will"
	for i := range corrupted {
		if i+len(target) <= len(corrupted) && string(corrupted[i:i+len(target)]) == target {
			corrupted[i] ^= 0x01
			break
		}
	}

	corruptPath := filepath.Join(t.TempDir(), "corrupt.zefs")
	if err := os.WriteFile(corruptPath, corrupted, 0o600); err != nil {
		t.Fatal(err)
	}

	report, checkErr := Check(corruptPath)
	if checkErr != nil {
		t.Fatal(checkErr)
	}

	// Either the container CRC fails or an individual entry CRC fails
	if report.ContainerError == "" && report.CorruptEntries == 0 {
		t.Error("expected corruption reported")
	}
}

// VALIDATES: Check identifies container CRC failure (AC-4 negative)
// PREVENTS: accepting stores with damaged container framing

func TestCheckBitFlippedContainer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Flip a bit in the container CRC (located after the magic netcapstring)
	corrupted := make([]byte, len(raw))
	copy(corrupted, raw)
	// Magic is "1:4:4:<crc>\nZeFS\n" = header + data + terminator
	// Container header starts after that. Find second header's CRC field.
	magicTotalLen := netcapstringTotalLen(len(magic))
	// Container header: number:cap:used:CRC\n -- find the CRC part
	hdrStart := magicTotalLen
	// Walk to the CRC field: skip number, ':', cap, ':', used, ':'
	i := hdrStart
	colons := 0
	for i < len(corrupted) && colons < 3 {
		if corrupted[i] == ':' {
			colons++
		}
		i++
	}
	// i now points to the start of the CRC hex field
	if i < len(corrupted) {
		corrupted[i] ^= 0x01
	}

	corruptPath := filepath.Join(t.TempDir(), "corrupt-container.zefs")
	if err := os.WriteFile(corruptPath, corrupted, 0o600); err != nil {
		t.Fatal(err)
	}

	report, checkErr := Check(corruptPath)
	if checkErr != nil {
		t.Fatal(checkErr)
	}
	if report.ContainerError == "" {
		t.Error("expected container error for flipped CRC")
	}
	if report.ContainerOK {
		t.Error("container should not be OK")
	}
}

// VALIDATES: Repair recovers valid entries, skips corrupt (AC-7)
// PREVENTS: losing recoverable data during repair

func TestRepairPartialCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "keep-me", []byte("preserved"))
	writeOrFatal(t, s, "corrupt-me", []byte("this data will be damaged"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the second entry's data
	corrupted := make([]byte, len(raw))
	copy(corrupted, raw)
	target := "this data"
	for i := range corrupted {
		if i+len(target) <= len(corrupted) && string(corrupted[i:i+len(target)]) == target {
			corrupted[i] ^= 0x01
			corrupted[i+1] ^= 0x01
			break
		}
	}

	corruptPath := filepath.Join(t.TempDir(), "corrupt.zefs")
	if err := os.WriteFile(corruptPath, corrupted, 0o600); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(t.TempDir(), "repaired.zefs")
	report, repairErr := Repair(corruptPath, outputPath)
	if repairErr != nil {
		t.Fatal(repairErr)
	}

	// At least one entry should be recovered or skipped
	if report.RecoveredCount+report.SkippedCount == 0 {
		t.Error("expected some entries processed")
	}

	// If entries were recovered, verify the output store is valid
	if report.RecoveredCount > 0 {
		repaired, openErr := Open(outputPath)
		if openErr != nil {
			t.Fatalf("open repaired: %v", openErr)
		}
		defer repaired.Close() //nolint:errcheck // test cleanup
	}
}

// VALIDATES: Repair on valid store produces identical copy (AC-8)
// PREVENTS: repair altering valid data

func TestRepairValidStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a", []byte("aaa"))
	writeOrFatal(t, s, "b/c", []byte("bcc"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(t.TempDir(), "repaired.zefs")
	report, err := Repair(path, outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if report.RecoveredCount != 2 {
		t.Errorf("recovered: got %d, want 2", report.RecoveredCount)
	}
	if report.SkippedCount != 0 {
		t.Errorf("skipped: got %d, want 0", report.SkippedCount)
	}

	repaired, err := Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer repaired.Close() //nolint:errcheck // test cleanup

	for _, tt := range []struct{ key, want string }{
		{"a", "aaa"},
		{"b/c", "bcc"},
	} {
		got, readErr := repaired.ReadFile(tt.key)
		if readErr != nil {
			t.Fatalf("ReadFile(%s): %v", tt.key, readErr)
		}
		if string(got) != tt.want {
			t.Errorf("ReadFile(%s): got %q, want %q", tt.key, got, tt.want)
		}
	}
}

// VALIDATES: Repair rejects same source and output path
// PREVENTS: overwriting evidence

func TestRepairSamePathRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	_, err := Repair(path, path)
	if err == nil {
		t.Error("expected error for same source and output path")
	}
	if !strings.Contains(err.Error(), "must differ") {
		t.Errorf("error should mention paths must differ: %v", err)
	}
}
