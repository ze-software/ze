// Design: docs/architecture/core-design.md -- admission for heavy jobs in a shared checkout
// Detail: registry.go -- the entry that IS the claim, and the scan that reads every one of them
// Detail: attach.go -- what a second asker does instead of running the same job twice
// Detail: treehash.go -- the fingerprint of the tree a job judges
// Detail: jobkey.go -- the fingerprint of the work a job does
//
// Package lejob determines whether each heavy job runs now.
//
// Several Claude sessions and their subagents use one checkout on one machine.
// Each heavy target uses resources allocated for the WHOLE box. A Go test run
// uses a quarter of the cores. golangci-lint uses one worker per core. If two
// sessions start a heavy job at the same time, they oversubscribe the machine
// until it stops responding. This problem was reported on 2026-08-17 with
// three concurrent sessions and "running the linting by hand can cause the
// machine to freeze".
//
// This package is a port of internal/le/lejob/answer.go. It preserves the registry
// format field for field and file name for file name. Thus, a job admitted by
// the shell and a job admitted here can detect each other. During migration,
// the two implementations use one queue and do not admit themselves separately.
//
// Five properties define the design. If one property is removed, the
// 2026-08-17 failure can occur again.
//
//   - A REGISTRY, not a lock. tmp/.ze-jobs/<label>.<pid>.job contains one
//     FIELD=VALUE line per field, and its PRESENCE is the claim. No separate
//     record identifies the running job. The holder removes its entry when it
//     exits. A waiter removes an entry if its process is gone. Thus, a crashed
//     job causes a delay of one poll interval and does not require an operator.
//
//   - FAIL CLOSED. An entry that cannot be read counts as OCCUPIED. If
//     "cannot parse" meant "nothing is running", all sessions would start at
//     the same time. This package prevents that failure. An unreadable entry
//     is removed only after it is older than the stall window. This rule keeps
//     the registry bounded.
//
//   - ATTACH on identical work. Serialization alone would make eight sessions
//     wait for eight runs of the same work. If a second asker's label, tree,
//     and work key match a running job, it follows that job's log and uses its
//     exit code. One run supplies both answers.
//
//   - Break a stalled holder on EVIDENCE. A slot is broken if the holder's
//     process is gone. It is also broken if the holder is alive and its log
//     has not grown during the stall window. Elapsed time alone is never the
//     reason. An operator who reads "20 minutes elapsed" cannot determine if
//     the kill was correct. One who reads "no output for 31 minutes" can.
//
//   - NESTING. A wrapped job can run wrapped stages. For example, a verify
//     holds a slot and then runs a lint that is also wrapped. If the inner job
//     waited for the slot held by its parent, neither job would release it.
//     Therefore, a job whose parent entry is still present runs INSIDE the
//     parent's slot.
//
// The library is primary, and the command is secondary. `le job run` is one
// caller. Another caller is a launcher that must build a binary before it can
// execute the binary.
//
// The launcher gets a slot and builds the binary. It releases the slot before
// it executes the binary. Two sessions can both find bin/ze missing and start a
// 630-package build. This oversubscription is what the package prevents. A
// fresh checkout or the first command after a clean has this risk.
package lejob

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/lepath"
)

// The registry's paths, all relative to the checkout root. They are relative
// because the shell half writes them relative and a follower has to open what
// a holder wrote, whichever half wrote it.
const (
	// JobsDir holds one entry, one log and one result per job.
	JobsDir = "tmp/.ze-jobs"
	// lockName serializes the scan-and-claim. It is the ONE lock here, and it
	// is held for the length of a directory scan rather than for the length of
	// a job.
	lockName = ".registry.lock"
	// OwnerFile is the documented view of the current holder: ai/rules/git-safety.md
	// and ai/rules/commands.md both tell readers this file names it. It is a
	// copy of the entry, written and removed with it.
	OwnerFile = "tmp/.ze-verify.lock.owner"
	// DurationFile records how long each job took, one tab-separated line per
	// finished job. A waiting session reads it to say what the previous run of
	// this label cost.
	DurationFile = "tmp/.ze-verify-duration.txt"
)

