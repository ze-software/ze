// Related: central.go -- the central admitter and the unshared hold these tests drive
// Related: contention_test.go -- the fixtures: admission, sleeper, detach
// Related: waiter_test.go -- waitForWaiter, the state a waiting line is read from
//
// VALIDATES: spec-native-action-job-admission AC-1 (one entry, labeled by the
// command name, with the typed line), AC-4 (one slot per tree), AC-10 (a
// central line waits, is listed, and an interrupt ends the wait), AC-11 (the
// body's code, and a release on panic), AC-13 (no checkout runs unadmitted),
// AC-18 (identical central lines never share a verdict), and assumption A-6
// (every reader of the registry takes an entry that names no log).
// PREVENTS: a heavy action that holds two slots or none, a slot a crashed body
// leaves held by a live process, a second build that takes the first one's
// verdict, a line that hangs on a registry it cannot read, and a waiter told
// that a healthy holder's log has gone silent.
//
// No case waits for the clock. A holder is a fabricated entry for a process
// that is alive, and a waiting line is read from its waiter record.

package job

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// centralLine is the typed line the cases admit, and centralName the command
// name it resolves to.
var centralLine = []string{"./le", "test", "integration", "bgp"}

const (
	centralName  = "test integration"
	centralLabel = "test-integration"
)

// entriesOf answers the registry entries under root, in name order.
func entriesOf(t *testing.T, root string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(root, JobsDir, "*"+entrySuffix))
	if err != nil {
		t.Fatalf("list the registry entries: %v", err)
	}
	return names
}

// registryFilesWith answers the registry files under root with one suffix.
func registryFilesWith(t *testing.T, root, suffix string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(root, JobsDir, "*"+suffix))
	if err != nil {
		t.Fatalf("list the registry files ending %s: %v", suffix, err)
	}
	return names
}

// loglessEntry fabricates the entry an unshared claim writes, for a process
// the case names: no log beside it, an empty LOG field and an unmeasured tree.
func loglessEntry(t *testing.T, root, label string, pid int) string {
	t.Helper()
	dir := filepath.Join(root, JobsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("make the registry: %v", err)
	}
	var tb textbuf.Buffer
	writeField(&tb, "LABEL", label)
	tb.Str("PID=").Int(int64(pid)).Byte('\n')
	tb.Str("PGID=0\n")
	writeField(&tb, "TREE", Unknown)
	writeField(&tb, "KEY", jobKey(centralLine))
	tb.Str("STARTED=").Int(time.Now().Unix()).Byte('\n')
	writeField(&tb, "LOG", "")
	tb.Str("STATE=running\n")
	writeField(&tb, commandField, commandLine(centralLine))

	path := filepath.Join(dir, entryName(label, pid, ".claim"+entrySuffix))
	if err := os.WriteFile(path, []byte(tb.String()), 0o644); err != nil { //nolint:gosec // a registry file this test fabricates
		t.Fatalf("write a logless entry: %v", err)
	}
	return path
}

// centralOver answers the central admitter over one fixture tree, with a wait
// root the case cancels, and everything it writes going to errOut.
func centralOver(t *testing.T, root string, errOut *bytes.Buffer) (centralPoint, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return centralPoint{
		root: func() (string, error) { return root, nil },
		policy: func(root string) (*Admission, error) {
			adm := admission(t, root)
			adm.Err = errOut
			return adm, nil
		},
		waitRoot: func() (context.Context, context.CancelFunc) { return ctx, cancel },
		errOut:   errOut,
	}, cancel
}

