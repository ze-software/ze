// Design: docs/architecture/doctor-and-health-checks.md -- plugin registration and delivery checks
// Related: delivery.go -- deliverBatch notes each batch, the projection error is recorded
// Related: ../server/doctor_snapshot.go -- PluginDoctorState.Records reads DoctorRecord
//
// What the doctor reads of one plugin process's lifetime: the commands and
// aliases refused at registration, the subscriptions dropped, whether the
// post-startup callback reached it, the first failed event application, the
// recent delivery problems and the last config-committed outcome. Each record
// lives on the Process it describes, so a new Process (a respawn, a restart)
// starts clean, and a reader gets value copies through DoctorRecord.

package process

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorRecordMax bounds each per-process list the doctor reads. A plugin
// declares its commands and subscriptions once per lifetime, so the bound is
// reached only by a plugin declaring far more than any in the tree; the
// entries past it are counted, never kept.
const doctorRecordMax = 16

// DeliverySlowThreshold is the batch delivery time over which a batch is
// slow: a batch of exactly this long is not.
const DeliverySlowThreshold = 500 * time.Millisecond

// DeliveryWindow is how far back the delivery problems the doctor reports
// reach. A problem older than the window no longer counts.
const DeliveryWindow = 5 * time.Minute

// deliveryProblemMax bounds the delivery problems kept: the newest overwrite
// the oldest, and the summary says so (DeliveryWindowSummary.Truncated) while
// an overwritten one would still be inside the window.
const deliveryProblemMax = 64

// DeliverySlow reports whether a batch that took elapsed to deliver is slow.
func DeliverySlow(elapsed time.Duration) bool {
	return elapsed > DeliverySlowThreshold
}

// CommandRefusal is one command or deprecated alias the plugin declared that
// the command registry refused.
type CommandRefusal struct {
	// Command is the name the plugin declared.
	Command string
	// Alias is true for a deprecated alias, false for a command.
	Alias bool
	// Owner names who holds the name: another plugin's process name, or
	// "builtin". Empty when the refusal is not a name conflict (an invalid
	// name, an alias whose canonical command is not registered).
	Owner string
	// Reason is the registry's refusal text.
	Reason string
}

// SubscriptionDropCause says why a plugin's event subscriptions were dropped.
type SubscriptionDropCause uint8

// The subscription drop causes.
const (
	SubscriptionDropUnspecified SubscriptionDropCause = iota
	// SubscriptionDropUnknownNamespace: a subscribe block named a namespace no
	// component registered, and the whole block was skipped.
	SubscriptionDropUnknownNamespace
	// SubscriptionDropNoDefaultNamespace: a block named no namespace and none
	// is registered as the default, so its subscriptions match nothing.
	SubscriptionDropNoDefaultNamespace
	// SubscriptionDropReadyParamsUnparsable: the ready request's parameters,
	// which carry the startup subscriptions, did not parse.
	SubscriptionDropReadyParamsUnparsable
)

// String names the cause for a doctor row.
func (c SubscriptionDropCause) String() string {
	switch c {
	case SubscriptionDropUnspecified:
		return "unspecified"
	case SubscriptionDropUnknownNamespace:
		return "unknown event namespace"
	case SubscriptionDropNoDefaultNamespace:
		return "no default event namespace registered"
	case SubscriptionDropReadyParamsUnparsable:
		return "ready parameters did not parse"
	}
	return "unknown cause"
}

// SubscriptionDrop is one dropped subscription block.
type SubscriptionDrop struct {
	Cause SubscriptionDropCause
	// Namespace is the namespace the block named, empty when it named none.
	Namespace string
	// Detail is the parse error or the valid namespaces, for the operator.
	Detail string
}

// PostStartupOutcome is where the post-startup callback to a plugin stands.
type PostStartupOutcome uint8

// The post-startup outcomes. Unspecified is "not sent yet": a plugin still
// starting, or one no fan-out reached.
const (
	PostStartupUnspecified PostStartupOutcome = iota
	// PostStartupPending: the callback was sent and its answer is awaited,
	// within the sender's bound.
	PostStartupPending
	PostStartupDelivered
	PostStartupUndelivered
)

