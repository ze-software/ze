// Design: docs/architecture/zefs-format.md -- corruption detection and recovery
// Overview: store.go -- BlobStore format and decode logic

package zefs

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"
)

// EntryStatus describes the integrity state of a single store entry.
type EntryStatus struct {
	Key    string
	Size   int
	Status string // "ok", "crc-mismatch", "parse-error", "truncated"
	Error  string // detail when Status is not "ok"
}

// CheckReport is the result of a store integrity check.
type CheckReport struct {
	Path           string
	MagicOK        bool
	ContainerOK    bool
	ContainerError string
	Entries        []EntryStatus
	TotalEntries   int
	CorruptEntries int
}

// RepairReport is the result of a store repair operation.
type RepairReport struct {
	SourcePath     string
	OutputPath     string
	Recovered      []string
	Skipped        []EntryStatus
	RecoveredCount int
	SkippedCount   int
}

// MoveAside renames a store file to <path>.replaced-<date> (local time) so a
// fresh store can replace it without destroying the old one, which is kept for
// post-mortem. Returns the backup path. Shared by the config storage layer's
// corrupt-store self-heal (storage.NewBlob) and the `ze init --force` path.
func MoveAside(path string) (string, error) {
	dst := make([]byte, 0, len(path)+len(".replaced-")+len("2006-01-02T150405"))
	dst = append(dst, path...)
	dst = append(dst, ".replaced-"...)
	dst = time.Now().AppendFormat(dst, "2006-01-02T150405")
	dest := string(dst)
	if err := os.Rename(path, dest); err != nil {
		return "", fmt.Errorf("zefs: move aside %s: %w", path, err)
	}
	return dest, nil
}