// TestCentralLabelIsTheCommandName asks whether the label a central line holds
// its slot under is its command name, as a path component. Its method: the
// derivation alone, then one line through the admitter with the entry read
// while the body runs.
func TestCentralLabelIsTheCommandName(t *testing.T) {
	for name, want := range map[string]string{
		"test integration":     "test-integration",
		"chaos run":            "chaos-run",
		"repo check generated": "repo-check-generated",
		"go vet-platforms":     "go-vet-platforms",
		"odd.name/of two":      "odd-name-of-two",
	} {
		got := centralLabelOf(name)
		if got != want {
			t.Errorf("centralLabelOf(%q) = %q, want %q", name, got, want)
		}
		if !validLabel(got) {
			t.Errorf("centralLabelOf(%q) = %q, which is not a valid label", name, got)
		}
	}

	detach(t)
	// A named caller, so the line has no unknown owner to report on stderr.
	identify(t, "central-session-1", "central-agent-1")
	root := t.TempDir()
	var errOut bytes.Buffer
	point, _ := centralOver(t, root, &errOut)
	var labels, commands []string
	code := point.admit(centralName, centralLine, func() int {
		adm := admission(t, root)
		for _, path := range entriesOf(t, root) {
			held, ok := adm.readEntry(path)
			if !ok {
				t.Errorf("the entry %s cannot be read", path)
				continue
			}
			labels = append(labels, held.label)
			commands = append(commands, held.command)
		}
		return 6
	})
	if code != 6 {
		t.Errorf("the line answered %d, want the body's 6: %s", code, errOut.String())
	}
	if len(labels) != 1 || labels[0] != centralLabel {
		t.Errorf("the body ran holding entries labeled %v, want exactly one, %s", labels, centralLabel)
	}
	if len(commands) != 1 || commands[0] != strings.Join(centralLine, " ") {
		t.Errorf("the entry records the command %v, want the typed line %q", commands, strings.Join(centralLine, " "))
	}
	if left := entriesOf(t, root); len(left) != 0 {
		t.Errorf("the line left entries behind: %v", left)
	}
	if errOut.Len() != 0 {
		t.Errorf("an uncontended central line wrote %q, want nothing of its own", errOut.String())
	}
}

// TestUnsharedClaimWritesNoLogAndUnknownTree asks what an unshared hold writes
// to the registry and what it leaves. Its method: one hold in a tree that is
// not a git repository, its entry read from inside the body.
func TestUnsharedClaimWritesNoLogAndUnknownTree(t *testing.T) {
	detach(t)
	root := t.TempDir()
	adm := admission(t, root)

	var body string
	var entries, logs []string
	code, err := adm.Hold(t.Context(), centralLabel, centralLine, func() int {
		entries = entriesOf(t, root)
		logs = registryFilesWith(t, root, logSuffix)
		if len(entries) == 1 {
			body = read(t, entries[0])
		}
		return 7
	})
	if err != nil || code != 7 {
		t.Fatalf("the hold answered %d, %v; want the body's 7", code, err)
	}
	if len(entries) != 1 {
		t.Fatalf("the body ran holding %v, want exactly one entry", entries)
	}
	if len(logs) != 0 {
		t.Errorf("an unshared claim created the log %v, want none", logs)
	}
	for name, want := range map[string]string{
		"LABEL": centralLabel, "TREE": Unknown, "LOG": "", "STATE": "running",
		commandField: strings.Join(centralLine, " "),
	} {
		if got := field([]byte(body), name); got != want {
			t.Errorf("the entry's %s is %q, want %q", name, got, want)
		}
	}
	if !strings.Contains(body, "\nLOG=\n") {
		t.Errorf("the entry is\n%s\nwant an empty LOG line, which every reader takes as no log", body)
	}

	if left := entriesOf(t, root); len(left) != 0 {
		t.Errorf("the release left entries: %v", left)
	}
	if receipts := registryFilesWith(t, root, resultSuffix); len(receipts) != 0 {
		t.Errorf("an unshared claim wrote the receipt %v, and nothing follows it", receipts)
	}
	if rows := read(t, filepath.Join(root, DurationFile)); !strings.HasPrefix(rows, centralLabel+"\t") {
		t.Errorf("the duration record is %q, want one row for %s", rows, centralLabel)
	}
}

// TestUnsharedClaimSkipsTheFingerprint asks whether an unshared hold measures
// its inputs. Its method: a git on the path that records every call. The
// unshared hold must leave no record, and a shared admission in the same tree
// must leave one, which is what shows the stand-in is the git admission runs.
func TestUnsharedClaimSkipsTheFingerprint(t *testing.T) {
	detach(t)
	root := t.TempDir()
	bin := t.TempDir()
	called := filepath.Join(bin, "git-was-called")
	script := "#!/bin/sh\necho called >> " + called + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil { //nolint:gosec // an executable stand-in this test runs
		t.Fatalf("write the git stand-in: %v", err)
	}
	t.Setenv("PATH", bin)

	adm := admission(t, root)
	code, err := adm.Hold(t.Context(), centralLabel, centralLine, func() int { return 0 })
	if err != nil || code != 0 {
		t.Fatalf("the unshared hold answered %d, %v", code, err)
	}
	if read(t, called) != "" {
		t.Error("an unshared hold ran git: it fingerprints inputs nothing can share")
	}

	ticket, err := adm.Admit("shared", []string{"true"})
	if err == nil {
		ticket.Release(0)
	}
	if read(t, called) == "" {
		t.Fatalf("a shared admission ran no git (it answered %v), so this case does not see the fingerprint at all", err)
	}
}

