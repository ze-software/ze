// Design: docs/architecture/diagnostics/crash-capture.md -- kernel panic harvest and readiness
// Overview: kernel_linux.go -- the pstore reader and the machine probes this file reads through
// Overview: kernel_other.go -- the non-Linux stubs
// Related: persist.go -- the crash directory, the retention count and the artifact rotation
//
// A kernel panic leaves no process behind to write a report. The kernel writes
// the record itself into a memory region a warm reboot does not clear, and Ze
// reads it on the next healthy boot. The record then becomes an ordinary crash
// artifact, so the directory probe, the retention count, `show crashes` and the
// support bundle all carry it with no second store.

package crashlog

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Kind names what produced a crash artifact. `show crashes`, the support bundle
// and the web view all read this one spelling.
const (
	// KindPanic is a Go panic this process caught and wrote itself.
	KindPanic = "panic"
	// KindKernel is a kernel panic recovered from the reserved region.
	KindKernel = "kernel"
)

// ReserveRegionName is the name the appliance build gives the reserved memory
// region on the kernel command line, and the name ramoops is bound to. The
// build writes it and the readiness probe reads it back, so both sides name the
// region from this one declaration.
const ReserveRegionName = "zecrash"

// The bounds on a crash reservation, in megabytes. They are declared here
// because three surfaces enforce them: the `system crash-dump` YANG range an
// operator commits against, the `image.crash-dump` validator the appliance
// build runs, and the readiness answer that reports what the kernel booted with.
//
// 4 MiB is the smallest region ramoops carves usable zones out of. 256 MiB is
// far more than a backtrace needs, and past it the operator pays RAM on every
// boot for a record that never grows (R-4). A memory image is sized to RAM
// instead, so its own floor is higher and its ceiling is a staging budget.
const (
	ReserveMegabytesMin     uint16 = 4
	ReserveMegabytesMax     uint16 = 256
	MemoryImageMegabytesMin uint16 = 64
	MemoryImageMegabytesMax uint16 = 1024
)

// kernelFileSuffix marks a crash artifact recovered from the kernel. The kind is
// derived from the name, so one directory listing distinguishes the two kinds
// with no sidecar file and no second store. It is a SUFFIX rather than a prefix
// so the name still sorts by timestamp, which is the order the retention pass in
// persist.go depends on.
const kernelFileSuffix = "-kernel" + crashFileSuffix

// recordIDMax bounds the record identity a source may put in a file name. A
// pstore entry names itself, so the name is data this process did not author.
const recordIDMax = 32

// memoryImageHeadroomBytes is the free space a memory image needs beyond the
// image itself. The write is sequential and the filesystem still needs room for
// its own metadata and for the next userspace crash file, so an image sized to
// the last free byte would fail at the one moment it is needed (R-2).
const memoryImageHeadroomBytes int64 = 512 << 20

// KernelRecord is one pending kernel crash record as the platform source holds
// it. Text is the kernel's own message and backtrace, copied verbatim.
type KernelRecord struct {
	// ID is the source's identity for this record. The harvest clears the
	// record by this ID and derives the artifact name from it, so one record
	// harvested twice produces one artifact rather than two.
	ID   string
	When time.Time
	Text string
}

// KernelSource reads and clears the kernel crash records the platform keeps
// across a warm reboot. NOT safe for concurrent use: the harvest runs once, at
// boot, on the goroutine that starts the daemon.
type KernelSource interface {
	// Available reports whether the source can be read, and names what is
	// missing when it cannot. A source that cannot answer says so, so an
	// unmounted filesystem is never read as "no kernel crash happened".
	Available() (ok bool, reason string)
	// Pending returns every record waiting to be harvested, oldest first.
	Pending() ([]KernelRecord, error)
	// Clear removes one record from the source.
	Clear(id string) error
}

// openKernelSource is the seam the harvest reads the platform through. The
// platform implementation is chosen at build time; a test replaces it.
var openKernelSource = openPlatformKernelSource

// HarvestResult reports what one harvest did. Written names each artifact
// created, Retained counts the records deliberately left in the source for the
// next boot, and Reason says why any were left.
type HarvestResult struct {
	Written  []string
	Retained int
	Reason   string
}

// HarvestKernelCrashes turns every pending kernel crash record into a
// kernel-kind artifact in the crash directory, then clears the source.
//
// It runs once, early in daemon start, after the crash directory is resolved.
// Running it a second time with no new kernel crash writes nothing: the source
// is empty, and the artifact name is derived from the record, so even a record
// the source refused to clear lands on the same file.
func HarvestKernelCrashes() HarvestResult {
	Init()
	return harvestKernelCrashes(openKernelSource, crashDir, crashKeep)
}