// Check verifies the integrity of a ZeFS store at the given path.
// It reads the file, validates the magic, container framing, and every
// entry's CRC32c. Returns a structured report.
func Check(path string) (*CheckReport, error) {
	report := &CheckReport{Path: path}

	data, err := os.ReadFile(path) //nolint:gosec // caller-provided path for integrity check
	if err != nil {
		return nil, fmt.Errorf("zefs: check: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("zefs: check: empty file")
	}

	if len(data) > maxImportSize {
		return nil, fmt.Errorf("zefs: check: file exceeds maximum size %d bytes", maxImportSize)
	}

	// Check magic
	magicData, _, magicNext, magicErr := decodeNetcapstringRef(data, 0)
	if magicErr != nil {
		report.ContainerError = "magic: " + magicErr.Error()
		return report, nil //nolint:nilerr // partial report with corruption info is the success path
	}
	if string(magicData) != magic {
		report.ContainerError = "invalid magic: " + string(magicData)
		return report, nil
	}
	report.MagicOK = true

	// Check container
	containerData, _, _, containerErr := decodeNetcapstringRef(data, magicNext)
	if containerErr != nil {
		report.ContainerError = "container: " + containerErr.Error()
		return report, nil //nolint:nilerr // partial report with corruption info is the success path
	}
	report.ContainerOK = true

	// Check individual entries
	off := 0
	for off < len(containerData) {
		if containerData[off] == '\n' || containerData[off] == 0 || containerData[off] == ' ' {
			break
		}

		nameData, _, nameNext, nameErr := decodeNetcapstringRef(containerData, off)
		if nameErr != nil {
			report.Entries = append(report.Entries, EntryStatus{
				Key:    "<offset " + strconv.Itoa(off) + ">",
				Status: "parse-error",
				Error:  "entry name: " + nameErr.Error(),
			})
			report.CorruptEntries++
			break
		}
		off = nameNext

		valueData, _, valueNext, valueErr := decodeNetcapstringRef(containerData, off)
		if valueErr != nil {
			report.Entries = append(report.Entries, EntryStatus{
				Key:    string(nameData),
				Status: "parse-error",
				Error:  "entry data: " + valueErr.Error(),
			})
			report.CorruptEntries++
			break
		}
		off = valueNext

		report.Entries = append(report.Entries, EntryStatus{
			Key:    string(nameData),
			Size:   len(valueData),
			Status: "ok",
		})
		report.TotalEntries++

		if report.TotalEntries > maxEntryCount {
			report.ContainerError = "entry count exceeds maximum " + strconv.Itoa(maxEntryCount)
			return report, nil
		}
	}

	return report, nil
}

// Repair reads a potentially corrupt store and writes all recoverable
// entries to a new store at outputPath. The source file is never modified.
func Repair(srcPath, dstPath string) (*RepairReport, error) {
	if srcPath == dstPath {
		return nil, fmt.Errorf("zefs: repair: source and output paths must differ")
	}

	report := &RepairReport{
		SourcePath: srcPath,
		OutputPath: dstPath,
	}

	data, err := os.ReadFile(srcPath) //nolint:gosec // caller-provided path for repair
	if err != nil {
		return nil, fmt.Errorf("zefs: repair: %w", err)
	}
	if len(data) > maxImportSize {
		return nil, fmt.Errorf("zefs: repair: file exceeds maximum size %d bytes", maxImportSize)
	}

	// Try to parse magic
	magicData, _, magicNext, magicErr := decodeNetcapstringRef(data, 0)
	if magicErr != nil || string(magicData) != magic {
		return report, nil //nolint:nilerr // empty report = nothing recoverable from non-ZeFS file
	}

	// Use lenient scan: parse container header to find data boundaries,
	// then scan entries individually (even if container CRC is bad)
	containerData, extractErr := extractContainerData(data, magicNext)
	if extractErr != nil || len(containerData) == 0 {
		return report, nil //nolint:nilerr // empty report = nothing recoverable
	}

	dst, createErr := Create(dstPath)
	if createErr != nil {
		return nil, fmt.Errorf("zefs: repair: create output: %w", createErr)
	}

	wl, lockErr := dst.Lock()
	if lockErr != nil {
		dst.Close() //nolint:errcheck // failing path, lock already failed
		return nil, fmt.Errorf("zefs: repair: lock output: %w", lockErr)
	}

	off := 0
	for off < len(containerData) {
		if containerData[off] == '\n' || containerData[off] == 0 || containerData[off] == ' ' {
			break
		}

		nameData, _, nameNext, nameErr := decodeNetcapstringRef(containerData, off)
		if nameErr != nil {
			report.Skipped = append(report.Skipped, EntryStatus{
				Key:    "<offset " + strconv.Itoa(off) + ">",
				Status: "parse-error",
				Error:  nameErr.Error(),
			})
			report.SkippedCount++
			break
		}

		valueData, _, valueNext, valueErr := decodeNetcapstringRef(containerData, nameNext)
		if valueErr != nil {
			report.Skipped = append(report.Skipped, EntryStatus{
				Key:    string(nameData),
				Status: "parse-error",
				Error:  valueErr.Error(),
			})
			report.SkippedCount++
			off = skipToNextEntry(containerData, nameNext)
			continue
		}
		off = valueNext

		key := string(nameData)
		if !fs.ValidPath(key) || key == "." {
			report.Skipped = append(report.Skipped, EntryStatus{
				Key:    key,
				Status: "parse-error",
				Error:  "invalid key",
			})
			report.SkippedCount++
			continue
		}

		if writeErr := wl.WriteFile(key, valueData, 0); writeErr != nil {
			report.Skipped = append(report.Skipped, EntryStatus{
				Key:    key,
				Status: "parse-error",
				Error:  writeErr.Error(),
			})
			report.SkippedCount++
			continue
		}

		report.Recovered = append(report.Recovered, key)
		report.RecoveredCount++

		if report.RecoveredCount > maxEntryCount {
			break
		}
	}

	if err := wl.Release(); err != nil {
		dst.Close() //nolint:errcheck // failing path, release already failed
		return report, fmt.Errorf("zefs: repair: flush output: %w", err)
	}
	if err := dst.Close(); err != nil {
		return report, fmt.Errorf("zefs: repair: close output: %w", err)
	}

	return report, nil
}

// extractContainerData returns the container's data region without CRC verification.
// Used by Repair to access entries even when the container CRC is invalid.
func extractContainerData(data []byte, containerOff int) ([]byte, error) {
	if containerOff >= len(data) {
		return nil, fmt.Errorf("zefs: container offset past end")
	}

	off := containerOff

	// Scan number field
	numStart := off
	for off < len(data) && data[off] != ':' {
		off++
	}
	if off >= len(data) {
		return nil, fmt.Errorf("zefs: truncated container header")
	}
	off++ // skip ':'

	numStr := string(data[numStart : off-1])
	var number int
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("zefs: invalid container number field")
		}
		number = number*10 + int(c-'0')
	}
	if number <= 0 || number > maxNumberWidth {
		return nil, fmt.Errorf("zefs: invalid container number field")
	}

	// Parse cap field
	if off+number > len(data) {
		return nil, fmt.Errorf("zefs: truncated container capacity")
	}
	var cap_ int
	for i := range number {
		c := data[off+i]
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("zefs: invalid container capacity field")
		}
		cap_ = cap_*10 + int(c-'0')
	}
	off += number

	if off >= len(data) || data[off] != ':' {
		return nil, fmt.Errorf("zefs: missing colon after container capacity")
	}
	off++

	// Parse used field
	if off+number > len(data) {
		return nil, fmt.Errorf("zefs: truncated container used")
	}
	var used int
	for i := range number {
		c := data[off+i]
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("zefs: invalid container used field")
		}
		used = used*10 + int(c-'0')
	}
	if used > cap_ {
		used = cap_
	}
	off += number

	// Skip ':' + CRC (8 hex chars) + '\n'
	if off >= len(data) || data[off] != ':' {
		return nil, fmt.Errorf("zefs: missing colon after container used")
	}
	off++
	if off+8 >= len(data) {
		return nil, fmt.Errorf("zefs: truncated container CRC")
	}
	off += 8
	if data[off] != '\n' {
		return nil, fmt.Errorf("zefs: missing newline after container CRC")
	}
	off++

	if off+used > len(data) {
		used = len(data) - off
	}

	return data[off : off+used], nil
}

// skipToNextEntry advances past a malformed entry value by scanning for
// the next plausible netcapstring header (digit followed by colon).
func skipToNextEntry(data []byte, off int) int {
	for i := off + 1; i < len(data)-1; i++ {
		if data[i] >= '1' && data[i] <= '9' && data[i+1] == ':' {
			return i
		}
	}
	return len(data)
}