// waitsFor runs ask, an admission expected to queue, until its waiter record
// is on disk, then cancels it and answers what it returned.
func waitsFor(t *testing.T, root, label string, ask func(context.Context) error) error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	answered := make(chan error, 1)
	go func() { answered <- ask(ctx) }()

	waitForWaiter(t, root, label)
	cancel()
	select {
	case err := <-answered:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the queued admission did not end when its context was canceled")
		return nil
	}
}

// TestNothingAttachesToAnUnsharedHolder asks whether two identical lines ever
// share one run when either is unshared. Its method: a real unshared claim
// with a sharing asker of the same label and line behind it, then a sharing
// claim with an unshared asker behind it. Each asker must queue.
func TestNothingAttachesToAnUnsharedHolder(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)

	unshared := admission(t, root)
	unshared.Unshared = true
	holder, err := unshared.Admit(centralLabel, centralLine)
	if err != nil || holder.Kind != KindClaimed {
		t.Fatalf("the unshared holder was admitted as %v, %v", holder, err)
	}
	err = waitsFor(t, root, centralLabel, func(ctx context.Context) error {
		ticket, err := admission(t, root).AdmitContext(ctx, centralLabel, centralLine)
		if err == nil {
			t.Errorf("a sharing asker behind an unshared holder was admitted as %v", ticket.Kind)
			ticket.Release(0)
		}
		return err
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the sharing asker answered %v, want its canceled wait", err)
	}
	holder.Release(0)

	shared, err := admission(t, root).Admit(centralLabel, centralLine)
	if err != nil || shared.Kind != KindClaimed {
		t.Fatalf("the sharing holder was admitted as %v, %v", shared, err)
	}
	defer shared.Release(0)
	ran := false
	err = waitsFor(t, root, centralLabel, func(ctx context.Context) error {
		_, err := admission(t, root).Hold(ctx, centralLabel, centralLine, func() int { ran = true; return 0 })
		return err
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the unshared asker answered %v, want its canceled wait", err)
	}
	if ran {
		t.Error("the unshared asker ran its body while an identical job held the only slot")
	}
}

// TestCentralHolderNamesItselfForItsBody asks whether a tree holds one slot.
// Its method: inside a hold's body, the parent variable is read, a
// self-admitting job and a second central hold are admitted, and the registry
// is counted. After the hold the variable holds what it held before.
func TestCentralHolderNamesItselfForItsBody(t *testing.T) {
	detach(t)
	root := t.TempDir()
	adm := admission(t, root)

	var named string
	var held []string
	innerKind := KindUnspecified
	nestedEntries := -1
	code, err := adm.Hold(t.Context(), centralLabel, centralLine, func() int {
		named = env.Get(ParentKey)
		held = entriesOf(t, root)

		inner, err := admission(t, root).Admit("inner", []string{"true"})
		if err != nil {
			t.Errorf("the nested job was refused: %v", err)
			return 1
		}
		innerKind = inner.Kind
		inner.Release(0)

		nested, err := admission(t, root).Hold(t.Context(), "nested", []string{"./le", "nested"}, func() int {
			nestedEntries = len(entriesOf(t, root))
			return 3
		})
		if err != nil {
			t.Errorf("the nested central line was refused: %v", err)
		}
		return nested
	})
	if err != nil || code != 3 {
		t.Fatalf("the hold answered %d, %v; want the nested body's 3", code, err)
	}
	if len(held) != 1 || named != held[0] {
		t.Errorf("the body ran with %s naming %q, want the one entry it holds, %v", ParentKey, named, held)
	}
	if innerKind != KindInside {
		t.Errorf("a self-admitting job in the body was admitted as %v, want inside the holder's slot", innerKind)
	}
	if nestedEntries != 1 {
		t.Errorf("a central line in the body ran with %d entries in the registry, want the tree's one", nestedEntries)
	}
	if got := env.Get(ParentKey); got != "" {
		t.Errorf("the hold left %s naming %q, want the empty value it found", ParentKey, got)
	}

	// A parent that is named and gone grants nothing: the line claims anew,
	// and the name it found is the name it leaves.
	gone := filepath.Join(root, JobsDir, "gone.1.claim"+entrySuffix)
	if err := env.Set(ParentKey, gone); err != nil {
		t.Fatalf("name a parent that is gone: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set(ParentKey, ""); err != nil {
			t.Errorf("clear %s: %v", ParentKey, err)
		}
	})
	claimed := 0
	if _, err := adm.Hold(t.Context(), centralLabel, centralLine, func() int {
		claimed = len(entriesOf(t, root))
		return 0
	}); err != nil {
		t.Fatalf("the hold under a parent that is gone: %v", err)
	}
	if claimed != 1 {
		t.Errorf("under a parent that is gone the body ran with %d entries, want a claim of its own", claimed)
	}
	if got := env.Get(ParentKey); got != gone {
		t.Errorf("the hold left %s naming %q, want the %q it found", ParentKey, got, gone)
	}
}

