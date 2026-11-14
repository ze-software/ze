// Related: overview.go -- the report these tests read
// Related: waiter_test.go -- holdAndQueue, the two real jobs the wiring case drives
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-15 (the registry half:
// slots, holders, waiters), AC-16 (an unavailable reading is shown as
// unavailable) and AC-17 (the report writes nothing).
// PREVENTS: a report that counts a dead job's entry as a held slot, one that
// prints a failed reading as 0, and one that changes the registry it reads.

package job

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/hostload"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/procscan"
)

// tableOf answers a process scan that lists exactly these pids, so a case
// needs no real process to have a live or a dead one.
func tableOf(pids ...int) func() ([]procscan.Process, error) {
	return func() ([]procscan.Process, error) {
		processes := make([]procscan.Process, 0, len(pids))
		for _, pid := range pids {
			processes = append(processes, procscan.Process{PID: pid})
		}
		return processes, nil
	}
}

// unreadMachine is the machine section of a case that is not about the
// machine: every reading unavailable, and no reader run.
func unreadMachine() Machine {
	unread := Unavailable("not read by this test")
	return Machine{
		Cores: unread, Load1: unread, Load5: unread, Load15: unread,
		MemoryAvailable: unread, MemoryTotal: unread,
		DiskFreeTmp: unread, DiskFreeBuildCache: unread,
		LeTestProcesses: unread, GoTestProcesses: unread,
	}
}

// fixedSources answers machine sources that read nothing: 8 cores, loads of
// 2, 1.5 and 1.25, 24 of 64 GiB of memory, 100.5 GiB free on one volume that
// holds both directories, three `le test` processes and no Go test (a true
// zero). A case replaces the source it is about.
func fixedSources() machineSources {
	return machineSources{
		cores: func() int { return 8 },
		load: func() (hostload.Averages, error) {
			return hostload.Averages{One: 2, Five: 1.5, Fifteen: 1.25}, nil
		},
		memoryAvailable: func() (uint64, error) { return 24 * bytesPerGiB, nil },
		memoryTotal:     func() (uint64, error) { return 64 * bytesPerGiB, nil },
		leTests:         func() (int, error) { return 3, nil },
		goTests:         func() (int, error) { return 0, nil },
		diskFree:        func(string) (uint64, error) { return 100*bytesPerGiB + bytesPerGiB/2, nil },
		sameVolume:      func(string, string) (bool, error) { return true, nil },
		tmp:             "/checkout/tmp",
		buildCache:      "/checkout/cache/go-cache",
	}
}

// fabricatedWaiter writes one waiter record for a pid, as writeWaiter spells
// it.
func fabricatedWaiter(t *testing.T, root, label string, pid int, waited time.Duration) {
	t.Helper()
	var tb textbuf.Buffer
	tb.Str("LABEL=").Str(label).Byte('\n')
	tb.Str("PID=").Int(int64(pid)).Byte('\n')
	tb.Str("WAITING=").Int(time.Now().Add(-waited).Unix()).Byte('\n')
	tb.Str("CMD=go test ./x/\n")
	tb.Str("SESSION=waiter-session\nAGENT=waiter-agent\n")

	path := filepath.Join(root, JobsDir, entryName(label, pid, ".claim"+waiterSuffix))
	if err := os.WriteFile(path, []byte(tb.String()), 0o644); err != nil { //nolint:gosec // a registry file this test fabricates
		t.Fatalf("write a waiter record: %v", err)
	}
}

