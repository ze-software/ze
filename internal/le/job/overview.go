// Design: docs/contributing/running-commands.md -- waiting for another session's job
// Overview: job.go -- the admission whose registry this reads
// Related: registry.go -- the entries, and the scan whose counting this follows
// Related: waiter.go -- the waiter records
// Related: answer.go -- the `report` action that answers this
//
// `le job report` is what the main thread reads before it starts a heavy job:
// how many slots this checkout has, who holds them, who is queued, and what
// the machine is doing. It shows readings and decides nothing: no threshold
// is applied here, and admission keeps queueing whatever the reader decides.
//
// It is read-only on the terms waiters keep. It takes no registry lock,
// because taking one creates the lock file and holds up every admission for
// the length of a process-table scan. It writes no file, removes none and
// signals no process. The cost of reading unlocked is stated where it shows:
// an entry caught between its creation and its fields reads as unreadable for
// that instant, and counts as occupied, as admission counts it.

package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/hostload"
	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/procscan"
)

// slotsNotice is the fixed line that says what the slot count leaves out. A
// reader who sees free slots on a loaded machine needs it to know where the
// load can come from.
//
// A heavy le action holds a slot, a Docker lab and a QEMU guest included (the
// dispatcher admits it, central.go), unless its declaration says it takes none
// (leroot.RegisterNoSlot, leaction.Action.NoSlot): `le exec`, which becomes
// the program it carries, and the few actions that add no load. Those, and the
// work that never passed through this checkout's le, are outside the count.
const slotsNotice = "the slots count only jobs admitted through this checkout's registry (" + JobsDir +
	"): commands run under the raw-admission escape (ZE_ADMIT_RAW), le exec and the few actions declared slot-free," +
	" work started outside ./le and other checkouts appear only in the machine readings"

// commandShown bounds how much of a recorded command line one row prints. The
// field is written by another session's process and is unbounded.
const commandShown = 200

// neverRead is the reason of a reading nobody filled.
const neverRead = "never read"

// bytesPerGiB is the unit the report prints memory and disk in. The document
// carries bytes.
const bytesPerGiB = 1 << 30

// liveness is what the process table says about the process a registry file
// names. The zero value is no verdict at all.
type liveness uint8

const (
	// livenessUnspecified is the zero value and is never a verdict.
	livenessUnspecified liveness = iota
	// livenessRunning is a process the scan listed.
	livenessRunning
	// livenessGone is a process the scan did not list: it exited, or it is a
	// zombie, which the scan leaves out because it holds nothing.
	livenessGone
	// livenessUnknown is a process nobody could judge, because the process
	// table could not be read.
	livenessUnknown
)

// The words a row carries for what holds it.
const (
	wordRunning = "running"
	wordUnknown = "unknown"
	// wordUnreadable marks an entry whose fields could not be read. It names
	// no process, so there is no liveness to state.
	wordUnreadable = "unreadable"
)

// word answers the word a row carries for a process that is shown. A gone
// process is never shown, so asking for its word is a Ze defect.
func (l liveness) word() string {
	switch l {
	case livenessRunning:
		return wordRunning
	case livenessUnknown:
		return wordUnknown
	case livenessUnspecified, livenessGone:
		panic("BUG: a job report row was built for a process with no shown liveness")
	default:
		panic("BUG: unknown job report liveness")
	}
}

// Reading is one machine measurement: a value, or the reason there is none.
//
// A failed reading cannot be written as a number. The fields are unexported
// and only Measured and Unavailable build one, so a zero in the report is a
// reading that succeeded and was zero. The zero Reading is unavailable too:
// a field nobody filled says so and never prints 0.
type Reading struct {
	measured bool
	value    float64
	reason   string
}

// Measured answers a reading that succeeded. A value that is not a finite
// number is not a measurement and reads as unavailable.
func Measured(value float64) Reading {
	if math.IsNaN(value) {
		return Unavailable("the reading is not a number")
	}
	if math.IsInf(value, 0) {
		return Unavailable("the reading is not a finite number")
	}
	return Reading{measured: true, value: value}
}

// Unavailable answers a reading that failed or that this platform cannot
// provide, with the reason a reader is shown.
func Unavailable(reason string) Reading {
	if reason == "" {
		panic("BUG: an unavailable job report reading carries no reason")
	}
	return Reading{reason: reason}
}