// parentSpelling answers the value the process environment holds under one
// exact spelling of the parent variable, and whether it holds that spelling.
func parentSpelling(spelling string) (string, bool) {
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		if name == spelling {
			return value, true
		}
	}
	return "", false
}

// TestNameParentWritesBothSpellingsAndRestoresThem asks whether a child
// started through a shell sees its parent, and whether the holder leaves the
// environment as it found it. Its method: the process environment read under
// each exact spelling while a parent is named and after the restore, from
// three starting states, then one real hold whose body reads both.
//
// VALIDATES: a child reached through a /bin/sh script (the le launcher, a
// tool shim) inherits the parent's entry under ZE_RUN_JOB, the one spelling
// dash hands on, and the dotted spelling env.Get prefers holds the same entry.
// PREVENTS: a central action that starts `le` through the launcher and waits
// for the slot its own parent holds; and a holder that leaves its entry, or a
// spelling the caller never set, in the environment of later work.
func TestNameParentWritesBothSpellingsAndRestoresThem(t *testing.T) {
	root := t.TempDir()
	holder := loglessEntry(t, root, "holder", os.Getpid())
	other := loglessEntry(t, root, "other", os.Getpid())

	for name, before := range map[string]map[string]string{
		"both unset before":                              {},
		"underscore set before to another live entry":    {ParentVariable: other},
		"dotted set before":                              {ParentKey: other},
		"both set before, to different entries":          {ParentKey: other, ParentVariable: holder + ".older"},
		"underscore set before to the empty value":       {ParentVariable: ""},
		"another spelling of the key set before as well": {"Ze_Run.Job": other},
	} {
		t.Run(name, func(t *testing.T) {
			detach(t)
			for spelling, value := range before {
				t.Setenv(spelling, value)
			}
			env.ResetCache()
			readBefore := env.Get(ParentKey)

			restore, err := NameParent(holder)
			if err != nil {
				t.Fatalf("NameParent: %v", err)
			}
			for _, spelling := range []string{ParentKey, ParentVariable} {
				if got, set := parentSpelling(spelling); !set || got != holder {
					t.Errorf("while the parent is named, %s is %q (set %v), want the holder's entry %q", spelling, got, set, holder)
				}
			}
			if got := parentSpellings(os.Environ()); len(got) != 2 {
				t.Errorf("while the parent is named the environment holds %v, want the two spellings and no third", got)
			}
			if got := env.Get(ParentKey); got != holder {
				t.Errorf("while the parent is named env.Get reads %q, want %q", got, holder)
			}

			if err := restore(); err != nil {
				t.Fatalf("restore: %v", err)
			}
			for _, spelling := range []string{ParentKey, ParentVariable, "Ze_Run.Job"} {
				want, wasSet := before[spelling]
				if got, set := parentSpelling(spelling); set != wasSet || got != want {
					t.Errorf("after the restore %s is %q (set %v), want %q (set %v)", spelling, got, set, want, wasSet)
				}
			}
			if got := parentSpellings(os.Environ()); len(got) != len(before) {
				t.Errorf("after the restore the environment holds %v, want the %d spellings it held before", got, len(before))
			}
			if got := env.Get(ParentKey); got != readBefore {
				t.Errorf("after the restore env.Get reads %q, want the %q it read before", got, readBefore)
			}
			env.ResetCache()
			if got := env.Get(ParentKey); got != readBefore {
				t.Errorf("after the restore a fresh reading of the environment is %q, want %q", got, readBefore)
			}
		})
	}

	t.Run("a hold names its own entry under both", func(t *testing.T) {
		detach(t)
		// A registry of its own: the entries fabricated above are for this
		// live process, and a hold behind them would wait.
		fresh := t.TempDir()
		adm := admission(t, fresh)
		var dotted, underscore string
		var own []string
		code, err := adm.Hold(t.Context(), centralLabel, centralLine, func() int {
			own = entriesOf(t, fresh)
			dotted, _ = parentSpelling(ParentKey)
			underscore, _ = parentSpelling(ParentVariable)
			return 0
		})
		if err != nil || code != 0 {
			t.Fatalf("the hold answered %d, %v", code, err)
		}
		if len(own) != 1 || dotted != own[0] || underscore != own[0] {
			t.Errorf("the body ran with %s=%q and %s=%q, want both naming the hold's one entry, %v", ParentKey, dotted, ParentVariable, underscore, own)
		}
		for _, spelling := range []string{ParentKey, ParentVariable} {
			if got, set := parentSpelling(spelling); set {
				t.Errorf("the hold left %s=%q in the environment, and it was unset before", spelling, got)
			}
		}
	})
}