// fabricatedRegistry fills a registry with one of everything the report has
// to tell apart, and answers the admission over it:
//
//	live.5001      an entry whose process runs, 90 seconds old, with an owner
//	dead.5002      an entry whose process is gone
//	broken.<pid>   an entry whose fields cannot be read
//	queued.6001    a waiter whose process runs
//	gone.6002      a waiter whose process is gone
func fabricatedRegistry(t *testing.T) *Admission {
	t.Helper()
	root := fixtureRepo(t)
	adm := admission(t, root)
	adm.Slots = 3

	live, _ := heldEntrySince(t, root, "live", 5001, 90*time.Second)
	appendFields(t, live, "SESSION=holder-session\nAGENT=holder-agent\n")
	heldEntry(t, root, "dead", 5002)
	writeEntry(t, root, "broken", "not a registry entry\n")
	fabricatedWaiter(t, root, "queued", 6001, 40*time.Second)
	fabricatedWaiter(t, root, "gone", 6002, 40*time.Second)
	return adm
}

// TestWaitingJobIsListedByTheReport is the wiring case: two real processes,
// one holding the only slot and one queued behind it, read through Overview
// with the real process scan.
//
// VALIDATES: the wiring row "a job that waits for a slot", and AC-15's rows.
func TestWaitingJobIsListedByTheReport(t *testing.T) {
	root := fixtureRepo(t)
	pair := holdAndQueue(t, root)
	adm := admission(t, root)

	view, err := adm.Overview()
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if view.Slots != 1 || view.Occupied != 1 || view.Free != 0 {
		t.Errorf("the report counts %d slots, %d occupied, %d free; want 1, 1, 0", view.Slots, view.Occupied, view.Free)
	}
	if len(view.Holders) != 1 || view.Holders[0].Label != "holder" || view.Holders[0].Liveness != wordRunning {
		t.Fatalf("the report lists holders %+v, want the one running holder", view.Holders)
	}
	if view.Holders[0].PID != pair.holder.Process.Pid {
		t.Errorf("the holder row names pid %d, want %d", view.Holders[0].PID, pair.holder.Process.Pid)
	}
	if len(view.Waiters) != 1 {
		t.Fatalf("the report lists waiters %+v, want the one queued job", view.Waiters)
	}
	queued := view.Waiters[0]
	if queued.Label != "queued" || queued.PID != pair.waiter.Process.Pid || queued.Liveness != wordRunning {
		t.Errorf("the waiter row is %+v, want the queued job's label, pid and a running process", queued)
	}
	if queued.Session != queuedSession || queued.Agent != queuedAgent {
		t.Errorf("the waiter row names owner %q / %q, want %s / %s", queued.Session, queued.Agent, queuedSession, queuedAgent)
	}
	if !strings.Contains(view.Notice, "only jobs admitted through this checkout's registry") {
		t.Errorf("the report's notice is %q, want the line on what the slots do not count", view.Notice)
	}

	pair.release()
	if err := pair.holder.Wait(); err != nil {
		t.Fatalf("the holder: %v", err)
	}
	if err := pair.waiter.Wait(); err != nil {
		t.Fatalf("the queued job: %v", err)
	}
	after, err := adm.Overview()
	if err != nil {
		t.Fatalf("Overview after both ended: %v", err)
	}
	if after.Occupied != 0 || after.Free != 1 || len(after.Holders) != 0 || len(after.Waiters) != 0 {
		t.Errorf("after both jobs ended the report is %+v, want an empty registry with its slot free", after)
	}
}