// Value answers the measurement and whether there is one.
func (r Reading) Value() (float64, bool) {
	return r.value, r.measured
}

// Reason answers why there is no measurement, and "" when there is one.
func (r Reading) Reason() string {
	if r.measured {
		return ""
	}
	if r.reason == "" {
		return neverRead
	}
	return r.reason
}

// MarshalJSON renders {"value": n} for a measurement and {"unavailable":
// "<reason>"} for anything else, so no renderer can print a failed reading as
// a number.
func (r Reading) MarshalJSON() ([]byte, error) {
	if r.measured {
		return json.Marshal(struct {
			Value float64 `json:"value"`
		}{Value: r.value})
	}
	return json.Marshal(struct {
		Unavailable string `json:"unavailable"`
	}{Unavailable: r.Reason()})
}

// writeTo renders the reading for a person.
func (r Reading) writeTo(tb *textbuf.Buffer) {
	if r.writeUnavailable(tb) {
		return
	}
	if r.value == math.Trunc(r.value) && math.Abs(r.value) < 1<<53 {
		tb.Int(int64(r.value))
		return
	}
	tb.Float2(r.value)
}

// writeUnavailable renders a reading that has no value, and reports whether
// it did.
func (r Reading) writeUnavailable(tb *textbuf.Buffer) bool {
	if r.measured {
		return false
	}
	tb.Str("unavailable (").Str(r.Reason()).Byte(')')
	return true
}

// writeLoad renders a load average with its two decimals, as the system
// prints it.
func (r Reading) writeLoad(tb *textbuf.Buffer) {
	if r.writeUnavailable(tb) {
		return
	}
	tb.Float2(r.value)
}

// writeBytes renders a byte count in GiB with one decimal, the unit a person
// compares with what a job needs.
func (r Reading) writeBytes(tb *textbuf.Buffer) {
	if r.writeUnavailable(tb) {
		return
	}
	tb.Float(r.value/bytesPerGiB, 1).Str(" GiB")
}

// Machine is what the report shows of the machine. Every field is a Reading,
// so each one is shown on its own when another failed.
type Machine struct {
	Cores Reading `json:"cores"`
	// Load1, Load5 and Load15 are the load averages. A low load average alone
	// is never a go signal: read the available memory beside it.
	Load1  Reading `json:"load-1m"`
	Load5  Reading `json:"load-5m"`
	Load15 Reading `json:"load-15m"`
	// MemoryAvailable and MemoryTotal are in bytes.
	MemoryAvailable Reading `json:"memory-available-bytes"`
	MemoryTotal     Reading `json:"memory-total-bytes"`
	// DiskFreeTmp is the free space, in bytes, on the volume that holds this
	// checkout's tmp/, and DiskFreeBuildCache the same for the Go build cache.
	DiskFreeTmp        Reading `json:"disk-free-tmp-bytes"`
	DiskFreeBuildCache Reading `json:"disk-free-build-cache-bytes"`
	// LeTestProcesses and GoTestProcesses count the running `le test` and Go
	// test processes on the whole machine, whoever started them.
	LeTestProcesses Reading `json:"le-test-processes"`
	GoTestProcesses Reading `json:"go-test-processes"`
	// Volumes says whether tmp/ and the Go build cache share a volume
	// (volumesShared, volumesApart, or volumesUnknown and a reason). It is
	// not a Reading, so the document carries it beside the machine, as
	// Overview.DiskVolumes.
	Volumes string `json:"-"`
}

// What the report says about the two directories whose free space it shows.
// Any other text opens with volumesUnknown and carries the reason.
const (
	volumesShared  = "tmp and the Go build cache are on one volume"
	volumesApart   = "tmp and the Go build cache are on separate volumes"
	volumesUnknown = "unknown: "
)

// machineSources is every reader the machine section takes a reading from,
// and the two directories whose volumes it reads. A test replaces each one.
type machineSources struct {
	cores           func() int
	load            func() (hostload.Averages, error)
	memoryAvailable func() (uint64, error)
	memoryTotal     func() (uint64, error)
	leTests         func() (int, error)
	goTests         func() (int, error)
	// diskFree answers the bytes free on the volume that holds a directory,
	// and sameVolume whether two directories are on one volume.
	diskFree   func(dir string) (uint64, error)
	sameVolume func(dir, other string) (bool, error)
	// tmp is the checkout's tmp/, which is a link out of the tree, and
	// buildCache the Go build cache every le-driven command writes.
	tmp        string
	buildCache string
}