// TestCentralReleaseOnPanic asks whether a body that panics leaves its slot
// held. Its method: a panicking body under a hold, then a second hold on the
// same one-slot registry, which must be admitted at once.
func TestCentralReleaseOnPanic(t *testing.T) {
	detach(t)
	root := t.TempDir()
	adm := admission(t, root)

	recovered := func() (caught any) {
		defer func() { caught = recover() }()
		code, err := adm.Hold(t.Context(), centralLabel, centralLine, func() int {
			panic("BUG: the probe body crashed")
		})
		t.Errorf("a panicking body returned %d, %v; want the panic to continue", code, err)
		return nil
	}()
	if recovered == nil {
		t.Fatal("the hold swallowed its body's panic")
	}
	if left := entriesOf(t, root); len(left) != 0 {
		t.Errorf("the panicking body left its slot held: %v", left)
	}
	if got := env.Get(ParentKey); got != "" {
		t.Errorf("the panicking body left %s naming %q", ParentKey, got)
	}
	rows := read(t, filepath.Join(root, DurationFile))
	if strings.Count(rows, centralLabel+"\t") != 1 {
		t.Errorf("the duration record is %q, want exactly one release", rows)
	}

	code, err := adm.Hold(t.Context(), centralLabel, centralLine, func() int { return 0 })
	if err != nil || code != 0 {
		t.Errorf("the next hold answered %d, %v; want the freed slot", code, err)
	}
}

// queuedCentral starts a central line behind a fabricated holder that is
// alive, and returns once the line has written its waiter record.
type queuedCentral struct {
	root     string
	holder   string
	answered chan int
	ran      *atomic.Bool
	errOut   *bytes.Buffer
	cancel   context.CancelFunc
	end      func()
}

func queueCentral(t *testing.T, bodyCode int) queuedCentral {
	t.Helper()
	detach(t)
	root := t.TempDir()
	stand := sleeper(t)
	holder := loglessEntry(t, root, "holder", stand.Process.Pid)

	queued := queuedCentral{
		root: root, holder: holder, answered: make(chan int, 1),
		ran: &atomic.Bool{}, errOut: &bytes.Buffer{},
		end: func() { stop(t, stand) },
	}
	point, cancel := centralOver(t, root, queued.errOut)
	queued.cancel = cancel
	go func() {
		queued.answered <- point.admit(centralName, centralLine, func() int {
			queued.ran.Store(true)
			return bodyCode
		})
	}()

	waiting := waitForWaiter(t, root, centralLabel)
	if waiting.pid != os.Getpid() || waiting.command != strings.Join(centralLine, " ") {
		t.Errorf("the waiter record is %+v, want this process and the typed line", waiting)
	}
	if queued.ran.Load() {
		t.Fatal("the central line ran its body while the only slot was held")
	}
	return queued
}

// answer waits for the queued line's exit code.
func (q queuedCentral) answer(t *testing.T) int {
	t.Helper()
	select {
	case code := <-q.answered:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("the queued central line never answered")
		return 0
	}
}

