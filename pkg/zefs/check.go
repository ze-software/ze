// Design: docs/architecture/zefs-format.md -- corruption detection and recovery
// Overview: store.go -- BlobStore format and decode logic

package zefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const entryStatusParseError = "parse-error"

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

// MoveAside renames a store file or directory to <path>.replaced-<date> (local
// time), preserving the original for post-mortem. Callers MUST own the store
// before replacing it; opening a live store never quarantines it implicitly.
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
	magicData, _, magicNext, magicErr := DecodeNetcapstringRef(data, 0)
	if magicErr != nil {
		report.ContainerError = fmt.Sprintf("%s at offset 0: magic: %v", path, magicErr)
		return report, nil //nolint:nilerr // partial report with corruption info is the success path
	}
	if string(magicData) != magic {
		report.ContainerError = fmt.Sprintf("%s at offset 0: invalid magic %q", path, magicData)
		return report, nil
	}
	report.MagicOK = true

	// A bad outer checksum must remain bad even when its entries are recoverable.
	containerData, _, containerNext, containerErr := DecodeNetcapstringRef(data, magicNext)
	if containerErr == nil {
		report.ContainerOK = containerNext == len(data)
		if !report.ContainerOK {
			report.ContainerError = fmt.Sprintf("%s at offset %d: trailing bytes after container", path, containerNext)
		}
	} else {
		report.ContainerError = fmt.Sprintf("%s at offset %d: container: %v", path, magicNext, containerErr)
		var extractErr error
		containerData, extractErr = extractContainerData(data, magicNext)
		if extractErr != nil {
			return report, nil //nolint:nilerr // The report records corruption, not an I/O failure.
		}
	}
	dataOff, _, _, _, headerErr := netcapstringHeader(data, magicNext, false)
	if headerErr != nil {
		return report, nil //nolint:nilerr // Already reported as container corruption.
	}
	off := 0
	for off < len(containerData) {
		if containerData[off] == '\n' {
			break
		}
		if containerData[off] == 0 {
			break
		}
		if containerData[off] == ' ' {
			break
		}
		if len(report.Entries) == maxEntryCount {
			report.ContainerOK = false
			report.ContainerError = fmt.Sprintf("%s at offset %d: entry count exceeds maximum %d", path, dataOff+off, maxEntryCount)
			break
		}
		nameData, _, nameNext, nameErr := DecodeNetcapstringRef(containerData, off)
		if nameErr != nil {
			report.Entries = append(report.Entries, EntryStatus{
				Key:    fmt.Sprintf("%s <offset %d>", path, dataOff+off),
				Status: entryStatusParseError, Error: nameErr.Error(),
			})
			report.CorruptEntries++
			off = skipToNextEntry(containerData, skipToNextEntry(containerData, off))
			continue
		}
		valueData, _, valueNext, valueErr := DecodeNetcapstringRef(containerData, nameNext)
		status := EntryStatus{Key: string(nameData), Size: len(valueData), Status: "ok"}
		if valueErr != nil {
			status.Status = integrityStatus(valueErr)
			status.Error = fmt.Sprintf("%s at offset %d: %v", path, dataOff+nameNext, valueErr)
			valueNext = skipToNextEntry(containerData, nameNext)
		}
		if !fs.ValidPath(status.Key) {
			status.Status = entryStatusParseError
			status.Error = "invalid key"
		}
		if status.Key == "." {
			status.Status = entryStatusParseError
			status.Error = "invalid key"
		}
		report.Entries = append(report.Entries, status)
		if status.Status == "ok" {
			report.TotalEntries++
		} else {
			report.CorruptEntries++
		}
		off = valueNext
	}

	return report, nil
}