// machineSources answers the real readers for this admission's checkout: the
// machine through internal/core/hostload, the volumes through
// internal/core/diskspace.
func (a *Admission) machineSources() machineSources {
	return machineSources{
		cores:           runtime.NumCPU,
		load:            hostload.LoadAverages,
		memoryAvailable: hostload.MemoryAvailable,
		memoryTotal:     hostload.MemoryTotal,
		leTests:         hostload.HarnessProcesses,
		goTests:         hostload.GoTestProcesses,
		diskFree:        volumeFree,
		sameVolume:      sameVolume,
		tmp:             a.abs("tmp"),
		buildCache:      gotoolchain.GoCache(a.Root),
	}
}

// readMachine answers the machine section from its sources. Each reading is
// taken on its own: one that fails is unavailable with its reason, and the
// others are still read.
func readMachine(sources *machineSources) Machine {
	machine := Machine{
		Cores:              Measured(float64(sources.cores())),
		Load1:              Unavailable(neverRead),
		Load5:              Unavailable(neverRead),
		Load15:             Unavailable(neverRead),
		MemoryAvailable:    bytesReading(sources.memoryAvailable()),
		MemoryTotal:        bytesReading(sources.memoryTotal()),
		DiskFreeTmp:        bytesReading(sources.diskFree(sources.tmp)),
		DiskFreeBuildCache: bytesReading(sources.diskFree(sources.buildCache)),
		LeTestProcesses:    countReading(sources.leTests()),
		GoTestProcesses:    countReading(sources.goTests()),
	}

	averages, err := sources.load()
	if err != nil {
		failed := unavailableFor(err)
		machine.Load1, machine.Load5, machine.Load15 = failed, failed, failed
	} else {
		machine.Load1 = Measured(averages.One)
		machine.Load5 = Measured(averages.Five)
		machine.Load15 = Measured(averages.Fifteen)
	}

	machine.Volumes = volumesOf(sources)
	return machine
}

// volumesOf says whether the two directories share a volume, in the words the
// report prints.
func volumesOf(sources *machineSources) string {
	shared, err := sources.sameVolume(sources.tmp, sources.buildCache)
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str(volumesUnknown).Err(err).String()
	}
	if shared {
		return volumesShared
	}
	return volumesApart
}

// unavailableFor answers the unavailable reading of one failed read.
func unavailableFor(err error) Reading {
	reason := err.Error()
	if reason == "" {
		reason = "the reader failed and gave no reason"
	}
	return Unavailable(reason)
}

// bytesReading turns one byte count, or the error in its place, into a
// reading.
func bytesReading(bytes uint64, err error) Reading {
	if err != nil {
		return unavailableFor(err)
	}
	return Measured(float64(bytes))
}

// countReading turns one count, or the error in its place, into a reading.
func countReading(count int, err error) Reading {
	if err != nil {
		return unavailableFor(err)
	}
	return Measured(float64(count))
}

// volumeProbe answers the directory whose volume stands for dir: dir itself
// with its links followed, or its deepest ancestor that exists when dir has
// not been created yet, as a cache is on first use.
//
// A link whose target is gone is an error and never its parent: the parent of
// tmp/ is the checkout, which sits on another volume than the one tmp/ links
// to, and its free space would be shown under the wrong name.
func volumeProbe(dir string) (string, error) {
	probe := dir
	for {
		_, err := os.Lstat(probe)
		if err == nil {
			return filepath.EvalSymlinks(probe)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", err
		}
		probe = parent
	}
}

// volumeFree answers the bytes free on the volume that holds dir.
func volumeFree(dir string) (uint64, error) {
	probe, err := volumeProbe(dir)
	if err != nil {
		return 0, err
	}
	return diskspace.Free(probe)
}

// sameVolume answers whether two directories are on one volume, by the
// device each one's filesystem reports.
func sameVolume(dir, other string) (bool, error) {
	device, err := deviceOf(dir)
	if err != nil {
		return false, err
	}
	otherDevice, err := deviceOf(other)
	if err != nil {
		return false, err
	}
	return device.Dev == otherDevice.Dev, nil
}