// TestCentralLineWaitsForTheOnlySlotThenRuns asks whether a central line
// queues behind a holder and starts when the slot frees. Its method: a holder
// with no log whose process is ended, which frees its slot without removing
// its entry, as a killed holder does.
func TestCentralLineWaitsForTheOnlySlotThenRuns(t *testing.T) {
	queued := queueCentral(t, 4)

	queued.end()
	if code := queued.answer(t); code != 4 {
		t.Errorf("the admitted line answered %d, want its body's 4", code)
	}
	if !queued.ran.Load() {
		t.Error("the line answered without running its body")
	}
	if left := waiterRecords(t, queued.root, centralLabel); len(left) != 0 {
		t.Errorf("the admitted line left its waiter record: %v", left)
	}
	if left := entriesOf(t, queued.root); len(left) != 1 || left[0] != queued.holder {
		t.Errorf("the registry holds %v, want only the dead holder's entry, which no waiter removes", left)
	}

	banner := queued.errOut.String()
	if !strings.Contains(banner, "["+centralLabel+"] waiting: holder running (pid ") {
		t.Errorf("the waiting line printed %q, want the banner naming its holder", banner)
	}
	for _, clause := range []string{"has not grown", "log progress", "quiet window"} {
		if strings.Contains(banner, clause) {
			t.Errorf("the banner %q reports %q about a holder that writes no log", banner, clause)
		}
	}
}

// TestCentralWaitEndsOnSignal asks what ends a wait nobody wants any more. Its
// method: the wait's cancellation root, which SIGINT and SIGTERM end in the
// binary, is canceled while the line is queued.
func TestCentralWaitEndsOnSignal(t *testing.T) {
	queued := queueCentral(t, 0)

	queued.cancel()
	if code := queued.answer(t); code != centralInterrupted {
		t.Errorf("the interrupted wait answered %d, want %d", code, centralInterrupted)
	}
	if queued.ran.Load() {
		t.Error("the body ran after its wait was interrupted")
	}
	if left := waiterRecords(t, queued.root, centralLabel); len(left) != 0 {
		t.Errorf("the interrupted wait left its waiter record: %v", left)
	}
	if left := entriesOf(t, queued.root); len(left) != 1 || left[0] != queued.holder {
		t.Errorf("the registry holds %v, want only the holder: an interrupted wait takes no slot", left)
	}
	if said := queued.errOut.String(); !strings.Contains(said, "while waiting for a job slot: nothing ran") {
		t.Errorf("the interrupted wait printed %q, want it to say nothing ran", said)
	}
}

// TestCentralRunsUnadmittedWithNoCheckout asks what a central line does where
// no checkout exists. Its method: a root that answers lepath.ErrNoCheckout,
// and a policy that fails the case if it is read.
func TestCentralRunsUnadmittedWithNoCheckout(t *testing.T) {
	var errOut bytes.Buffer
	point := centralPoint{
		root: func() (string, error) { return "", lepath.ErrNoCheckout },
		policy: func(string) (*Admission, error) {
			t.Error("the admission policy was read with no checkout to hold a registry")
			return nil, lepath.ErrNoCheckout
		},
		waitRoot: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
		errOut:   &errOut,
	}
	ran := false
	if code := point.admit(centralName, centralLine, func() int { ran = true; return 9 }); code != 9 {
		t.Errorf("with no checkout the line answered %d, want its body's 9", code)
	}
	if !ran {
		t.Error("with no checkout the body did not run")
	}
	if errOut.Len() != 0 {
		t.Errorf("with no checkout the line printed %q, want nothing", errOut.String())
	}
}

