// Design: docs/architecture/container/image-store.md -- the fetch: the worker, the queue, the in-flight row, the failed row
// Related: fetch_steps.go -- what the worker does for one fetch
// Related: fetch_apply_linux.go -- the layer application the worker calls on Linux
// Related: fetch_apply_other.go -- the refusal on the systems containers do not run on

package imagestore

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// The bounds of the fetch queue and of the listing's failed rows (the
// design's boundary table).
const (
	// fetchWaitingMax is the most fetches that wait behind the one running.
	fetchWaitingMax = 8
	// failedRowsMax is the most failed rows the listing keeps. The oldest
	// leaves first.
	failedRowsMax = 16
	// fetchAnswerWaitDefault bounds how long the fetch command waits for its
	// fetch. A plugin command is cut at 30 seconds and the command
	// declaration has no field for a longer bound yet, so the command answers
	// before the cut: a fetch that runs longer goes on in the worker, and the
	// listing shows it fetching, then present, or failed with its code.
	fetchAnswerWaitDefault = 20 * time.Second
)

// fetchJob is one fetch the queue holds or the worker runs. received is
// written by the worker and read by the listing. answer and err are written
// once, before done is closed, and read only after it is.
type fetchJob struct {
	// ctx is the context of the command that started the fetch. A plugin
	// command runs under the plugin's run context, so the fetch stops when
	// the plugin stops, and not when its command answered.
	ctx       context.Context //nolint:containedctx // the context of the command that started the fetch, carried to the worker that runs it
	reference registry.Reference
	received  atomic.Uint64
	// resolved is the digest the reference resolved to, nil until the
	// registry answered. The listing drops the row of a fetch whose digest
	// is already published: publication comes before the fetch's end.
	resolved atomic.Pointer[string]
	done     chan struct{}
	answer   container.FetchAnswer
	err      error
}

// fetchQueue is the store's fetch state: the fetch the worker runs, the
// fetches that wait for it, and the rows of the fetches that failed. Safe for
// concurrent use.
type fetchQueue struct {
	mu sync.Mutex
	// running is the fetch the worker runs, nil when no worker runs.
	running *fetchJob
	waiting []*fetchJob
	// failed holds the row of the last failed fetch of each reference, the
	// oldest first.
	failed []container.ImageRow
}

// fetchError is a failed fetch: the operator error its caller reads, and the
// doctor code the failed row of the listing names.
type fetchError struct {
	code string
	err  *opererr.Error
}

// Error answers the operator error's text.
func (e *fetchError) Error() string { return e.err.Error() }

// Unwrap answers the operator error.
func (e *fetchError) Unwrap() error { return e.err }

// FetchImage fetches the image text names and answers at the end of the
// fetch, or when the fetch has run for the answer wait. The fetch runs on the
// store's one worker, so fetches run one at a time and at most eight wait. A
// fetch of a reference already queued or running is joined, never repeated.
//
// The answer wait exists because a plugin command is cut at 30 seconds: a
// fetch that takes longer is answered with an operator error that says it is
// still running and names the listing, and it goes on to its end in the
// worker, so the cut costs the caller the wait and never the image.
func (s *Store) FetchImage(ctx context.Context, text string) (container.FetchAnswer, error) {
	reference, err := registry.ParseReference(text)
	if err != nil {
		return container.FetchAnswer{}, opererr.Wrap(err,
			"create container image was not run: the image reference is not one the store can complete",
			"a reference is an optional registry host, a repository in lower case, and a tag or a digest",
			container.FixFetch("REFERENCE"),
		)
	}
	job, err := s.enqueueFetch(ctx, reference)
	if err != nil {
		return container.FetchAnswer{}, err
	}
	wait := time.NewTimer(s.answerWait)
	defer wait.Stop()
	select {
	case <-job.done:
		if job.err != nil {
			return container.FetchAnswer{}, operatorError(job.err)
		}
		return job.answer, nil
	case <-wait.C:
		return container.FetchAnswer{}, opererr.New(
			"image "+reference.String()+" is still being fetched, and the command answers before its fetch ends",
			"a plugin command is answered within 30 seconds; the fetch goes on, and the image listing shows its bytes received, then the image present, or the failure and its code",
			container.FixShowImageList,
		)
	case <-ctx.Done():
		return container.FetchAnswer{}, opererr.Wrap(ctx.Err(),
			"the fetch of image "+reference.String()+" was stopped with the container plugin",
			"the checked blobs stay, and the next fetch of the image downloads only the others",
			container.FixFetch(reference.String()),
		)
	}
}

// operatorError answers the operator error of a failed fetch, without the
// code its row carries: the caller reads the problem, why and fix.
func operatorError(err error) error {
	if failure, ok := errors.AsType[*fetchError](err); ok {
		return failure.err
	}
	return err
}