// TestReportShowsACentralHolder asks what the report shows for a heavy action
// the dispatcher admitted. Its method: a real unshared hold, with the report
// read from inside its body, where the slot is held.
//
// VALIDATES: spec-native-action-job-admission AC-2: a holder row with the
// label derived from the command name, the session and agent that typed the
// line, its age and the typed line, and a notice that no longer lists Docker
// labs among the work that takes no slot.
// PREVENTS: a centrally admitted action that the report shows without its
// owner or its command because its entry names no log.
func TestReportShowsACentralHolder(t *testing.T) {
	detach(t)
	identify(t, "central-session-1", "central-agent-1")
	root := fixtureRepo(t)
	adm := admission(t, root)
	line := []string{"./le", "test", "integration", "bgp"}

	var view *Overview
	code, err := adm.Hold(t.Context(), centralLabelOf("test integration"), line, func() int {
		seen, err := adm.overview(overviewPass{scan: tableOf(os.Getpid()), machine: unreadMachine, now: time.Now()})
		if err != nil {
			t.Errorf("overview from inside the hold: %v", err)
			return 1
		}
		view = seen
		return 0
	})
	if err != nil || code != 0 {
		t.Fatalf("the hold answered %d, %v", code, err)
	}
	if view.Occupied != 1 || view.Free != 0 || len(view.Holders) != 1 {
		t.Fatalf("the report counts %d occupied, %d free, holders %+v; want the one central holder", view.Occupied, view.Free, view.Holders)
	}
	holder := view.Holders[0]
	if holder.Label != "test-integration" || holder.PID != os.Getpid() || holder.Liveness != wordRunning {
		t.Errorf("the holder row is %+v, want test-integration, this process, running", holder)
	}
	if holder.Session != "central-session-1" || holder.Agent != "central-agent-1" {
		t.Errorf("the holder row names owner %q / %q, want the session and agent that typed the line", holder.Session, holder.Agent)
	}
	if holder.Command != "./le test integration bgp" {
		t.Errorf("the holder row's command is %q, want the typed line", holder.Command)
	}
	if holder.AgeSeconds == nil {
		t.Error("the holder row carries no age, and the entry recorded its start")
	}

	text := view.Text()
	for _, want := range []string{"  test-integration pid ", "    ./le test integration bgp\n", "central-session-1"} {
		if !strings.Contains(text, want) {
			t.Errorf("the report reads\n%s\nwant it to hold %q", text, want)
		}
	}
	if strings.Contains(view.Notice, "Docker") {
		t.Errorf("the notice is %q, and a Docker lab now holds a slot", view.Notice)
	}
	for _, want := range []string{"ZE_ADMIT_RAW", "le exec", "declared slot-free", "post-edit lint", "outside ./le", "other checkouts"} {
		if !strings.Contains(view.Notice, want) {
			t.Errorf("the notice is %q, want it to name %q as outside the count", view.Notice, want)
		}
	}

	after, err := adm.overview(overviewPass{scan: tableOf(os.Getpid()), machine: unreadMachine, now: time.Now()})
	if err != nil {
		t.Fatalf("overview after the hold: %v", err)
	}
	if after.Occupied != 0 || len(after.Holders) != 0 {
		t.Errorf("after the hold the report is %+v, want the slot free", after)
	}
}

// TestReportListsLiveHoldersAndOmitsDeadEntries reads a fabricated registry
// against an injected process table.
//
// A live entry is a row with its owner and age. A dead entry and a dead
// waiter are in no row and no count. An unreadable entry is a row and a held
// slot, as admission counts it.
func TestReportListsLiveHoldersAndOmitsDeadEntries(t *testing.T) {
	adm := fabricatedRegistry(t)

	view, err := adm.overview(overviewPass{scan: tableOf(5001, 6001), machine: unreadMachine, now: time.Now()})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if view.Slots != 3 || view.Occupied != 2 || view.Free != 1 {
		t.Errorf("the report counts %d slots, %d occupied, %d free; want 3, 2, 1", view.Slots, view.Occupied, view.Free)
	}
	if view.ProcessTable != "read" {
		t.Errorf("the process table reads %q, want read", view.ProcessTable)
	}
	if len(view.Holders) != 2 {
		t.Fatalf("the report lists holders %+v, want the unreadable entry and the live one", view.Holders)
	}

	// Rows come in name order: broken.* before live.*.
	broken, live := view.Holders[0], view.Holders[1]
	if broken.Liveness != wordUnreadable || broken.PID != 0 || !strings.HasPrefix(broken.Label, "broken.") {
		t.Errorf("the unreadable entry's row is %+v, want it named, with no pid, marked unreadable", broken)
	}
	if live.Label != "live" || live.PID != 5001 || live.Liveness != wordRunning {
		t.Errorf("the live entry's row is %+v, want label live, pid 5001, running", live)
	}
	if live.Session != "holder-session" || live.Agent != "holder-agent" || live.Command != "sleep" {
		t.Errorf("the live entry's row is %+v, want its session, agent and command", live)
	}
	if live.AgeSeconds == nil || *live.AgeSeconds < 90 || *live.AgeSeconds > 150 {
		t.Errorf("the live entry's age is %v, want the 90 seconds its STARTED says", live.AgeSeconds)
	}

	if len(view.Waiters) != 1 {
		t.Fatalf("the report lists waiters %+v, want the one whose process runs", view.Waiters)
	}
	queued := view.Waiters[0]
	if queued.Label != "queued" || queued.PID != 6001 || queued.Session != "waiter-session" || queued.Agent != "waiter-agent" {
		t.Errorf("the waiter row is %+v, want queued, 6001 and its owner", queued)
	}
	if queued.WaitedSeconds < 40 || queued.WaitedSeconds > 100 || queued.Command != "go test ./x/" {
		t.Errorf("the waiter row is %+v, want 40 seconds waited and its command", queued)
	}
}