// These are the bounds and default for the stall window. The window measures
// SILENCE, not run time. A job that continues to write to its log is never
// broken, regardless of its run time. The default exceeds the longest silent
// interval measured in this repository. On 2026-08-17, golangci-lint was silent
// for about 18 minutes during a 20 minute verify. Thus, a real run has spare
// time, and a wedged run is reclaimed within one hour.
const (
	StallMin     = 60 * time.Second
	StallMax     = 3600 * time.Second
	StallDefault = 1800 * time.Second
)

// SlotsMin is the least useful number of slots. Zero would queue every job for
// ever, so it is refused rather than clamped.
const SlotsMin = 1

// SlotsDefault is what a caller supplies when it does not know this machine's
// shared-job policy.
const SlotsDefault = 1

// The intervals a waiting job runs on.
const (
	// PollDefault is how often a waiting job re-checks the registry.
	PollDefault = 2 * time.Second
	// BannerDefault is how often a waiting job repeats its banner. A twenty
	// minute wait at the poll interval would otherwise put 600 identical lines
	// into an agent's context.
	BannerDefault = 30 * time.Second
	// ResultWaitDefault is how long an attacher waits for the result of the
	// job it followed, after that job's output has ended.
	ResultWaitDefault = 10 * time.Second
	// lockWait limits the wait for the registry lock. If the wait expires, the
	// process cannot inspect the registry. Therefore, it does not admit a job.
	lockWait = 10 * time.Second
	// lockRetry is how often the registry lock is re-tried inside that wait.
	lockRetry = 50 * time.Millisecond
)

// The keys the run-time knobs are read under. Two carry the spelling ai/rules
// documents, so a session following it reaches the same value.
const (
	// SlotsKey is how many admitted jobs run at once.
	SlotsKey = "ze.run.slots"
	// StallKey is the silence budget, in seconds.
	StallKey = "ze.job.stall.seconds"
	// ParentKey names the entry of the job this one runs inside. A wrapper
	// exports it, and a job that finds it runs in its parent's slot.
	ParentKey = "ze.run.job"
	// AttachKey identifies the caller's opt-out from sharing. It has no ze prefix
	// because ai/rules and plan/journal already instruct a caller to export
	// MAY_ATTACH. A second spelling would cause the documented spelling to have
	// no effect.
	AttachKey = "may.attach"
)

// typeString is the env registry's word for a free-text value.
const typeString = "string"

// typeInt is the env registry's word for a number.
const typeInt = "int"

var slotsEntry = env.MustRegister(env.EnvEntry{
	Key:         SlotsKey,
	Type:        typeInt,
	Default:     "1",
	Description: "how many admitted jobs run at once on this machine",
	// Private keeps the key out of `ze env list`. It is a build-host knob and
	// an operator has nothing to do with it.
	Private: true,
})

var stallEntry = env.MustRegister(env.EnvEntry{
	Key:         StallKey,
	Type:        typeInt,
	Default:     "1800",
	Description: "how long a live job may write nothing before its slot is broken, in seconds",
	// This is the older spelling. ai/rules/git-safety.md instructs readers to
	// increase it, so that instruction must remain effective. It now permits a
	// longer SILENCE, which is not necessary for a healthy slow job.
	Aliases: []string{"ZE_VERIFY_MAX_LOCK_AGE"},
	Private: true,
})

var parentEntry = env.MustRegister(env.EnvEntry{
	Key:         ParentKey,
	Type:        typeString,
	Default:     "",
	Description: "the registry entry of the job this process runs inside",
	Private:     true,
})

var attachEntry = env.MustRegister(env.EnvEntry{
	Key:         AttachKey,
	Type:        typeString,
	Default:     "1",
	Description: "whether this job may take a running job's verdict instead of running its own",
	Private:     true,
})