// enqueueFetch hands a fetch of reference to the worker and answers its job.
//
// The worker is started here when none runs: it takes the fetches one at a
// time and ends when none waits, so the store holds at most one worker, and
// none while it is idle. Its stop path is the empty queue, and each fetch it
// runs stops with the context of the command that started it.
func (s *Store) enqueueFetch(ctx context.Context, reference registry.Reference) (*fetchJob, error) {
	q := &s.fetches
	q.mu.Lock()
	defer q.mu.Unlock()
	if job := q.find(reference); job != nil {
		return job, nil
	}
	job := &fetchJob{ctx: ctx, reference: reference, done: make(chan struct{})}
	if q.running == nil {
		q.running = job
		go s.runFetches(job)
		return job, nil
	}
	if len(q.waiting) == fetchWaitingMax {
		return nil, opererr.New(
			"image "+reference.String()+" was not fetched: eight fetches already wait for the one running",
			"the store fetches one image at a time, and a queue without a bound would hold the memory of every caller",
			container.FixShowImageList,
		)
	}
	q.waiting = append(q.waiting, job)
	return job, nil
}

// runFetches is the store's worker. It runs job, then each fetch that waits,
// and returns when none does.
func (s *Store) runFetches(job *fetchJob) {
	for job != nil {
		answer, err := s.fetch(job)
		job = s.fetches.finish(job, answer, err)
	}
}

// find answers the queued or running fetch of reference, or nil. The caller
// MUST hold q.mu.
func (q *fetchQueue) find(reference registry.Reference) *fetchJob {
	if q.running != nil && q.running.reference == reference {
		return q.running
	}
	for _, job := range q.waiting {
		if job.reference == reference {
			return job
		}
	}
	return nil
}

// standing answers what the queue knows of reference, for an image no
// binding names: Absent with its fetch running or waiting, or with its last
// fetch failed and that failure's sentence and doctor code. It answers false
// when the queue holds neither.
func (q *fetchQueue) standing(reference registry.Reference) (container.Absent, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	whole := reference.String()
	if q.find(reference) != nil {
		return container.Absent{Reference: whole, Reason: container.AbsentFetchRunning}, true
	}
	for _, row := range q.failed {
		if row.Reference == whole {
			return container.Absent{Reference: whole, Reason: container.AbsentFetchFailed, Detail: row.Error + " (" + row.Code + ")"}, true
		}
	}
	return container.Absent{}, false
}

// finish records the end of job, wakes its callers, and answers the next
// fetch to run, or nil when none waits, which ends the worker.
func (q *fetchQueue) finish(job *fetchJob, answer container.FetchAnswer, err error) *fetchJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.forgetFailure(job.reference.String())
	if err != nil {
		q.rememberFailure(failedRow(job.reference.String(), err))
	}
	job.answer, job.err = answer, err
	close(job.done)
	if len(q.waiting) == 0 {
		q.running = nil
		return nil
	}
	next := q.waiting[0]
	q.waiting = q.waiting[1:]
	q.running = next
	return next
}

// forgetFailure removes the failed row of reference: a fetch of it ended. The
// caller MUST hold q.mu.
func (q *fetchQueue) forgetFailure(reference string) {
	for i, row := range q.failed {
		if row.Reference == reference {
			q.failed = append(q.failed[:i], q.failed[i+1:]...)
			return
		}
	}
}

// rememberFailure keeps row, dropping the oldest failed row past the bound.
// The caller MUST hold q.mu.
func (q *fetchQueue) rememberFailure(row container.ImageRow) {
	if len(q.failed) == failedRowsMax {
		q.failed = q.failed[1:]
	}
	q.failed = append(q.failed, row)
}

// failedRow answers the listing row of a fetch of reference that failed with
// err: its sentence and its doctor code. An error that carries no code of its
// own is the store's: a folder of the store that could not be written or read.
func failedRow(reference string, err error) container.ImageRow {
	row := container.ImageRow{
		Reference: reference,
		State:     container.ImageStateFailed,
		UsedBy:    []string{},
		Error:     err.Error(),
		Code:      container.CodeStoreUnwritable,
	}
	if failure, ok := errors.AsType[*fetchError](err); ok {
		row.Error = failure.err.Problem()
		row.Code = failure.code
		return row
	}
	if refusal, ok := errors.AsType[*opererr.Error](err); ok {
		row.Error = refusal.Problem()
	}
	return row
}

// rows answers the rows of the fetches in flight, the running one with the
// bytes it received, and the rows of the fetches that failed.
func (q *fetchQueue) rows() []queuedRow {
	q.mu.Lock()
	defer q.mu.Unlock()
	rows := make([]queuedRow, 0, 1+len(q.waiting)+len(q.failed))
	if q.running != nil {
		rows = append(rows, fetchingRow(q.running))
	}
	for _, job := range q.waiting {
		rows = append(rows, fetchingRow(job))
	}
	for _, failed := range q.failed {
		rows = append(rows, queuedRow{row: failed})
	}
	return rows
}

// queuedRow is a listing row of the fetch queue. resolved is the digest a
// fetch in flight resolved its reference to, and is empty for a fetch the
// registry has not answered yet and for a failed one.
type queuedRow struct {
	row      container.ImageRow
	resolved string
}

// fetchingRow answers the row of a fetch in flight. A waiting fetch has
// received nothing yet.
func fetchingRow(job *fetchJob) queuedRow {
	queued := queuedRow{row: container.ImageRow{
		Reference: job.reference.String(),
		State:     container.ImageStateFetching,
		Received:  job.received.Load(),
		UsedBy:    []string{},
	}}
	if resolved := job.resolved.Load(); resolved != nil {
		queued.resolved = *resolved
	}
	return queued
}