// Repair reads a potentially corrupt store and writes all recoverable
// entries to a new store at outputPath. The source file is never modified.
func Repair(srcPath, dstPath string) (report *RepairReport, retErr error) {
	if srcPath == dstPath {
		return nil, fmt.Errorf("zefs: repair: source and output paths must differ")
	}
	if _, err := os.Lstat(dstPath); !os.IsNotExist(err) {
		if err != nil {
			return nil, fmt.Errorf("zefs: repair output %s: %w", dstPath, err)
		}
		return nil, fmt.Errorf("zefs: repair output %s already exists", dstPath)
	}

	report = &RepairReport{
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
	magicData, _, magicNext, magicErr := DecodeNetcapstringRef(data, 0)
	if magicErr != nil || string(magicData) != magic {
		return report, nil //nolint:nilerr // empty report = nothing recoverable from non-ZeFS file
	}

	// Use lenient scan: parse container header to find data boundaries,
	// then scan entries individually (even if container CRC is bad)
	containerData, extractErr := extractContainerData(data, magicNext)
	if extractErr != nil || len(containerData) == 0 {
		return report, nil //nolint:nilerr // empty report = nothing recoverable
	}

	stage, stageErr := os.MkdirTemp(filepath.Dir(dstPath), ".zefs-repair-*")
	if stageErr != nil {
		return nil, stageErr
	}
	defer func() { retErr = errors.Join(retErr, os.RemoveAll(stage)) }()
	artifact := filepath.Join(stage, "repaired.zefs")
	dst, createErr := Create(artifact)
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
		if report.RecoveredCount+report.SkippedCount == maxEntryCount {
			break
		}
		if containerData[off] == '\n' || containerData[off] == 0 || containerData[off] == ' ' {
			break
		}

		nameData, _, nameNext, nameErr := DecodeNetcapstringRef(containerData, off)
		if nameErr != nil {
			report.Skipped = append(report.Skipped, EntryStatus{
				Key:    fmt.Sprintf("%s <container offset %d>", srcPath, off),
				Status: entryStatusParseError,
				Error:  nameErr.Error(),
			})
			report.SkippedCount++
			off = skipToNextEntry(containerData, skipToNextEntry(containerData, off))
			continue
		}

		valueData, _, valueNext, valueErr := DecodeNetcapstringRef(containerData, nameNext)
		if valueErr != nil {
			report.Skipped = append(report.Skipped, EntryStatus{
				Key:    string(nameData),
				Status: entryStatusParseError,
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
				Status: entryStatusParseError,
				Error:  "invalid key",
			})
			report.SkippedCount++
			continue
		}

		if writeErr := wl.WriteFile(key, valueData, 0); writeErr != nil {
			report.Skipped = append(report.Skipped, EntryStatus{
				Key:    key,
				Status: entryStatusParseError,
				Error:  writeErr.Error(),
			})
			report.SkippedCount++
			continue
		}

		report.Recovered = append(report.Recovered, key)
		report.RecoveredCount++
	}

	if err := wl.Release(); err != nil {
		dst.Close() //nolint:errcheck // failing path, release already failed
		return report, fmt.Errorf("zefs: repair: flush output: %w", err)
	}
	if err := dst.Close(); err != nil {
		return report, fmt.Errorf("zefs: repair: close output: %w", err)
	}
	// Hard-link publication is atomic and refuses every existing destination,
	// including aliases of the source. The private staging link is then removed.
	if err := os.Link(artifact, dstPath); err != nil {
		return report, fmt.Errorf("zefs: publish repaired blob %s: %w", dstPath, err)
	}

	return report, nil
}

// extractContainerData returns the container's data region without CRC verification.
// Used by Repair to access entries even when the container CRC is invalid.
func extractContainerData(data []byte, containerOff int) ([]byte, error) {
	off, _, used, _, err := netcapstringHeader(data, containerOff, false)
	if err != nil {
		return nil, err
	}
	used = min(used, len(data)-off)
	return data[off : off+used], nil
}

// skipToNextEntry follows a recoverable frame boundary, never a header-looking
// string inside corrupt payload data. If framing is lost, salvage stops.
func skipToNextEntry(data []byte, off int) int {
	start, capacity, _, _, err := netcapstringHeader(data, off, false)
	if err != nil {
		return len(data)
	}
	if capacity >= len(data)-start {
		return len(data)
	}
	return start + capacity + 1
}

func integrityStatus(err error) string {
	if strings.Contains(err.Error(), "CRC mismatch") {
		return "crc-mismatch"
	}
	if strings.Contains(err.Error(), "truncated") {
		return "truncated"
	}
	return entryStatusParseError
}

// CheckPath checks either a blob artifact or a live tree root (database/).
// A corrupt frame is reported in Entries; filesystem and security refusals
// return an error so CLI callers can distinguish corrupt from unreadable.
func CheckPath(path string) (*CheckReport, error) {
	info, err := os.Lstat(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("zefs: check %s: %w", path, err)
	}
	if info.IsDir() {
		report := &CheckReport{Path: path, MagicOK: true, ContainerOK: true}
		err := walkFrameTree(path, func(key string, frame []byte) error {
			data, _, next, decodeErr := DecodeNetcapstringRef(frame, 0)
			if decodeErr == nil {
				if next != len(frame) {
					decodeErr = fmt.Errorf("trailing bytes at offset %d", next)
				}
			}
			entry := EntryStatus{Key: key, Size: len(data), Status: "ok"}
			if decodeErr != nil {
				entry.Status = integrityStatus(decodeErr)
				entry.Error = fmt.Sprintf("%s: %v", filepath.Join(path, key), decodeErr)
				report.CorruptEntries++
			} else {
				report.TotalEntries++
			}
			report.Entries = append(report.Entries, entry)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return report, nil
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("zefs: check %s: refusing non-regular node %s", path, info.Mode())
	}
	return Check(path)
}

// RepairPath salvages a blob or framed tree into a new destination of the same
// shape. The source is never changed, and the destination MUST NOT exist.
func RepairPath(srcPath, dstPath string) (*RepairReport, error) {
	info, err := os.Lstat(filepath.Clean(srcPath))
	if err != nil {
		return nil, fmt.Errorf("zefs: repair %s: %w", srcPath, err)
	}
	if _, err := os.Lstat(dstPath); !os.IsNotExist(err) {
		if err != nil {
			return nil, fmt.Errorf("zefs: repair output %s: %w", dstPath, err)
		}
		return nil, fmt.Errorf("zefs: repair output %s already exists", dstPath)
	}
	if info.IsDir() {
		return repairFrameTree(srcPath, dstPath)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("zefs: repair %s: refusing non-regular node %s", srcPath, info.Mode())
	}
	return Repair(srcPath, dstPath)
}