func harvestKernelCrashes(open func() (KernelSource, error), dir string, keep int) HarvestResult {
	var tb textbuf.Buffer

	source, err := open()
	if err != nil {
		return HarvestResult{Reason: tb.Str("kernel crash source: ").Err(err).String()}
	}
	if ok, reason := source.Available(); !ok {
		return HarvestResult{Reason: reason}
	}

	records, err := source.Pending()
	if err != nil {
		return HarvestResult{Reason: tb.Reset().Str("read kernel crash records: ").Err(err).String()}
	}
	if len(records) == 0 {
		return HarvestResult{}
	}

	// R-9: a record is the only copy of what the kernel said. When the crash
	// directory did not resolve, every candidate was uncreatable or unwritable,
	// so the records stay where they are and the next boot tries again.
	if dir == "" {
		return HarvestResult{
			Retained: len(records),
			Reason:   "crash directory is not writable; kernel crash records are kept for the next boot",
		}
	}

	var result HarvestResult
	for i := range records {
		name, writeErr := writeKernelArtifact(dir, keep, records[i])
		if writeErr != nil {
			// R-9 again: the write failed, so the source keeps the record.
			result.Retained++
			if result.Reason == "" {
				result.Reason = tb.Reset().Str("write kernel crash artifact: ").Err(writeErr).String()
			}
			continue
		}
		result.Written = append(result.Written, name)

		// R-8: the source is cleared only after the artifact is on disk. A
		// source that refuses to clear leaves the record for the next boot,
		// which rewrites the same file rather than adding a second one.
		if clearErr := source.Clear(records[i].ID); clearErr != nil {
			if result.Reason == "" {
				result.Reason = tb.Reset().Str("clear kernel crash record: ").Err(clearErr).String()
			}
		}
	}
	return result
}

// writeKernelArtifact writes one record into the crash directory and returns the
// artifact name. The name is derived from the record, so the same record written
// twice lands on the same file.
func writeKernelArtifact(dir string, keep int, record KernelRecord) (string, error) {
	name := kernelArtifactName(record)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(buildKernelReport(record)), 0o600); err != nil {
		return "", err
	}
	rotateCrashFiles(dir, keep)
	return name, nil
}

// kernelArtifactName builds the artifact name for one record: the record's own
// time first so the listing stays chronological, then the record identity so two
// records from one boot cannot collide, then the kind suffix.
func kernelArtifactName(record KernelRecord) string {
	when := record.When
	if when.IsZero() {
		when = time.Now()
	}
	var tb textbuf.Buffer
	tb.Str(crashFilePrefix).Str(when.UTC().Format("20060102-150405"))
	if id := sanitizeRecordID(record.ID); id != "" {
		tb.Byte('-').Str(id)
	}
	return tb.Str(kernelFileSuffix).String()
}

// sanitizeRecordID reduces a source-supplied identity to the characters a crash
// file name may carry. The identity comes from the kernel's own filesystem, so
// it is data rather than a value this process chose.
func sanitizeRecordID(id string) string {
	var b []byte
	previousHyphen := true
	for i := 0; i < len(id) && len(b) < recordIDMax; i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b = append(b, c)
			previousHyphen = false
			continue
		}
		if c >= 'A' && c <= 'Z' {
			b = append(b, c+('a'-'A'))
			previousHyphen = false
			continue
		}
		if previousHyphen {
			continue
		}
		b = append(b, '-')
		previousHyphen = true
	}
	return strings.TrimSuffix(string(b), "-")
}

// buildKernelReport renders one record as a crash file. It carries the same
// metadata header a Go crash report carries, the kernel's own text, and the log
// ring of the boot that harvested it. That ring is the boot AFTER the fault, and
// the heading says so, because the ring of the crashed boot died with the kernel.
func buildKernelReport(record KernelRecord) string {
	var b []byte
	b = appendCrashMetadata(b)

	b = append(b, "\n=== Kernel Crash ===\n"...)
	b = append(b, "Kind: "...)
	b = append(b, KindKernel...)
	b = append(b, "\nRecord: "...)
	b = append(b, record.ID...)
	if !record.When.IsZero() {
		b = append(b, "\nRecorded: "...)
		b = append(b, record.When.UTC().Format(time.RFC3339)...)
	}
	b = append(b, '\n', '\n')
	b = append(b, record.Text...)
	if record.Text != "" && record.Text[len(record.Text)-1] != '\n' {
		b = append(b, '\n')
	}

	entries := slogutil.GlobalLogRing().Snapshot(64, "", "")
	if len(entries) > 0 {
		b = append(b, "\n=== Harvest Boot Log (last "...)
		b = strconv.AppendInt(b, int64(len(entries)), 10)
		b = append(b, " entries) ===\n"...)
		for i := range entries {
			b = append(b, entries[i].Timestamp.UTC().Format(time.RFC3339)...)
			b = append(b, " ["...)
			b = append(b, entries[i].Level...)
			b = append(b, "] "...)
			if entries[i].Component != "" {
				b = append(b, entries[i].Component...)
				b = append(b, ": "...)
			}
			b = append(b, entries[i].Message...)
			b = append(b, '\n')
		}
	}
	return string(b)
}

