// Design: docs/architecture/diagnostics/crash-capture.md -- panic containment at network-input task boundaries
//
// Package taskguard contains a panic at the entry of one task that handles
// network input (a session read loop, a receive goroutine, a request handler),
// so a Ze defect a peer happens to reach ends the unit that input belongs to and
// not the process.
//
// A recovered panic is never silent. The guard writes one ERROR line naming the
// component, the task, the unit, the message type where the task tracks one,
// the panic value, the stack, what restarts the unit and the `show crashes`
// command; it counts the panic in ze_task_panics_total{component,task}; and it
// writes a crash record through crashlog. It then hands control to the caller's
// end function, which tears the unit down through its existing path. The guard
// starts no goroutine and restarts nothing: the component's own backoff does.
//
// The guard goes once at the top of a task, never inside a decode function or
// per message inside a read loop.
package taskguard

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/metrics"
)

// MetricName is the counter every guard increments, labeled by component and
// task. The peer, interface or client is in the log line rather than a label, so
// the series count stays bounded by the task names components declare.
const MetricName = "ze_task_panics_total"

const metricHelp = "Panics recovered at a network-input task boundary; each ended one unit (session, neighbor, request or connection) while the process kept running."

// showCrashesCommand is the command that lists and prints the crash records the
// guard writes.
const showCrashesCommand = "ze show crashes"

// Guard is the panic boundary of one component. A component builds one with New,
// declares each guarded task with Task, and defers Task.Contain at the top of
// each task.
//
// Safe for concurrent use.
type Guard struct {
	component string
	log       *slog.Logger

	mu sync.Mutex
	// counts holds the panics each declared task recovered. Its keys are the
	// task names declared through Task, so its size is bounded by the code, not
	// by input. It carries the count across the time before a metrics registry
	// exists, which SetMetricsRegistry then adds to the counter.
	counts map[string]uint64
	// panics is nil until SetMetricsRegistry gives the guard a registry.
	panics metrics.CounterVec
}

// New returns the guard of component, logging through log, which MUST be
// non-nil: the log line is the boundary's first output. component is the
// metric label and the log field, so it is the component's stable short name
// ("bgp", "ospf", "l2tp").
func New(component string, log *slog.Logger) *Guard {
	return &Guard{component: component, log: log, counts: map[string]uint64{}}
}

// SetMetricsRegistry registers ze_task_panics_total in reg and publishes every
// declared task's count to it, including panics recovered before a registry
// existed. A component calls it from the hook that hands it its registry
// (ConfigureMetrics, or the reactor's metrics setup). Registering the same name
// in the same registry from several components returns the same vector.
func (g *Guard) SetMetricsRegistry(reg metrics.Registry) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.panics = reg.CounterVec(MetricName, metricHelp, []string{"component", "task"})
	for task, count := range g.counts {
		counter := g.panics.With(g.component, task)
		if count > 0 {
			counter.Add(float64(count))
		}
	}
}

// Panics returns how many panics task recovered since the guard was built, and
// false when no task of that name was declared, so a misspelled name is not
// read as zero panics.
func (g *Guard) Panics(task string) (uint64, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	count, declared := g.counts[task]
	return count, declared
}

// Task declares one guarded task of the component. name is the metric label
// and the log field ("read", "receive", "request"). restart tells the operator
// what brings the unit back, in plain words ("the session reconnects after the
// peer's connect-retry backoff"). Declare each task once, at construction.
func (g *Guard) Task(name, restart string) Task {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, declared := g.counts[name]; !declared {
		g.counts[name] = 0
		if g.panics != nil {
			g.panics.With(g.component, name)
		}
	}
	return Task{guard: g, name: name, restart: restart}
}

// Task is one guarded task of a component, built by Guard.Task.
type Task struct {
	guard   *Guard
	name    string
	restart string
}

// Context names the unit a contained panic ended. The guard reads it through
// the describe function after the panic, so what the task recorded while it
// ran (the last message type) is current.
type Context struct {
	// Unit names the unit for the operator: the peer address, the interface,
	// the neighbor, the client address.
	Unit string
	// MessageType names the message the task was handling. Empty means the task
	// tracks no message type, and the log line then carries no such field.
	MessageType string
}

// Contain is the boundary. It MUST be deferred directly at the top of the task,
// `defer task.Contain(describe, end)`, because recover only stops a panic when
// the deferred function itself calls it.
//
// With no panic it does nothing. On a panic it logs, counts and records it, then
// calls end with the recovered panic, and the task returns normally. end tears
// the unit down through the unit's existing path, and MAY set the task's named
// error result to the *Panic it receives. end may be nil when returning from the
// task is the unit's whole teardown.
//
// describe MUST be non-nil, and MUST NOT block: it reads the unit's identity.
func (t Task) Contain(describe func() Context, end func(*Panic)) {
	value := recover()
	if value == nil {
		return
	}

	p := &Panic{
		Component: t.guard.component,
		Task:      t.name,
		Context:   describe(),
		Value:     value,
		Stack:     debug.Stack(),
	}
	t.guard.record(t.restart, p)

	if end != nil {
		end(p)
	}
}

// record counts p, writes its crash record and logs it, in that order, so the
// log line can name the record the operator reads.
func (g *Guard) record(restart string, p *Panic) {
	g.mu.Lock()
	g.counts[p.Task]++
	if g.panics != nil {
		g.panics.With(g.component, p.Task).Inc()
	}
	g.mu.Unlock()

	attrs := []any{
		slog.String("component", p.Component),
		slog.String("task", p.Task),
		slog.String("unit", p.Unit),
	}
	if p.MessageType != "" {
		attrs = append(attrs, slog.String("message-type", p.MessageType))
	}
	attrs = append(attrs,
		slog.String("panic", fmt.Sprint(p.Value)),
		slog.String("restart", restart),
	)

	name, err := crashlog.RecordRecoveredPanic(p.origin(), p.Value, p.Stack)
	if err == nil {
		p.Report = name
		attrs = append(attrs, slog.String("next", showCrashesCommand+" name "+name))
	} else {
		attrs = append(attrs,
			slog.String("crash-record-error", err.Error()),
			slog.String("next", "attach this log line and its stack to the bug report"),
		)
	}
	attrs = append(attrs, slog.String("stack", string(p.Stack)))

	g.log.Error("an internal Ze defect ended this unit; the rest of Ze keeps running", attrs...)
}

// Panic is a panic a Task contained. It is an error, so a task that returns an
// error can return it to its caller's existing teardown.
type Panic struct {
	Component string
	Task      string
	Context
	// Value is what the task panicked with.
	Value any
	// Stack is the panicking goroutine's stack at the recover.
	Stack []byte
	// Report is the crash record's file name, empty when none was written.
	Report string
}

// Error names the component, the task, the unit and the panic value.
func (p *Panic) Error() string {
	return "internal Ze defect in " + p.origin() + ": panic: " + fmt.Sprint(p.Value)
}

// origin is the one-line description the crash record and Error carry.
func (p *Panic) origin() string {
	origin := p.Component + " " + p.Task + ", unit " + p.Unit
	if p.MessageType != "" {
		origin += ", message " + p.MessageType
	}
	return origin
}
