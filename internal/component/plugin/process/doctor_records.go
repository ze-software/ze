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
// the oldest.
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
	configCommit             ConfigCommit
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
		if inc := p.deliverySlowInc; inc != nil {
			inc()
		}
	}
	if failed {
		if inc := p.deliveryFailInc; inc != nil {
			inc()
		}
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
	r.problems[r.problemNext] = problem
	r.problemNext = (r.problemNext + 1) % deliveryProblemMax
	if r.problemCount < deliveryProblemMax {
		r.problemCount++
	}
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