// Intent is the operator's configured crash-capture setting, lowered from the
// `system crash-dump` YANG subtree by the caller that holds the config tree.
// crashlog is a core package, so it reads no config of its own and the caller
// that has the tree supplies the four values.
type Intent struct {
	Enabled              bool
	ReserveMegabytes     uint16
	MemoryImage          bool
	MemoryImageMegabytes uint16
}

// MemoryImageReadiness answers whether a full memory image could be captured.
type MemoryImageReadiness struct {
	Configured     bool
	Armed          bool
	Reason         string
	ShortfallBytes int64
}

// Fields renders the answer as the structured block every surface carries.
// The keys are declared here and nowhere else, so `show crashes`, the support
// bundle and the web view cannot disagree about what a field is called.
func (m MemoryImageReadiness) Fields() map[string]any {
	fields := map[string]any{
		"configured": m.Configured,
		"armed":      m.Armed,
	}
	if m.Reason != "" {
		fields["reason"] = m.Reason
	}
	if m.ShortfallBytes > 0 {
		fields["shortfall-bytes"] = m.ShortfallBytes
	}
	return fields
}

// Readiness answers configured versus armed for kernel crash capture.
//
// The two fields come from different places on purpose, and neither is derived
// from the other. Configured is what the operator committed. Armed is what the
// RUNNING kernel carries, because a reservation is a boot argument: a commit
// looks like it took effect and does nothing until the next boot. Reporting one
// field would hide exactly that gap.
type Readiness struct {
	Configured        bool
	Armed             bool
	Reason            string
	Region            string
	ReserveMegabytes  uint16
	Directory         string
	DirectoryWritable bool
	PstoreAvailable   bool
	MemoryImage       MemoryImageReadiness
}

// Fields renders the answer as the structured block every surface carries. The
// keys are lower kebab-case and match the YANG leaves they report on, and they
// are declared here once so no surface can spell one differently.
func (r Readiness) Fields() map[string]any {
	fields := map[string]any{
		"configured":         r.Configured,
		"armed":              r.Armed,
		"directory":          r.Directory,
		"directory-writable": r.DirectoryWritable,
		"pstore-available":   r.PstoreAvailable,
		"memory-image":       r.MemoryImage.Fields(),
	}
	if r.Reason != "" {
		fields["reason"] = r.Reason
	}
	if r.Region != "" {
		fields["region"] = r.Region
		fields["reserve-megabytes"] = r.ReserveMegabytes
	}
	return fields
}

// bootReservation describes the crash reservation the running kernel booted
// with, read from its command line.
type bootReservation struct {
	Name      string
	Megabytes uint16
	Present   bool
}

// machine is the set of facts readiness reads off the running box. The platform
// files supply the real one, and a test supplies its own, so the configured
// versus armed logic is provable without a reserved region, a mounted pstore or
// a disk of a chosen size.
type machine struct {
	Reservation func() bootReservation
	Pstore      func() (ok bool, reason string)
	FreeBytes   func(dir string) (int64, error)
	TotalMemory func() int64
	ArchSupport func() (ok bool, reason string)
}

// platformMachine binds the probes for the platform this binary was built for.
func platformMachine() machine {
	return machine{
		Reservation: readBootReservation,
		Pstore:      platformPstoreAvailable,
		FreeBytes:   platformFreeBytes,
		TotalMemory: platformTotalMemoryBytes,
		ArchSupport: memoryImageArchSupport,
	}
}

// CrashReadiness answers configured versus armed for the intent the caller holds
// and the kernel this process is running on.
func CrashReadiness(intent Intent) Readiness {
	Init()
	return crashReadiness(intent, crashDir, platformMachine())
}

func crashReadiness(intent Intent, dir string, box machine) Readiness {
	reservation := box.Reservation()
	pstoreOK, pstoreReason := box.Pstore()

	readiness := Readiness{
		Configured:        intent.Enabled,
		Armed:             reservation.Present && pstoreOK,
		Region:            reservation.Name,
		ReserveMegabytes:  reservation.Megabytes,
		Directory:         dir,
		DirectoryWritable: dir != "",
		PstoreAvailable:   pstoreOK,
	}
	readiness.Reason = readinessReason(intent, reservation, pstoreOK, pstoreReason, readiness.DirectoryWritable)
	readiness.MemoryImage = memoryImageReadiness(intent, dir, box)
	return readiness
}

