// Design: docs/architecture/dns/server-harness.md -- the per-query panic boundary
// both wrappers defer
// Related: handler.go -- Authoritative defers the authoritative-query task
// Related: recursive.go -- Recursive defers the recursive-query task
// Related: metrics.go -- SetMetricsRegistry publishes the guard's counter

package dnsserver

import (
	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// The task names of queryGuard, the task label of ze_task_panics_total. One per
// wrapper, so an operator tells an authoritative answer func's defect from a
// recursive one's without reading the stack.
const (
	taskAuthoritativeQuery = "authoritative-query"
	taskRecursiveQuery     = "recursive-query"
)

// queryRestart tells the operator what follows a contained query panic: the
// unit is one query, so nothing has to come back.
const queryRestart = "nothing restarts: this query drew no reply, its client retries or gives up, and the listener keeps answering other queries"

// queryGuard is the panic boundary of every query the harness answers. It is
// the harness's, not the consumer's, so no consumer can build a handler that
// recovers a panic in silence: the line, the ze_task_panics_total{component="dns"}
// count and the crash record happen whatever the consumer passes, a nil log
// included. Its line goes to the dns.server subsystem logger for the same
// reason, since a nil log means the consumer wants no write-failure line, never
// that a Ze defect should go unreported.
//
// A test may replace it before building a handler and restore it after: each
// wrapper declares its task on queryGuard when it is built.
var queryGuard = taskguard.New("dns", slogutil.Logger("dns.server"))

// describeQuery names the query a contained panic ended: the client address
// and the first question, which is all a standard query carries. It runs only
// after a panic, so it reads both lazily and tolerates a writer with no
// address.
func describeQuery(w dns.ResponseWriter, r *dns.Msg) func() taskguard.Context {
	return func() taskguard.Context {
		unit := "query from an unknown client"
		if addr := w.RemoteAddr(); addr != nil {
			unit = "query from " + addr.String()
		}
		if len(r.Question) > 0 {
			question := r.Question[0]
			unit += ", question " + question.Name + " " + dns.TypeToString[question.Qtype]
		}
		return taskguard.Context{Unit: unit}
	}
}