// DeliveryProblem is one batch that failed with no caller waiting for it, or
// that took longer than DeliverySlowThreshold.
type DeliveryProblem struct {
	Time    time.Time
	Failed  bool
	Elapsed time.Duration
	Events  int
	Error   string
}

// DeliveryWindowSummary is the delivery problems inside DeliveryWindow.
type DeliveryWindowSummary struct {
	Failures  int
	Slow      int
	Slowest   time.Duration
	LastError string
	Window    time.Duration
	// Truncated is true when the ring overwrote a problem that is still
	// inside the window: Failures and Slow count the kept problems only, so
	// the plugin had at least that many.
	Truncated bool
}

// ConfigCommit is the last config-committed callback sent to the plugin: the
// transaction and, when the plugin was not told, why.
type ConfigCommit struct {
	Transaction string
	Error       string
	Time        time.Time
}

// DoctorRecord is the value copy of a process's doctor records.
type DoctorRecord struct {
	CommandRefusals          []CommandRefusal
	CommandRefusalsDropped   int
	SubscriptionDrops        []SubscriptionDrop
	SubscriptionDropsDropped int
	PostStartup              PostStartupOutcome
	PostStartupError         string
	// ProjectionError is the first failed event application of this
	// process's lifetime (a delivery panic or a failed batch), empty when
	// none failed.
	ProjectionError string
	Delivery        DeliveryWindowSummary
	ConfigCommit    ConfigCommit
}

// doctorRecords holds a process's records. Safe for concurrent use.
type doctorRecords struct {
	mu                       sync.Mutex
	commandRefusals          []CommandRefusal
	commandRefusalsDropped   int
	subscriptionDrops        []SubscriptionDrop
	subscriptionDropsDropped int
	postStartup              PostStartupOutcome
	postStartupErr           string
	projectionErr            string
	problems                 [deliveryProblemMax]DeliveryProblem
	problemNext              int
	problemCount             int
	// overwrittenNewest is the time of the newest problem the ring
	// overwrote, zero while it overwrote none.
	overwrittenNewest time.Time
	configCommit      ConfigCommit
}

// RecordCommandRefusal records a command or alias the registry refused.
func (p *Process) RecordCommandRefusal(refusal CommandRefusal) {
	r := &p.doctor
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.commandRefusals) >= doctorRecordMax {
		r.commandRefusalsDropped++
		return
	}
	r.commandRefusals = append(r.commandRefusals, refusal)
}

// RecordSubscriptionDrop records a dropped subscription block.
func (p *Process) RecordSubscriptionDrop(drop SubscriptionDrop) {
	r := &p.doctor
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.subscriptionDrops) >= doctorRecordMax {
		r.subscriptionDropsDropped++
		return
	}
	r.subscriptionDrops = append(r.subscriptionDrops, drop)
}

// SetPostStartup records where the post-startup callback stands; err is the
// cause of PostStartupUndelivered and is ignored otherwise.
func (p *Process) SetPostStartup(outcome PostStartupOutcome, err error) {
	r := &p.doctor
	r.mu.Lock()
	defer r.mu.Unlock()
	r.postStartup = outcome
	r.postStartupErr = ""
	if outcome == PostStartupUndelivered && err != nil {
		r.postStartupErr = err.Error()
	}
}

// RecordConfigCommit records the outcome of the config-committed callback for
// transaction tx: a nil err is a plugin that was told. The latest call
// replaces the record, so a later confirmed commit clears a failure.
func (p *Process) RecordConfigCommit(tx string, err error) {
	commit := ConfigCommit{Transaction: tx, Time: time.Now()}
	if err != nil {
		commit.Error = err.Error()
	}
	r := &p.doctor
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configCommit = commit
}