// deviceOf answers the stat record of the directory that stands for dir's
// volume. The caller compares the device field, whose type differs between
// platforms.
func deviceOf(dir string) (*syscall.Stat_t, error) {
	probe, err := volumeProbe(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(probe)
	if err != nil {
		return nil, err
	}
	record, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("%s: this platform reports no device for a directory", probe)
	}
	return record, nil
}

// HolderRow is one registry entry that occupies a slot.
type HolderRow struct {
	Label string `json:"label"`
	// PID is absent for an entry whose fields could not be read.
	PID int `json:"pid,omitempty"`
	// Liveness is running, unknown (the process table could not be read) or
	// unreadable (the entry could not be read). All three occupy a slot.
	Liveness string `json:"liveness"`
	// AgeSeconds is absent when the entry recorded no start.
	AgeSeconds *int64 `json:"age-seconds,omitempty"`
	// Session and Agent are empty when the entry names none: a main thread
	// has no agent, and an entry an older le wrote has neither.
	Session string `json:"session"`
	Agent   string `json:"agent"`
	// Command is the recorded command line, shortened, as data.
	Command string `json:"command"`
	// Entry is the registry file, relative to the checkout root.
	Entry string `json:"entry"`
}

// WaiterRow is one job waiting for a slot.
type WaiterRow struct {
	Label string `json:"label"`
	PID   int    `json:"pid"`
	// Liveness is running, or unknown when the process table could not be
	// read.
	Liveness      string `json:"liveness"`
	WaitedSeconds int64  `json:"waited-seconds"`
	// Session and Agent are empty when the record names none.
	Session string `json:"session"`
	Agent   string `json:"agent"`
	Command string `json:"command"`
}

// Overview is the whole answer of `le job report`: the slots, who holds and
// who waits for them, and the machine.
//
// An entry or a waiter record whose process is gone is left out of the rows
// and the counts, because it holds nothing.
type Overview struct {
	// Slots is how many admitted jobs run at once (Admission.Slots).
	Slots int `json:"slots"`
	// Occupied counts what admission counts: every live entry, every entry
	// whose process could not be judged, and every unreadable entry.
	Occupied int `json:"occupied"`
	// Free is the slots left, and zero when more are occupied than exist.
	Free int `json:"free"`
	// ProcessTable is "read", or "unavailable: <reason>" when the scan failed
	// and every row's liveness is therefore unknown.
	ProcessTable string      `json:"process-table"`
	Holders      []HolderRow `json:"holders"`
	Waiters      []WaiterRow `json:"waiters"`
	Machine      Machine     `json:"machine"`
	// DiskVolumes says whether the two disk readings of Machine are one
	// volume read twice: "tmp and the Go build cache are on one volume", "...
	// on separate volumes", or "unknown: <reason>".
	DiskVolumes string `json:"disk-volumes"`
	Notice      string `json:"notice"`
}

// overviewPass is what one report reads besides the registry: the process
// scanner, the machine reader and the clock. A test replaces each one.
type overviewPass struct {
	scan    func() ([]procscan.Process, error)
	machine func() Machine
	now     time.Time
}

// Overview answers the report for this admission's checkout.
//
// It only reads. Safe to call while other sessions hold and wait for slots:
// a file removed between the listing and its read is a job that ended, and
// is left out.
//
// The answer is a pointer because the document is large and its prose
// rendering (Text) has a pointer receiver.
func (a *Admission) Overview() (*Overview, error) {
	machine := func() Machine {
		sources := a.machineSources()
		return readMachine(&sources)
	}
	return a.overview(overviewPass{scan: procscan.Scan, machine: machine, now: time.Now()})
}