// TestCentralRefusesWhenItsSlotCannotBeAskedFor asks what a central line does
// when admission itself fails. Its method: a checkout that cannot be
// resolved, a policy out of range, a registry path that is a file, and a
// registry lock that cannot be opened. Each must refuse with an operator
// error and code 2, run nothing, and return at once.
func TestCentralRefusesWhenItsSlotCannotBeAskedFor(t *testing.T) {
	detach(t)
	for name, arrange := range map[string]func(t *testing.T, point *centralPoint, root string){
		"the checkout cannot be resolved": func(_ *testing.T, point *centralPoint, _ string) {
			point.root = func() (string, error) { return "", os.ErrPermission }
		},
		"the policy is out of range": func(t *testing.T, point *centralPoint, _ string) {
			point.policy = func(root string) (*Admission, error) {
				adm := admission(t, root)
				adm.Slots = 0
				return nil, adm.Validate()
			}
		},
		"the registry path is a file": func(t *testing.T, _ *centralPoint, root string) {
			if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o755); err != nil {
				t.Fatalf("make tmp: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, JobsDir), nil, 0o600); err != nil {
				t.Fatalf("occupy the registry path: %v", err)
			}
		},
		"the registry lock cannot be opened": func(t *testing.T, _ *centralPoint, root string) {
			if err := os.MkdirAll(filepath.Join(root, JobsDir, lockName), 0o755); err != nil {
				t.Fatalf("occupy the lock path: %v", err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			var errOut bytes.Buffer
			point, _ := centralOver(t, root, &errOut)
			arrange(t, &point, root)

			ran := false
			if code := point.admit(centralName, centralLine, func() int { ran = true; return 0 }); code != centralRefused {
				t.Errorf("the line answered %d, want the refusal's %d", code, centralRefused)
			}
			if ran {
				t.Error("the body ran unadmitted after admission failed")
			}
			said := errOut.String()
			for _, want := range []string{"error: " + strings.Join(centralLine, " ") + ": ", "\nfix: "} {
				if !strings.Contains(said, want) {
					t.Errorf("the refusal reads %q, want it to hold %q", said, want)
				}
			}
		})
	}
}

// TestBusyBannerOmitsLogForALoglessHolder asks what the waiting banner says
// about a holder whose entry names no log. Its method: one scan of a registry
// holding such an entry for a live process, and the banner of its answer.
func TestBusyBannerOmitsLogForALoglessHolder(t *testing.T) {
	root := t.TempDir()
	stand := sleeper(t)
	loglessEntry(t, root, "holder", stand.Process.Pid)
	var errOut bytes.Buffer
	adm := admission(t, root)
	adm.Err = &errOut

	result := adm.claim(t.Context(), &pending{label: "asker", argv: []string{"true"}, tree: Unknown, key: "asker-key", unshared: true})
	if result.err != nil || result.state != stateBusy {
		t.Fatalf("the scan answered state %v, %v; want busy behind the live holder", result.state, result.err)
	}
	if result.holder != "holder" || result.log != "" {
		t.Errorf("the scan names holder %q with log %q, want holder and no log", result.holder, result.log)
	}

	adm.reportBusy("asker", result)
	banner := errOut.String()
	for _, want := range []string{"[asker] waiting: holder running (pid ", "; entry " + JobsDir} {
		if !strings.Contains(banner, want) {
			t.Errorf("the banner is %q, want it to hold %q", banner, want)
		}
	}
	for _, clause := range []string{"has not grown", "log progress", "quiet window"} {
		if strings.Contains(banner, clause) {
			t.Errorf("the banner %q reports %q about a holder that writes no log", banner, clause)
		}
	}
}

// TestALoglessEntryIsReadByThePrunerAndTheHolderListing asks whether the two
// readers outside admission take an entry with no log beside it. Its method:
// a dead, aged logless entry, listed by Holders and then pruned.
func TestALoglessEntryIsReadByThePrunerAndTheHolderListing(t *testing.T) {
	root := t.TempDir()
	dead := loglessEntry(t, root, "gone", 5002)
	age(t, dead)

	holders, err := Holders(root)
	if err != nil {
		t.Fatalf("Holders: %v", err)
	}
	if len(holders) != 1 || holders[0].PID != 5002 || holders[0].Label != "gone" {
		t.Errorf("Holders answered %+v, want the one logless entry", holders)
	}

	report, err := Prune(t.Context(), root, PrunePolicy{Now: time.Now(), Age: PruneAgeMin, Scan: tableOf()})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(report.Removed) != 1 || report.Removed[0].Path != dead || report.Removed[0].Reason != PruneRemovedDeadEntry {
		t.Errorf("the prune removed %+v, want the dead logless entry and nothing else", report.Removed)
	}
	if len(report.Kept) != 0 {
		t.Errorf("the prune kept %+v, want nothing: an entry with no log is not unreadable", report.Kept)
	}
	if left := entriesOf(t, root); len(left) != 0 {
		t.Errorf("the prune left %v", left)
	}
}