// recordProjectionErr keeps the first failed event application for the
// doctor. The delivery goroutine's own projectionErr stays the source of the
// barrier answers; this is its copy for a reader on another goroutine.
func (p *Process) recordProjectionErr(err error) {
	r := &p.doctor
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.projectionErr == "" {
		r.projectionErr = err.Error()
	}
}

// noteBatch records one delivered batch when it was slow, or when it failed
// and no caller waits for its result (unwaited): a waiting caller receives
// the error and reports it itself. It logs the problem with its doctor code
// and counts it in the plugin's metrics.
func (p *Process) noteBatch(elapsed time.Duration, events int, err error, unwaited bool) {
	slow := DeliverySlow(elapsed)
	failed := err != nil && unwaited
	if slow {
		logger().Warn("event delivery batch slow",
			"plugin", p.config.Name, "events", events, "elapsed", elapsed,
			"threshold", DeliverySlowThreshold, "error", err,
			"code", diagnostic.CodeDoctorPluginDeliveryFailed)
		p.noteDeliverySlow()
	}
	if failed {
		p.noteDeliveryFailed()
	}
	if !slow && !failed {
		return
	}
	problem := DeliveryProblem{Time: time.Now(), Failed: failed, Elapsed: elapsed, Events: events}
	if err != nil {
		problem.Error = err.Error()
	}
	r := &p.doctor
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.problemCount == deliveryProblemMax {
		victim := r.problems[r.problemNext].Time
		if victim.After(r.overwrittenNewest) {
			r.overwrittenNewest = victim
		}
	}
	r.problems[r.problemNext] = problem
	r.problemNext = (r.problemNext + 1) % deliveryProblemMax
	if r.problemCount < deliveryProblemMax {
		r.problemCount++
	}
}

// restartOverlapGoroutinesFix is the fix of an overlap whose previous process
// runs in the daemon: no command ends a goroutine, and the full dump
// (show system goroutines, ze-cli-show-cmd.yang) prints every stack, the
// unfinished engine's or delivery loop's included.
const restartOverlapGoroutinesFix = "ze show system goroutines full"

// restartOverlapRelayLogFix is the fix of an overlap whose previous process
// was forked: no ze command ends a process, and the stderr relay logs under
// the plugin.relay component (stderrLogger, ze-log-cmd.yang `show log recent`)
// what the previous process still writes while ze reads its stderr.
const restartOverlapRelayLogFix = "ze show log recent component plugin.relay"

// recordRestartOverlap logs and raises a respawn whose join of the process it
// replaces expired after bound, so the replacement started before ze finished
// stopping that process: Respawn's only record of it. The event code is
// registered in register.go.
//
// A second restart is no fix, because it replaces the NEW process, and ze
// takes no further step to end the previous one, so the fix is the command
// that shows what is left (restartOverlapText).
func recordRestartOverlap(proc *Process, bound time.Duration, cause error) {
	name := proc.Name()
	messageLine, fixLine := restartOverlapText(proc, bound)
	logger().Warn("plugin restarted before ze finished stopping its previous process",
		"plugin", name, "detail", messageLine, "bound", bound, "error", cause,
		"fix", fixLine, "code", diagnostic.CodeDoctorPluginRestartOverlap)
	report.RaiseErrorFix(reportSourcePlugin, diagnostic.CodeDoctorPluginRestartOverlap, name,
		messageLine, fixLine)
}