// overview is Overview with the pass a test replaces.
//
// The process table is scanned ONCE, whatever the registry holds: admission's
// own liveness test forks one `ps` per entry, and the directory keeps
// thousands of files.
func (a *Admission) overview(pass overviewPass) (*Overview, error) {
	view := &Overview{
		Slots:        a.Slots,
		ProcessTable: "read",
		Holders:      []HolderRow{},
		Waiters:      []WaiterRow{},
		Machine:      pass.machine(),
		Notice:       slotsNotice,
	}
	view.DiskVolumes = view.Machine.Volumes
	if view.DiskVolumes == "" {
		view.DiskVolumes = volumesUnknown + neverRead
	}

	listed, err := os.ReadDir(a.jobsDir())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// A registry that cannot be listed has no slot count to state, and an
		// empty report would read as an idle machine.
		return nil, fmt.Errorf("list the job registry %s: %w", JobsDir, err)
	}

	table := scanTable(pass.scan)
	if table.err != nil {
		var tb textbuf.Buffer
		view.ProcessTable = tb.Str("unavailable: ").Err(table.err).String()
	}

	// os.ReadDir answers in name order, which is the order of the rows.
	for _, listing := range listed {
		path := filepath.Join(a.jobsDir(), listing.Name())
		if strings.HasSuffix(listing.Name(), entrySuffix) {
			if row, occupies := a.holderRow(path, table, pass.now); occupies {
				view.Holders = append(view.Holders, row)
			}
			continue
		}
		if !strings.HasSuffix(listing.Name(), waiterSuffix) {
			continue
		}
		// A waiter record is a plain file its writer created here. Anything
		// else under that name could point outside the registry.
		if !listing.Type().IsRegular() {
			continue
		}
		if row, waits := waiterRow(path, table, pass.now); waits {
			view.Waiters = append(view.Waiters, row)
		}
	}

	view.Occupied = len(view.Holders)
	view.Free = max(view.Slots-view.Occupied, 0)
	return view, nil
}

// processTable is one scan of the running processes, or why there is none.
type processTable struct {
	running map[int]struct{}
	err     error
}

// scanTable reads the process table once.
func scanTable(scan func() ([]procscan.Process, error)) processTable {
	processes, err := scan()
	if err != nil {
		return processTable{err: err}
	}
	running := make(map[int]struct{}, len(processes))
	for _, process := range processes {
		running[process.PID] = struct{}{}
	}
	return processTable{running: running}
}

// judge answers what the scan says about one process.
func (t processTable) judge(pid int) liveness {
	if t.err != nil {
		return livenessUnknown
	}
	if _, listed := t.running[pid]; listed {
		return livenessRunning
	}
	return livenessGone
}

// holderRow answers the row of one entry, and whether the entry occupies a
// slot. It counts as scanAndClaim counts: an unreadable entry and a process
// nobody could judge are occupied, and a process that is gone is not.
func (a *Admission) holderRow(path string, table processTable, now time.Time) (HolderRow, bool) {
	held, readable := a.readEntry(path)
	if !readable {
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			// Removed between the listing and the read: its job ended.
			return HolderRow{}, false
		}
		return HolderRow{
			Label:    strings.TrimSuffix(filepath.Base(path), entrySuffix),
			Liveness: wordUnreadable,
			Entry:    a.rel(path),
		}, true
	}

	verdict := table.judge(held.pid)
	if verdict == livenessGone {
		return HolderRow{}, false
	}

	row := HolderRow{
		Label:    held.label,
		PID:      held.pid,
		Liveness: verdict.word(),
		Session:  held.who.session,
		Agent:    held.who.agent,
		Command:  truncate(held.command, commandShown),
		Entry:    a.rel(path),
	}
	if held.startedKnown {
		age := int64(max(now.Sub(held.started), 0) / time.Second)
		row.AgeSeconds = &age
	}
	return row, true
}

// waiterRow answers the row of one waiter record, and whether it names a job
// that is still waiting.
func waiterRow(path string, table processTable, now time.Time) (WaiterRow, bool) {
	waiting, readable := readWaiter(path)
	if !readable {
		return WaiterRow{}, false
	}
	verdict := table.judge(waiting.pid)
	if verdict == livenessGone {
		return WaiterRow{}, false
	}
	return WaiterRow{
		Label:         waiting.label,
		PID:           waiting.pid,
		Liveness:      verdict.word(),
		WaitedSeconds: int64(max(now.Sub(waiting.since), 0) / time.Second),
		Session:       waiting.who.session,
		Agent:         waiting.who.agent,
		Command:       truncate(waiting.command, commandShown),
	}, true
}