// readinessReason names the one thing an operator does next, in the order they
// do it: arm the reservation, then make the record readable, then make the crash
// directory writable.
func readinessReason(intent Intent, reservation bootReservation, pstoreOK bool, pstoreReason string, dirWritable bool) string {
	var tb textbuf.Buffer
	if !intent.Enabled {
		if reservation.Present {
			return "the running kernel carries a crash reservation, but system crash-dump enabled is not set"
		}
		return ""
	}
	if !reservation.Present {
		return tb.Str("crash capture is configured but the running kernel booted without a reservation; ").
			Str("build the appliance image with image.crash-dump and reboot to arm it").String()
	}
	if !pstoreOK {
		return tb.Reset().Str("the reservation is present but kernel crash records cannot be read: ").Str(pstoreReason).String()
	}
	if !dirWritable {
		return "no crash directory is writable, so a harvested record could not be stored"
	}
	return ""
}

// memoryImageReadiness answers the memory-image half. It is armed only when the
// architecture supports the capture kernel AND the target has room for the
// image, and it reports the shortfall in bytes ahead of time rather than failing
// at the one moment the image is needed (R-2).
func memoryImageReadiness(intent Intent, dir string, box machine) MemoryImageReadiness {
	readiness := MemoryImageReadiness{Configured: intent.MemoryImage}
	if !intent.MemoryImage {
		return readiness
	}
	if ok, reason := box.ArchSupport(); !ok {
		readiness.Reason = reason
		return readiness
	}
	if dir == "" {
		readiness.Reason = "no crash directory is writable, so a memory image has nowhere to go"
		return readiness
	}

	needed := memoryImageEstimateBytes(intent, box) + memoryImageHeadroomBytes
	free, err := box.FreeBytes(dir)
	if err != nil {
		var tb textbuf.Buffer
		readiness.Reason = tb.Str("cannot measure free space in ").Str(dir).Str(": ").Err(err).String()
		return readiness
	}
	if free < needed {
		readiness.ShortfallBytes = needed - free
		var tb textbuf.Buffer
		readiness.Reason = tb.Reset().Str("a memory image needs ").Int(needed).Str(" bytes in ").Str(dir).
			Str(" and ").Int(free).Str(" bytes are free").String()
		return readiness
	}
	readiness.Armed = true
	return readiness
}

// memoryImageEstimateBytes sizes the image. A full memory image is the size of
// RAM, so the configured reservation is a floor rather than the estimate: a
// target whose RAM cannot be read still needs what the operator asked for.
func memoryImageEstimateBytes(intent Intent, box machine) int64 {
	configured := int64(intent.MemoryImageMegabytes) << 20
	total := box.TotalMemory()
	if total > configured {
		return total
	}
	return configured
}

// readBootReservation reads the reservation off the running kernel command line.
func readBootReservation() bootReservation {
	cmdline, err := platformKernelCmdline()
	if err != nil {
		return bootReservation{}
	}
	return parseBootReservation(cmdline)
}

// parseBootReservation reads `reserve_mem=<size>M:<align>:<name>` and
// `ramoops.mem_name=<name>` out of a kernel command line. Both are required and
// both must name the same region: a region nothing binds to is memory taken from
// the operator for no capture, and a binding with no region captures nothing.
func parseBootReservation(cmdline string) bootReservation {
	var region string
	var megabytes uint16
	var bound string

	for token := range strings.FieldsSeq(cmdline) {
		if value, ok := strings.CutPrefix(token, "ramoops.mem_name="); ok {
			bound = value
			continue
		}
		value, ok := strings.CutPrefix(token, "reserve_mem=")
		if !ok {
			continue
		}
		size, name, ok := parseReserveMem(value)
		if !ok {
			continue
		}
		region, megabytes = name, size
	}

	if region == "" || bound == "" || region != bound {
		return bootReservation{}
	}
	return bootReservation{Name: region, Megabytes: megabytes, Present: true}
}

// parseReserveMem splits the `<size>M:<align>:<name>` value of a reserve_mem
// token. Only a megabyte size is read, because a megabyte size is the only form
// the appliance build writes.
func parseReserveMem(value string) (megabytes uint16, name string, ok bool) {
	size, rest, found := strings.Cut(value, ":")
	if !found {
		return 0, "", false
	}
	_, name, found = strings.Cut(rest, ":")
	if !found || name == "" {
		return 0, "", false
	}
	digits, found := strings.CutSuffix(size, "M")
	if !found {
		return 0, "", false
	}
	n, err := strconv.ParseUint(digits, 10, 16)
	if err != nil {
		return 0, "", false
	}
	return uint16(n), name, true
}
