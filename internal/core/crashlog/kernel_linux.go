// Design: docs/architecture/diagnostics/crash-capture.md -- the Linux pstore reader
// Overview: kernel.go -- the harvest and the readiness answer this file serves
//
// pstore is the kernel's own crash store. With CONFIG_PSTORE_RAM the ramoops
// backend writes the oops or panic log into a reserved memory region that a warm
// reboot does not clear, and exposes each record as one file under
// /sys/fs/pstore. Removing the file frees the region for the next fault, which
// is why the harvest clears a record only after the artifact is on disk.

//go:build linux

package crashlog

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// pstoreDir is where the kernel exposes the pstore filesystem. sysfs creates
	// the mount point when CONFIG_PSTORE is built in, so Ze mounts the
	// filesystem there and never creates the directory itself.
	pstoreDir = "/sys/fs/pstore"
	// procCmdline holds the command line the running kernel booted with, which
	// is where the reservation either is or is not.
	procCmdline = "/proc/cmdline"
	// procMeminfo holds the total RAM a memory image would have to hold.
	procMeminfo = "/proc/meminfo"

	// kernelRecordMax bounds one record. The record is kernel-authored but its
	// length is not a field this code trusts: the reader copies at most this
	// many bytes and never sizes an allocation from the file's own header.
	// A ramoops record_size is 16 KiB by default, so 1 MiB is generous.
	kernelRecordMax = 1 << 20
	// kernelRecordsMax bounds one harvest. A region sized in megabytes holds
	// tens of records, so a directory with more entries than this is a defect
	// rather than a fault history worth reading.
	kernelRecordsMax = 64
)

// pstoreSource reads the kernel's crash records out of the pstore filesystem.
type pstoreSource struct {
	dir string
}

func openPlatformKernelSource() (KernelSource, error) {
	return &pstoreSource{dir: pstoreDir}, nil
}

// Available mounts the pstore filesystem when it is not mounted yet, then
// reports whether records can be read. gokrazy mounts sysfs and nothing else, so
// the first boot after a fault finds the mount point empty and this call is what
// makes the record reachable.
func (s *pstoreSource) Available() (bool, string) {
	var tb textbuf.Buffer
	if s.mounted() {
		return true, ""
	}
	if _, err := os.Stat(s.dir); err != nil {
		return false, tb.Str(s.dir).Str(" is absent; the running kernel was built without CONFIG_PSTORE").String()
	}
	if err := unix.Mount("pstore", s.dir, "pstore", 0, ""); err != nil {
		return false, tb.Reset().Str("mount pstore on ").Str(s.dir).Str(": ").Err(err).String()
	}
	if !s.mounted() {
		return false, tb.Reset().Str("pstore mounted on ").Str(s.dir).Str(" but does not answer as a pstore filesystem").String()
	}
	return true, ""
}

// mounted reports whether s.dir is a pstore filesystem. The filesystem type is
// what answers, not the presence of the directory: sysfs creates an empty
// mount point whether or not pstore was ever mounted on it.
func (s *pstoreSource) mounted() bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(s.dir, &stat); err != nil {
		return false
	}
	return stat.Type == unix.PSTOREFS_MAGIC
}

// Pending returns every record the kernel left, oldest first. Every entry is
// harvested, not only the dmesg one: a record left behind holds part of the
// reserved region, so the next fault has less room to write into.
func (s *pstoreSource) Pending() ([]KernelRecord, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	if len(names) > kernelRecordsMax {
		names = names[:kernelRecordsMax]
	}

	records := make([]KernelRecord, 0, len(names))
	for _, name := range names {
		record, err := s.read(name)
		if err != nil {
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].When.Before(records[j].When) })
	return records, nil
}

func (s *pstoreSource) read(name string) (KernelRecord, error) {
	path, err := s.path(name)
	if err != nil {
		return KernelRecord{}, err
	}
	f, err := os.Open(path) //nolint:gosec // path validated by s.path against the pstore directory
	if err != nil {
		return KernelRecord{}, err
	}
	defer f.Close() //nolint:errcheck // read-only close

	text, err := io.ReadAll(io.LimitReader(f, kernelRecordMax))
	if err != nil {
		return KernelRecord{}, err
	}

	when := time.Now()
	if info, statErr := f.Stat(); statErr == nil {
		when = info.ModTime()
	}
	return KernelRecord{ID: name, When: when, Text: string(text)}, nil
}

// Clear removes one record, which frees its part of the reserved region.
func (s *pstoreSource) Clear(id string) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// path joins a record name onto the pstore directory. The name comes from the
// kernel's own filesystem, so it is checked rather than trusted: a separator or
// a parent reference would reach a file outside the store.
func (s *pstoreSource) path(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("kernel crash record name ").Quoted(name).Str(" is not a plain file name").String())
	}
	return filepath.Join(s.dir, name), nil
}

// platformPstoreAvailable answers whether kernel crash records can be read on
// this machine, and names what is missing when they cannot.
func platformPstoreAvailable() (bool, string) {
	source, err := openPlatformKernelSource()
	if err != nil {
		var tb textbuf.Buffer
		return false, tb.Str("open kernel crash source: ").Err(err).String()
	}
	return source.Available()
}

// platformKernelCmdline returns the command line the running kernel booted with.
func platformKernelCmdline() (string, error) {
	data, err := os.ReadFile(procCmdline)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// platformTotalMemoryBytes returns the target's RAM, or 0 when it cannot be
// read. A memory image is sized to RAM, so this is the estimate the space check
// works from.
func platformTotalMemoryBytes() int64 {
	data, err := os.ReadFile(procMeminfo)
	if err != nil {
		return 0
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "MemTotal:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 2 || fields[1] != "kB" {
			return 0
		}
		kb, err := parseUint63(fields[0])
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}

// platformFreeBytes answers the bytes available on the filesystem holding dir.
func platformFreeBytes(dir string) (int64, error) {
	free, err := diskspace.Free(dir)
	if err != nil {
		return 0, err
	}
	if free > 1<<62 {
		return 1 << 62, nil
	}
	return int64(free), nil //nolint:gosec // bounded above
}

// memoryImageArchSupport reports whether this build can stage a capture kernel,
// and names the limit when it cannot. gokrazy's reboot path kexecs on amd64
// only, so an arm64 appliance that accepted the option would capture nothing and
// say nothing.
func memoryImageArchSupport() (bool, string) {
	if runtime.GOARCH == "amd64" {
		return true, ""
	}
	var tb textbuf.Buffer
	return false, tb.Str("a memory image needs a kexec-staged capture kernel, which this build does not have on ").
		Str(runtime.GOARCH).Str("; only amd64 stages one").String()
}

// parseUint63 reads a non-negative decimal count that fits an int64.
func parseUint63(s string) (int64, error) {
	var n int64
	if s == "" {
		return 0, errors.New("empty number")
	}
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			var tb textbuf.Buffer
			return 0, errors.New(tb.Str("not a number: ").Quoted(s).String())
		}
		if n > (1<<62)/10 {
			var tb textbuf.Buffer
			return 0, errors.New(tb.Str("number too large: ").Quoted(s).String())
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}