// Text renders the report for a person. It ends in a newline.
//
// This is the prose rendering leroot uses for the bare command, and every
// pipe operator bypasses it.
func (o *Overview) Text() string {
	var tb textbuf.Buffer
	tb.Str("job slots: ").Int(int64(o.Slots)).Str(" total, ").Int(int64(o.Occupied)).
		Str(" occupied, ").Int(int64(o.Free)).Str(" free\n")
	if o.ProcessTable != "read" {
		tb.Str("process table: ").Str(o.ProcessTable).Str(", so every liveness below is unknown\n")
	}

	tb.Str("holders:\n")
	if len(o.Holders) == 0 {
		tb.Str("  none\n")
	}
	for _, row := range o.Holders {
		tb.Str("  ").Str(row.Label)
		if row.PID != 0 {
			tb.Str(" pid ").Int(int64(row.PID))
		}
		tb.Str(", ").Str(row.Liveness)
		if row.AgeSeconds != nil {
			tb.Str(", ").Int(*row.AgeSeconds).Str("s")
		}
		writeOwnerText(&tb, row.Session, row.Agent)
		tb.Str(", entry ").Str(row.Entry).Byte('\n')
		if row.Command != "" {
			tb.Str("    ").Str(row.Command).Byte('\n')
		}
	}

	tb.Str("waiters:\n")
	if len(o.Waiters) == 0 {
		tb.Str("  none\n")
	}
	for _, row := range o.Waiters {
		tb.Str("  ").Str(row.Label).Str(" pid ").Int(int64(row.PID)).Str(", ").Str(row.Liveness).
			Str(", waited ").Int(row.WaitedSeconds).Str("s")
		writeOwnerText(&tb, row.Session, row.Agent)
		tb.Byte('\n')
		if row.Command != "" {
			tb.Str("    ").Str(row.Command).Byte('\n')
		}
	}

	tb.Str("machine:\n")
	o.Machine.writeText(&tb, o.DiskVolumes)
	tb.Str(o.Notice).Byte('\n')
	return tb.String()
}

// writeText renders the machine section: what the machine has, then what is
// using it. It states readings and no verdict: no line says go or wait.
func (m *Machine) writeText(tb *textbuf.Buffer, volumes string) {
	tb.Str("  cores: ")
	m.Cores.writeTo(tb)
	tb.Byte('\n')

	tb.Str("  load 1m: ")
	m.Load1.writeLoad(tb)
	load, loadRead := m.Load1.Value()
	cores, coresRead := m.Cores.Value()
	if loadRead && coresRead && cores > 0 {
		tb.Str(" (").Float2(load / cores).Str(" per core)")
	}
	tb.Str("\n  load 5m: ")
	m.Load5.writeLoad(tb)
	tb.Str("\n  load 15m: ")
	m.Load15.writeLoad(tb)
	tb.Byte('\n')

	_, availableRead := m.MemoryAvailable.Value()
	_, totalRead := m.MemoryTotal.Value()
	tb.Str("  memory available: ")
	m.MemoryAvailable.writeBytes(tb)
	if availableRead && totalRead {
		tb.Str(" of ")
		m.MemoryTotal.writeBytes(tb)
		tb.Str(" total\n")
	} else {
		tb.Str("\n  memory total: ")
		m.MemoryTotal.writeBytes(tb)
		tb.Byte('\n')
	}

	if volumes == volumesShared {
		tb.Str("  disk free, tmp and Go build cache (one volume): ")
		m.DiskFreeTmp.writeBytes(tb)
		tb.Byte('\n')
	} else {
		tb.Str("  disk free, tmp: ")
		m.DiskFreeTmp.writeBytes(tb)
		tb.Str("\n  disk free, Go build cache: ")
		m.DiskFreeBuildCache.writeBytes(tb)
		tb.Byte('\n')
		if volumes != volumesApart {
			tb.Str("  disk volumes: ").Str(volumes).Byte('\n')
		}
	}

	tb.Str("  le test processes: ")
	m.LeTestProcesses.writeTo(tb)
	tb.Str("\n  go test processes: ")
	m.GoTestProcesses.writeTo(tb)
	tb.Byte('\n')
}

// writeOwnerText renders who asked for a job. An empty session is one the
// file did not name; an empty agent is a main thread or the same.
func writeOwnerText(tb *textbuf.Buffer, session, agent string) {
	tb.Str(", session ")
	if session == "" {
		tb.Str("unknown")
	} else {
		tb.Str(session)
	}
	if agent != "" {
		tb.Str(", agent ").Str(agent)
	}
}