// TestReportCountsUnjudgedProcessesAsOccupied: when the process table cannot
// be read, no entry is known to be dead. Every one is shown with liveness
// unknown and counted as occupied, which is what admission does with a
// process it cannot judge, and the report says the table was not read.
func TestReportCountsUnjudgedProcessesAsOccupied(t *testing.T) {
	adm := fabricatedRegistry(t)
	failing := func() ([]procscan.Process, error) { return nil, errors.New("ps did not start") }

	view, err := adm.overview(overviewPass{scan: failing, machine: unreadMachine, now: time.Now()})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if view.Occupied != 3 || view.Free != 0 {
		t.Errorf("the report counts %d occupied, %d free; want 3 and 0 when nothing can be judged dead", view.Occupied, view.Free)
	}
	if view.ProcessTable != "unavailable: ps did not start" {
		t.Errorf("the process table reads %q, want the reason it is unavailable", view.ProcessTable)
	}
	unknown := 0
	for _, row := range view.Holders {
		if row.Liveness == wordUnknown {
			unknown++
		}
	}
	if unknown != 2 || len(view.Waiters) != 2 {
		t.Errorf("the report shows %d unknown holders and %d waiters, want 2 and 2: %+v", unknown, len(view.Waiters), view)
	}
	if !strings.Contains(view.Text(), "process table: unavailable: ps did not start") {
		t.Errorf("the text does not say the process table was unreadable:\n%s", view.Text())
	}
}