// ErrLabel says the label would not be a path component under the registry
// directory. A separator or a dot in a label is how a job escapes it.
var ErrLabel = errors.New("a label is [A-Za-z0-9_-] and nothing else")

// ErrNoCommand says the caller named no command to run.
var ErrNoCommand = errors.New("a job needs a command to run")

// Admission manages the registry and its policy for one checkout.
//
// NOT safe for concurrent use. Each job entry is named for the process that
// holds it. Therefore, two goroutines that request admission at the same time
// would write one entry. This package coordinates PROCESSES, so the registry
// is on disk.
type Admission struct {
	// Root is the checkout the registry lives under.
	Root string
	// Slots specifies the maximum number of jobs that can run at the same time.
	Slots int
	// Stall specifies the SILENCE limit. If a live holder does not write during
	// this interval, the breaker breaks its slot.
	Stall time.Duration
	// Poll is how often a waiting job re-checks the registry.
	Poll time.Duration
	// Banner is how often a waiting job repeats what it is waiting for.
	Banner time.Duration
	// ResultWait is how long an attacher waits for the verdict of the job it
	// followed.
	ResultWait time.Duration
	// MayAttach permits this job to use the verdict of a running job. A caller
	// that requires an independent answer sets it to false. This setting causes
	// a duplicate run and does not use a shared verdict.
	MayAttach bool
	// Out is where the job's output goes, a followed job's replayed log
	// included.
	Out io.Writer
	// Err receives every banner. A command's ANSWER is a payload on stdout, and
	// the caller can pipe it. A progress line on stdout would become part of the
	// document.
	Err io.Writer
	// Color controls whether banners contain ANSI sequences. The shell half
	// always emitted them. This implementation emits them only for a terminal
	// because a follower replays a log that would otherwise contain them.
	Color bool
}

// New answers the admission for the checkout this process runs in, with the
// policy the environment states.
func New() (*Admission, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	return NewIn(root)
}

// NewIn returns the admission manager for one named checkout. Tests use it,
// as do tools that already know their root.
//
// Values outside the permitted range cause an error and are not clamped. A
// stall window that is too small kills a healthy job between two log lines. A
// stall window that is too large lets a wedged job hold the only slot for
// hours. The caller must not receive a policy different from the value it
// specified.
func NewIn(root string) (*Admission, error) {
	adm := &Admission{
		Root:       root,
		Slots:      env.GetInt(slotsEntry.Key, SlotsDefault),
		Stall:      stallWindow(),
		Poll:       PollDefault,
		Banner:     BannerDefault,
		ResultWait: ResultWaitDefault,
		MayAttach:  attachAllowed(),
		Out:        os.Stdout,
		Err:        os.Stderr,
		Color:      terminal(os.Stderr),
	}
	if err := adm.Validate(); err != nil {
		return nil, err
	}
	return adm, nil
}