// restartOverlapText says what of proc had not finished when a join of bound
// expired, and the command that shows it. Process.Wait joins every goroutine
// of the process (p.wg): for a plugin that runs in the daemon its engine and
// its event delivery loop; for a forked plugin its reaper (monitorCmd, which
// also waits for the stderr relay to read to its end), the stderr relay, the
// exit supervisor and the event delivery loop.
//
// One fact tells the parts apart: Running, which an engine clears when it
// returns and a forked plugin's reaper clears once the process has exited and
// been reaped. While it holds, the engine or the process itself had not
// ended, and the two plugin generations may hold the same resources. Once it
// is clear, what is left is the delivery loop (in the daemon), or the delivery
// loop or the stderr relay (forked), which nothing here tells apart; the relay
// reads until every holder of the plugin's stderr closes it, so a process the
// plugin started keeps it open. Neither part holds a plugin resource, so the
// text claims no overlap of resources for them.
//
// No pid is named: a forked plugin's process group id is its reaped leader's
// pid, which the kernel may already have given to another program.
//
// Each textbuf.Buffer is read once: String empties the buffer, so a second
// read answers "".
func restartOverlapText(proc *Process, bound time.Duration) (message, fix string) {
	var text textbuf.Buffer
	text.Str("plugin ").Str(proc.Name())
	if proc.config.Internal {
		if proc.Running() {
			text.Str(" was restarted before its previous engine ended: that engine runs in the daemon and had not returned ").
				Str(bound.String()).
				Str(" after ze stopped it, so the replacement runs beside it and the two may hold the same resources; ze takes no further step to end it")
		} else {
			text.Str(" was restarted before ze finished stopping its previous process: its engine had returned, but its event delivery had not finished ").
				Str(bound.String()).Str(" after ze stopped it; ze takes no further step to end it")
		}
		return text.String(), restartOverlapGoroutinesFix
	}
	if proc.Running() {
		text.Str(" was restarted before its previous process exited: that process had not exited ").
			Str(bound.String()).
			Str(" after ze stopped it, so the replacement runs beside it and the two may hold the same resources; ze takes no further step to end it")
	} else {
		text.Str(" was restarted before ze finished stopping its previous process: that process had exited, but ze had not finished its event delivery or reading its stderr ").
			Str(bound.String()).
			Str(" after ze stopped it, and that stderr stays open while a process the plugin started still holds it; ze takes no further step to end them")
	}
	return text.String(), restartOverlapRelayLogFix
}

// raiseRelayStopped raises the stopped stderr relay on the report bus, where
// the recent-events check reports it: the plugin keeps running with its later
// output, a panic block included, never relayed, and a restart starts a new
// relay. The event code is registered in register.go.
func raiseRelayStopped(name string, cause error) {
	var message textbuf.Buffer
	message.Str("the stderr relay of plugin ").Str(name).
		Str(" stopped, so its later output is not logged: ").Err(cause)
	report.RaiseErrorFix(reportSourcePlugin, diagnostic.CodeDoctorPluginStderrRelayStopped, name,
		message.String(), RestartCommand(name))
}

// DoctorRecord answers a copy of the process's records, with the delivery
// problems summarized over the DeliveryWindow that ends at now. Safe for
// concurrent use.
func (p *Process) DoctorRecord(now time.Time) DoctorRecord {
	r := &p.doctor
	r.mu.Lock()
	defer r.mu.Unlock()
	record := DoctorRecord{
		CommandRefusals:          append([]CommandRefusal(nil), r.commandRefusals...),
		CommandRefusalsDropped:   r.commandRefusalsDropped,
		SubscriptionDrops:        append([]SubscriptionDrop(nil), r.subscriptionDrops...),
		SubscriptionDropsDropped: r.subscriptionDropsDropped,
		PostStartup:              r.postStartup,
		PostStartupError:         r.postStartupErr,
		ProjectionError:          r.projectionErr,
		ConfigCommit:             r.configCommit,
		Delivery:                 DeliveryWindowSummary{Window: DeliveryWindow},
	}
	since := now.Add(-DeliveryWindow)
	if !r.overwrittenNewest.IsZero() {
		record.Delivery.Truncated = !r.overwrittenNewest.Before(since)
	}
	var newest time.Time
	for i := range r.problemCount {
		problem := &r.problems[i]
		if problem.Time.Before(since) {
			continue
		}
		if problem.Failed {
			record.Delivery.Failures++
		}
		if DeliverySlow(problem.Elapsed) {
			record.Delivery.Slow++
			record.Delivery.Slowest = max(record.Delivery.Slowest, problem.Elapsed)
		}
		if problem.Error != "" && !problem.Time.Before(newest) {
			newest = problem.Time
			record.Delivery.LastError = problem.Error
		}
	}
	return record
}