// TestReportShowsAnUnavailableReadingAsUnavailable gives the report one
// reading that failed beside readings that succeeded, one of them a true
// zero, and reads both renderings.
//
// VALIDATES: AC-16. The failed reading says unavailable with its reason in
// JSON and in text and is never a number, every other reading is still
// shown, and a zero appears only for the reading that measured zero.
//
// Method: the machine is read through readMachine from injected sources, one
// of which fails (the load averages) and one of which fails on one of its two
// calls (the build cache's volume), so the test proves the reader keeps going
// past a failure and not only that the renderer prints one.
func TestReportShowsAnUnavailableReadingAsUnavailable(t *testing.T) {
	adm := admission(t, fixtureRepo(t))
	sources := fixedSources()
	sources.load = func() (hostload.Averages, error) {
		return hostload.Averages{}, errors.New("the load average could not be read")
	}
	sources.diskFree = func(dir string) (uint64, error) {
		if dir == sources.buildCache {
			return 0, errors.New("the build cache volume did not answer")
		}
		return 200 * bytesPerGiB, nil
	}
	sources.sameVolume = func(string, string) (bool, error) { return false, nil }
	machine := func() Machine { return readMachine(&sources) }

	view, err := adm.overview(overviewPass{scan: tableOf(), machine: machine, now: time.Now()})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}

	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("encode the report: %v", err)
	}
	var decoded struct {
		Machine map[string]map[string]any `json:"machine"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode the report: %v\n%s", err, encoded)
	}
	load := decoded.Machine["load-1m"]
	if load["unavailable"] != "the load average could not be read" {
		t.Errorf("load-1m is %v, want it unavailable with its reason", load)
	}
	if _, numbered := load["value"]; numbered {
		t.Errorf("load-1m carries a value beside its failure: %v", load)
	}
	if cores := decoded.Machine["cores"]; cores["value"] != float64(8) {
		t.Errorf("cores is %v, want the measured 8 beside the failed reading", cores)
	}
	if zero := decoded.Machine["go-test-processes"]; zero["value"] != float64(0) {
		t.Errorf("go-test-processes is %v, want the measured zero", zero)
	}
	// One failed source fails each reading it feeds and no other.
	for _, name := range []string{"load-5m", "load-15m"} {
		if decoded.Machine[name]["unavailable"] != "the load average could not be read" {
			t.Errorf("%s is %v, want it unavailable with the load reader's reason", name, decoded.Machine[name])
		}
	}
	if cache := decoded.Machine["disk-free-build-cache-bytes"]; cache["unavailable"] != "the build cache volume did not answer" {
		t.Errorf("disk-free-build-cache-bytes is %v, want it unavailable with its reason", cache)
	}
	for name, want := range map[string]float64{
		"memory-available-bytes": 24 * bytesPerGiB,
		"memory-total-bytes":     64 * bytesPerGiB,
		"disk-free-tmp-bytes":    200 * bytesPerGiB,
		"le-test-processes":      3,
	} {
		if got := decoded.Machine[name]; got["value"] != want {
			t.Errorf("%s is %v, want the measured %v beside the failed readings", name, got, want)
		}
	}
	if len(decoded.Machine) != 10 {
		t.Errorf("the machine section holds %d readings, want 10: %v", len(decoded.Machine), decoded.Machine)
	}

	text := view.Text()
	for _, want := range []string{
		"load 1m: unavailable (the load average could not be read)",
		"load 15m: unavailable (the load average could not be read)",
		"cores: 8\n",
		"memory available: 24.0 GiB of 64.0 GiB total\n",
		"disk free, tmp: 200.0 GiB\n",
		"disk free, Go build cache: unavailable (the build cache volume did not answer)\n",
		"le test processes: 3\n",
		"go test processes: 0\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the text lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "load 1m: 0") {
		t.Errorf("the text prints the failed load reading as a number:\n%s", text)
	}

	// A reading nobody filled is unavailable too, in both renderings.
	var unfilled Reading
	blank, err := json.Marshal(unfilled)
	if err != nil {
		t.Fatalf("encode an unfilled reading: %v", err)
	}
	if string(blank) != `{"unavailable":"never read"}` {
		t.Errorf("an unfilled reading encodes as %s, want it unavailable", blank)
	}
	if _, measured := unfilled.Value(); measured {
		t.Error("an unfilled reading claims a measurement")
	}
	if _, measured := Measured(0).Value(); !measured {
		t.Error("a measured zero does not claim its measurement")
	}
}

// TestReportReadingRendersInEveryFormat marshals a measured reading, a
// measured zero and an unavailable reading, and reads them through each
// renderer `le job report` answers through: `| json`, `| yaml` and `| table`
// (leroot encodes the payload as JSON and hands that text to the shared
// renderers of internal/component/command, so no renderer reflects over
// Reading's unexported fields).
//
// VALIDATES: AC-15 ("renders through json, yaml and table") and AC-16 in each
// format: a value is a number under "value", a failure is its reason under
// "unavailable", and a failure is never a number.
func TestReportReadingRendersInEveryFormat(t *testing.T) {
	const reason = "vm_stat did not start"
	document := struct {
		Load   Reading `json:"load-1m"`
		Tests  Reading `json:"go-test-processes"`
		Memory Reading `json:"memory-available-bytes"`
	}{Load: Measured(2.5), Tests: Measured(0), Memory: Unavailable(reason)}

	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := `{"load-1m":{"value":2.5},"go-test-processes":{"value":0},"memory-available-bytes":{"unavailable":"` + reason + `"}}`
	if string(encoded) != want {
		t.Errorf("json is\n%s\nwant\n%s", encoded, want)
	}

	var parsed any
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	renderings := map[string]string{
		"yaml":  command.RenderYAML(parsed),
		"table": command.ApplyTable(string(encoded)),
	}
	for format, rendered := range renderings {
		for _, part := range []string{"load-1m", "2.5", "go-test-processes", "memory-available-bytes", "unavailable", reason} {
			if !strings.Contains(rendered, part) {
				t.Errorf("the %s rendering lacks %q:\n%s", format, part, rendered)
			}
		}
		if strings.Contains(rendered, "measured") || strings.Contains(rendered, "{}") {
			t.Errorf("the %s rendering shows the reading's fields or an empty object:\n%s", format, rendered)
		}
	}
	// In YAML the three readings are three maps of one key each.
	for _, line := range []string{"value: 2.5", "value: 0", "unavailable: "} {
		if !strings.Contains(renderings["yaml"], line) {
			t.Errorf("the yaml rendering lacks the line %q:\n%s", line, renderings["yaml"])
		}
	}
}

// TestMachineTextStatesReadingsAndNoVerdict reads the machine section a
// person sees: the load per core, memory as available of total, and one disk
// line when tmp and the build cache share a volume.
//
// VALIDATES: AC-15's machine half, and the decision that the report shows
// readings and decides nothing.
// PREVENTS: one volume printed twice as if it were two, and a line added to
// the machine section that states a verdict: the section is compared whole.
func TestMachineTextStatesReadingsAndNoVerdict(t *testing.T) {
	adm := admission(t, fixtureRepo(t))
	sources := fixedSources()
	machine := func() Machine { return readMachine(&sources) }

	view, err := adm.overview(overviewPass{scan: tableOf(), machine: machine, now: time.Now()})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if view.DiskVolumes != volumesShared {
		t.Errorf("disk-volumes is %q, want %q", view.DiskVolumes, volumesShared)
	}
	text := view.Text()
	wantMachine := "machine:\n" +
		"  cores: 8\n" +
		"  load 1m: 2.00 (0.25 per core)\n" +
		"  load 5m: 1.50\n" +
		"  load 15m: 1.25\n" +
		"  memory available: 24.0 GiB of 64.0 GiB total\n" +
		"  disk free, tmp and Go build cache (one volume): 100.5 GiB\n" +
		"  le test processes: 3\n" +
		"  go test processes: 0\n" +
		slotsNotice + "\n"
	if !strings.HasSuffix(text, wantMachine) {
		t.Errorf("the machine section reads\n%s\nwant it to end with\n%s", text, wantMachine)
	}

	// Whether the volumes are shared could not be read: both are shown, and
	// the report says what it does not know.
	sources.sameVolume = func(string, string) (bool, error) { return false, errors.New("stat did not answer") }
	view, err = adm.overview(overviewPass{scan: tableOf(), machine: machine, now: time.Now()})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	for _, want := range []string{
		"  disk free, tmp: 100.5 GiB\n",
		"  disk free, Go build cache: 100.5 GiB\n",
		"  disk volumes: unknown: stat did not answer\n",
	} {
		if !strings.Contains(view.Text(), want) {
			t.Errorf("the text lacks %q:\n%s", want, view.Text())
		}
	}
}

// TestVolumeOfADanglingLinkIsAnErrorNotItsParent: tmp/ is a link out of the
// checkout. When its target is gone, the free space of the checkout's own
// volume must not be shown under tmp's name.
//
// A directory that does not exist yet reads its deepest existing ancestor
// (a cache is created on first use), and a link that resolves reads its
// target.
func TestVolumeOfADanglingLinkIsAnErrorNotItsParent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "tmp")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "gone"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	if probe, err := volumeProbe(filepath.Join(root, "tmp")); err != nil || probe != resolvedTarget {
		t.Errorf("volumeProbe(a link) = %q, %v; want its target %q", probe, err, resolvedTarget)
	}
	if probe, err := volumeProbe(filepath.Join(root, "cache", "go-cache")); err != nil || probe != resolvedRoot {
		t.Errorf("volumeProbe(a directory not created yet) = %q, %v; want its existing ancestor %q", probe, err, resolvedRoot)
	}
	for _, dir := range []string{filepath.Join(root, "dangling"), filepath.Join(root, "dangling", "go-cache")} {
		if probe, err := volumeProbe(dir); err == nil {
			t.Errorf("volumeProbe(%s) = %q, want an error for a link whose target is gone", dir, probe)
		}
		if free, err := volumeFree(dir); err == nil {
			t.Errorf("volumeFree(%s) = %d, want an error and never the parent's free space", dir, free)
		}
	}

	if free, err := volumeFree(filepath.Join(root, "tmp")); err != nil || free == 0 {
		t.Errorf("volumeFree(tmp) = %d, %v; want the free bytes of a volume a test can write to", free, err)
	}
	if shared, err := sameVolume(root, target); err != nil || !shared {
		t.Errorf("sameVolume(a directory, its child) = %v, %v; want true", shared, err)
	}
}

// registrySnapshot is the registry directory as bytes: every file's name,
// content and modification time.
func registrySnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	listed, err := os.ReadDir(filepath.Join(root, JobsDir))
	if err != nil {
		t.Fatalf("list the registry: %v", err)
	}
	snapshot := make(map[string]string, len(listed))
	for _, listing := range listed {
		path := filepath.Join(root, JobsDir, listing.Name())
		body, err := os.ReadFile(path) //nolint:gosec // a registry file this test fabricated
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		snapshot[listing.Name()] = info.ModTime().String() + "\n" + string(body)
	}
	return snapshot
}

// TestReportWritesNothing snapshots a registry holding live, dead and
// unreadable entries, logs, a result receipt and waiter records, runs the
// report, and compares.
//
// VALIDATES: AC-17. Every file is byte-identical with its modification time,
// no file appeared (the registry lock among them) and none was removed, the
// dead job's evidence included. A checkout with no registry gets none.
func TestReportWritesNothing(t *testing.T) {
	adm := fabricatedRegistry(t)
	receipt := filepath.Join(adm.jobsDir(), entryName("finished", 5003, ".claim"+resultSuffix))
	if err := os.WriteFile(receipt, []byte("0\n"), 0o644); err != nil { //nolint:gosec // a registry file this test fabricates
		t.Fatal(err)
	}
	before := registrySnapshot(t, adm.Root)

	if _, err := adm.overview(overviewPass{scan: tableOf(5001, 6001), machine: unreadMachine, now: time.Now()}); err != nil {
		t.Fatalf("overview: %v", err)
	}

	after := registrySnapshot(t, adm.Root)
	if len(after) != len(before) {
		t.Errorf("the registry held %d files before the report and %d after", len(before), len(after))
	}
	for name, was := range before {
		if is, kept := after[name]; !kept || is != was {
			t.Errorf("%s changed or went while the report ran", name)
		}
	}

	bare := admission(t, t.TempDir())
	view, err := bare.overview(overviewPass{scan: tableOf(), machine: unreadMachine, now: time.Now()})
	if err != nil {
		t.Fatalf("overview over a checkout with no registry: %v", err)
	}
	if view.Occupied != 0 || view.Free != bare.Slots {
		t.Errorf("a checkout with no registry reports %d occupied and %d free, want 0 and %d", view.Occupied, view.Free, bare.Slots)
	}
	if _, err := os.Stat(bare.jobsDir()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the report created the registry directory it found absent: %v", err)
	}
}