// stallWindow returns the SILENCE limit specified by the environment. It
// accepts either of the two spellings that specify this limit.
//
// The alias is read EXPLICITLY because env.Get resolves an alias only when the
// caller passes the alias. If the caller requests the canonical key, env.Get
// reads only the canonical spelling. Therefore, a session that exports
// ZE_VERIFY_MAX_LOCK_AGE would otherwise receive the default. The file
// ai/rules/git-safety.md instructs readers to increase that name, so it must
// remain effective. internal/le/gotoolchain reads its two aliases in the same way.
//
// A value that is not a number produces zero instead of the default. Zero is
// outside the permitted range, so Validate rejects and identifies it. An
// unparseable control is a caller error. A silent default would apply a policy
// that the caller did not specify. This behavior is how a slot count of 1 can
// become 4 without detection.
func stallWindow() time.Duration {
	spelled := env.Get(stallEntry.Key)
	for _, alias := range stallEntry.Aliases {
		if spelled != "" {
			break
		}
		spelled = env.Get(alias)
	}
	if spelled == "" {
		return StallDefault
	}

	seconds, err := strconv.Atoi(spelled)
	if err != nil {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// attachAllowed reads the caller's opt-out from sharing. An unset value
// shares, and any value other than 1 declines: a typo costs a duplicate run,
// never a borrowed verdict.
func attachAllowed() bool {
	asked := env.Get(attachEntry.Key)
	return asked == "" || asked == "1"
}

// terminal reports whether a file is a terminal, which is the one thing that
// makes an ANSI sequence worth writing.
func terminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Validate reports what is wrong with this policy, in the words the caller has
// to act on.
func (a *Admission) Validate() error {
	if a.Stall < StallMin || a.Stall > StallMax {
		var tb textbuf.Buffer
		return errors.New(tb.Str("stall window ").Int(int64(a.Stall / time.Second)).
			Str("s is out of range (").Int(int64(StallMin / time.Second)).Str("..").
			Int(int64(StallMax / time.Second)).Str(" seconds); set ZE_JOB_STALL_SECONDS. It budgets SILENCE, ").
			Str("not run time: a job that keeps writing to its log is never broken, however long it runs").
			String())
	}

	slotsMax := runtime.NumCPU()
	if a.Slots < SlotsMin || a.Slots > slotsMax {
		var tb textbuf.Buffer
		return errors.New(tb.Str("slot count ").Int(int64(a.Slots)).Str(" is out of range (").
			Int(int64(SlotsMin)).Str("..").Int(int64(slotsMax)).
			Str("); set ZE_RUN_SLOTS. Native actions default to one slot; use ZE_RUN_SLOTS=1 to serialize").
			String())
	}
	return nil
}

// Kind says how a job was admitted. The zero value names no admission at all,
// so a ticket nobody filled in cannot read as a held slot.
type Kind uint8

const (
	// KindUnspecified is the zero value and is never a live admission.
	KindUnspecified Kind = iota
	// KindClaimed is a slot this job holds and MUST release.
	KindClaimed
	// KindAttached is another job's verdict, already observed. Nothing is held
	// and nothing has to be released.
	KindAttached
	// KindInside is a job running in its parent's slot. Nothing is held.
	KindInside
	// KindUnadmitted is a job that skipped admission because it records no
	// verdict and does no work: make's no-execute modes.
	KindUnadmitted
)

// String answers the word a report carries for this admission.
func (k Kind) String() string {
	switch k {
	case KindClaimed:
		return "claimed"
	case KindAttached:
		return "attached"
	case KindInside:
		return "inside"
	case KindUnadmitted:
		return "unadmitted"
	default:
		return "unspecified"
	}
}

// ticket contains the answer to "does this job run now".
//
// A ticket with KindClaimed holds a slot. Its holder MUST call Release exactly
// once. All other kinds hold no slot, and Release is a no-op for them. Thus, a
// caller can defer the call unconditionally.
//
// A holder MUST write its progress to Log while it works. The breaker uses
// that file as liveness evidence, and a follower reads it for replay. If a
// holder remains silent, the breaker breaks its slot while it works. Run
// writes the progress for a job that is another program. A caller that does
// the work in-process has the same responsibility.
type ticket struct {
	// Label is the job's name, and the first component of its entry.
	Label string
	// Kind says whether this ticket holds a slot.
	Kind Kind
	// Code is the verdict of the job this one attached to. It is meaningful
	// for KindAttached and zero everywhere else.
	Code int
	// Tree is the tree hash the job is judging, measured at admission.
	Tree string
	// Key fingerprints the work: the command plus the make command-line
	// variables the caller typed.
	Key string
	// Entry is the registry entry this job holds, relative to the root.
	Entry string
	// Log is the file the holder MUST write its progress to, relative to the
	// root.
	Log string
	// Waited is how long admission took.
	Waited time.Duration

	adm     *Admission
	started time.Time
}

// Release drops the slot: it records the job's verdict where a follower can
// find it, appends the run's duration, and removes the entry.
//
// The result is written BEFORE the entry goes, never after. An attacher
// watching the entry disappear must find the code already written rather than
// a race it can lose.
//
// MUST be called exactly once for a ticket whose Kind is KindClaimed, and MUST
// be paired with the admit that answered it. It is a no-op for every other
// kind, and for a second call on one ticket.
func (t *ticket) Release(code int) {
	if t == nil || t.Kind != KindClaimed || t.adm == nil {
		return
	}
	t.Kind = KindUnspecified

	adm := t.adm
	adm.appendDuration(t.Label, time.Since(t.started))
	adm.writeResult(t.Entry, code)
	adm.remove(adm.abs(t.Entry))
	adm.remove(adm.abs(t.Log))
	adm.remove(adm.abs(OwnerFile))
}

// admit waits until this job can run and then returns its admission result.
//
// argv identifies the work that the job will run. admit reads it to calculate
// the work key but does not execute it. A caller that does its work in-process
// passes the argv that identifies that work.
//
// admit can return a slot to hold, another job's verdict, or a parent's slot
// to use. Only the first result must be released.
func (a *Admission) admit(label string, argv []string) (*ticket, error) {
	if !validLabel(label) {
		return nil, ErrLabel
	}
	if len(argv) == 0 {
		return nil, ErrNoCommand
	}
	if err := a.Validate(); err != nil {
		return nil, err
	}

	if parent, nested := a.insideParent(); nested {
		var tb textbuf.Buffer
		a.note(tb.Byte('[').Str(label).Str("] running inside ").Str(parent).String())
		return &ticket{Label: label, Kind: KindInside, adm: a}, nil
	}

	if err := os.MkdirAll(a.jobsDir(), 0o755); err != nil { //nolint:gosec // the registry is read and written by every session on this machine
		return nil, err
	}

	job := pending{
		label:     label,
		argv:      argv,
		tree:      TreeHash(a.Root),
		key:       jobKey(argv),
		mayAttach: a.MayAttach,
	}
	return a.queue(&job)
}

// queue is the admission loop. It queries the registry and then runs, shares,
// or waits before it queries the registry again.
//
// The loop intentionally has no iteration limit. A job waits while the
// machine is busy, for as long as one hour. The registry controls this wait.
// Each holder in the registry is alive and writing, or the registry reaps it.
// See registry.go.
func (a *Admission) queue(job *pending) (*ticket, error) {
	started := time.Now()
	var lastBanner time.Time

	for {
		result := a.claim(job)

		switch result.state {
		case stateClaimed:
			return &ticket{
				Label: job.label, Kind: KindClaimed, Tree: job.tree, Key: job.key,
				Entry: result.entry, Log: result.log, Waited: time.Since(started),
				adm: a, started: result.started,
			}, nil

		case stateAttach:
			if code, observed := a.attach(job.label, result); observed {
				return &ticket{
					Label: job.label, Kind: KindAttached, Code: code, Tree: job.tree,
					Key: job.key, Waited: time.Since(started), adm: a,
				}, nil
			}
			// Sharing is offered once. Repeated attachment to jobs that die with
			// no result would follow one corpse after another. The job would
			// never run. Thus, an attach that observed nothing sends this job
			// back to the ordinary queue for good.
			job.mayAttach = false
			job.treeStale = true

		case stateBusy:
			job.treeStale = true
			if time.Since(lastBanner) >= a.Banner {
				a.reportBusy(job.label, result)
				lastBanner = time.Now()
			}
			time.Sleep(a.Poll)
		}
	}
}

// insideParent answers the label of the job this process already runs inside,
// and reports whether there is one.
//
// The function checks the parent's ENTRY, not the variable alone. A job whose
// parent has finished is not nested. If that job ran without admission, no
// claim would protect the heavy work.
func (a *Admission) insideParent() (string, bool) {
	named := env.Get(parentEntry.Key)
	if named == "" {
		return "", false
	}
	body, err := os.ReadFile(named) //nolint:gosec // the path is this process's own parent entry, exported by the job that started it
	if err != nil {
		return "", false
	}
	if label := field(body, "LABEL"); label != "" {
		return label, true
	}
	return named, true
}

// Run is the whole of the wrapper: admission, the child, its log, and the
// release.
//
// The child runs in dir, or in the root when dir is empty. The child gets
// environ, or os.Environ when environ is nil. The function adds this job's
// entry. Thus, a wrapped stage uses this slot and does not queue behind it.
//
// The child's stdout and stderr are merged and teed to the job's log, which is
// what a follower replays and what the breaker judges. The cost, stated
// because it is visible: the child's stdout is a pipe, so a tool that colors
// only for a terminal stops coloring.
func (a *Admission) Run(label string, argv []string, dir string, environ []string) (Report, int) {
	report := Report{Label: label, Command: argv, Admission: KindUnspecified.String()}

	if dryRun() {
		// Admission is SKIPPED rather than refused. This records no verdict.
		// Thus, make prints its recipes and exits. A wrapped recipe gives
		// `$(MAKE) ...` to the shell. GNU make executes a recipe line that
		// contains $(MAKE) even under -n, -t, and -q. This behavior lets
		// recursive make participate in those modes.
		//
		// Without this branch, the job queues for a slot and writes no log. No
		// scan sees progress, and `make -n` hangs until the stall window expires.
		report.Admission = KindUnadmitted.String()
		report.Code = a.stream(argv, a.runDir(dir), environ, nil)
		return report, report.Code
	}

	ticket, err := a.admit(label, argv)
	if err != nil {
		a.note(errorLine(err))
		report.Code = 2
		return report, report.Code
	}

	report.Admission = ticket.Kind.String()
	report.Tree = ticket.Tree
	report.Key = ticket.Key
	report.WaitedSeconds = int(ticket.Waited / time.Second)

	switch ticket.Kind {
	case KindAttached:
		report.Code = ticket.Code
		return report, report.Code
	case KindInside:
		report.Code = a.stream(argv, a.runDir(dir), environ, nil)
		return report, report.Code
	case KindUnspecified, KindUnadmitted, KindClaimed:
		// A claimed slot is the rest of this function.
	}

	logFile, err := os.OpenFile(a.abs(ticket.Log), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // this job's own registry log, named from a validated label
	if err != nil {
		ticket.Release(gaterun.CannotStart)
		a.note(errorLine(err))
		report.Code = gaterun.CannotStart
		return report, report.Code
	}

	a.reportPrevious(label)
	code := a.stream(argv, a.runDir(dir), a.childEnviron(environ, ticket), logFile)
	if err := logFile.Close(); err != nil {
		a.note(errorLine(err))
	}
	ticket.Release(code)

	report.Code = code
	return report, code
}

// runDir answers where the child runs: what the caller named, or the checkout
// root, which is where every wrapped recipe runs today.
func (a *Admission) runDir(dir string) string {
	if dir != "" {
		return dir
	}
	return a.Root
}

// childEnviron gives the child this job's entry. Thus, a wrapped stage in the
// child uses this slot and does not wait behind its parent.
//
// The entry path is absolute because a stage can run outside the checkout root.
// Registry paths are relative to the root. If a nested job cannot find its
// parent's entry, it enters the queue. A job behind its own parent cannot start.
func (a *Admission) childEnviron(environ []string, ticket *ticket) []string {
	if environ == nil {
		environ = os.Environ()
	}

	var tb textbuf.Buffer
	named := tb.Str(parentVariable).Byte('=').Str(a.abs(ticket.Entry)).String()

	tb.Reset()
	inherited := tb.Str(parentVariable).Byte('=').String()

	out := make([]string, 0, len(environ)+1)
	for _, pair := range environ {
		if strings.HasPrefix(pair, inherited) {
			continue
		}
		out = append(out, pair)
	}
	return append(out, named)
}

// parentVariable is the environment spelling of ParentKey. The child is
// another program rather than a Ze process, so it reads the variable rather
// than the registry key.
const parentVariable = "ZE_RUN_JOB"

// stream runs the child with its output going to this job's stdout and, when
// there is one, to its log as well.
//
// This function differs from gaterun.Stream because it owns a tee. It also
// forwards a signal to the child, so the job releases its slot. The exit-code
// rule comes from gaterun.
func (a *Admission) stream(argv []string, dir string, environ []string, logFile io.Writer) int {
	if len(argv) == 0 {
		a.note("error: a job declared no command to run")
		return gaterun.CannotStart
	}
	if environ == nil {
		environ = os.Environ()
	}

	out := a.Out
	if logFile != nil {
		out = io.MultiWriter(a.Out, logFile)
	}

	// context.Background has no deadline. A job that needs a wall-clock cap
	// carries the cap in its OWN argv. There, `timeout` can signal the complete
	// process group. A deadline here would kill the child but leave its
	// grandchildren holding the log pipe open. internal/le/functional runs every
	// suite under `timeout` to prevent that failure.
	//nolint:gosec // argv is what the caller asked to run; le is a build-host tool driven by a developer's argv
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = environ
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Start(); err != nil {
		var tb textbuf.Buffer
		a.note(tb.Str("  cannot run ").Str(argv[0]).Str(": ").Err(err).String())
		return gaterun.CannotStart
	}

	stop := forwardSignals(cmd)
	err := cmd.Wait()
	stop()

	if err != nil {
		return gaterun.ExitCode(err)
	}
	return 0
}

// forwardSignals sends an interrupt to the child. Thus, this job finishes its
// wait, releases its slot, and answers a code. It does not leave an entry for
// the next scan to reap.
//
// The returned function MUST be called after the child has been waited for. The
// call stops delivery and ends the goroutine. That is the complete life of this
// goroutine.
func forwardSignals(cmd *exec.Cmd) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		select {
		case got := <-signals:
			if cmd.Process != nil {
				// A signal the child cannot be sent is one it has already
				// answered: the wait below is what reports the outcome either
				// way.
				_ = cmd.Process.Signal(got) //nolint:errcheck // the wait reports what happened to the child
			}
		case <-done:
		}
	}()

	return func() {
		signal.Stop(signals)
		close(done)
	}
}

// dryRun reports whether make is in a no-execute mode, where a job records no
// verdict and takes no slot.
//
// The first word of MAKEFLAGS carries the single-letter flags, and a word that
// starts with a dash or holds an `=` is not that word.
func dryRun() bool {
	flags, _, _ := strings.Cut(os.Getenv("MAKEFLAGS"), " ")
	if flags == "" || strings.HasPrefix(flags, "-") || strings.Contains(flags, "=") {
		return false
	}
	return strings.ContainsAny(flags, "ntq")
}

// validLabel reports whether a label is a path component under the registry
// directory. A separator or a dot in a label is how a job escapes it.
func validLabel(label string) bool {
	if label == "" {
		return false
	}
	for i := range len(label) {
		c := label[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

// abs answers the absolute path of one root-relative registry path.
func (a *Admission) abs(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(a.Root, rel)
}

// note writes one line for a person watching the run.
func (a *Admission) note(line string) {
	if a.Err == nil {
		return
	}
	if _, err := io.WriteString(a.Err, line); err != nil {
		return
	}
	if _, err := io.WriteString(a.Err, "\n"); err != nil {
		return
	}
}

// errorLine spells a failure the way every ported le tool spells one.
func errorLine(err error) string {
	var tb textbuf.Buffer
	return tb.Str("error: ").Err(err).String()
}
