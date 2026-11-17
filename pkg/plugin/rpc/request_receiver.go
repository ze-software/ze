// Design: docs/architecture/api/process-protocol.md -- connection-scoped request cancellation.
// Related: internal/component/plugin/server/request_cancel.go -- engine receiver adapter.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RequestReceiver owns live request correlations and optional resource scopes
// on one authenticated connection. Completed entries disappear. The single
// reader enrolls before dispatch; its owner stops admission before joining all
// dispatch workers. This registry grants cancellation, never route authority.
type RequestReceiver struct {
	mu         sync.Mutex
	parent     context.Context
	connection context.Context
	stop       context.CancelFunc
	abort      context.CancelFunc
	scope      func(*Request) (RequestScope, error)
	active     map[uint64]*receiverRequest
	resources  map[requestResource]uint64
}

type receiverCall struct {
	exclusive bool
	resource  requestResource
	done      chan struct{}
	joining   bool
}

// receiverRequest always owns its context cancel. selected is an optional
// cancellation capability, created only after resource validation; nil grants
// no cancel-request authority. Ordinary calls still join through the existing
// dispatch WaitGroup and allocate no extra completion channel.
type receiverRequest struct {
	cancel   context.CancelFunc
	selected *receiverCall
}

// RequestIDCollision refuses an ID whose original handler still owns completion.
type RequestIDCollision struct{ ID uint64 }

func (collision *RequestIDCollision) Error() string {
	return fmt.Sprintf("request ID %d is already live on this connection", collision.ID)
}

// RequestScope is returned only by an operation's admission validator after it
// authenticates a resource its existing owner already holds. It grants no route
// authority; the handler rechecks its own live rights. The generic constructor
// requires a comparable type; producers MUST use a concrete immutable owner
// key (a parsed destination or issued receipt), never an interface payload.
type RequestScope struct {
	exclusive bool
	resource  requestResource
	deadline  time.Time
}

type requestResource struct {
	method   string
	identity any
}

// NewRequestScope binds a validated comparable identity to its operation and deadline.
func NewRequestScope[T comparable](method string, identity T, deadline time.Time) RequestScope {
	return RequestScope{exclusive: true, resource: requestResource{method: method, identity: identity}, deadline: deadline}
}

// NewRequestCancellationScope enables joined cancellation for one authenticated
// request correlation without claiming exclusive ownership of a domain resource.
// Parallel calls retain independent contexts and completion receipts. This is
// for an existing concurrent operation; route/resource authorization remains in
// that operation's evaluator. Selected/source scopes use NewRequestScope.
func NewRequestCancellationScope(method string, deadline time.Time) RequestScope {
	return RequestScope{resource: requestResource{method: method}, deadline: deadline}
}

// RequestAdmission is constructed only after its context and completion pair
// exist. The dispatch owner MUST invoke finish exactly once after its handler
// and response attempt; cancellation receipts MUST wait for that completion.
type RequestAdmission struct {
	Context context.Context
	Finish  func()
}

// NewRequestReceiver captures one connection lifetime and its synchronous scope validator.
func NewRequestReceiver(parent context.Context, scope func(*Request) (RequestScope, error)) *RequestReceiver {
	parent, abort := context.WithCancel(parent)
	connection, stop := context.WithCancel(parent)
	return &RequestReceiver{parent: parent, connection: connection, stop: stop, abort: abort, scope: scope,
		active: make(map[uint64]*receiverRequest), resources: make(map[requestResource]uint64)}
}

// Admit is called only by the single request reader. Its map mutations remain
// synchronized with dispatch completion and cancellation handlers. An opt-in
// request MUST be enrolled before the reader dispatches any later cancellation.
// Except for a fatal RequestIDCollision, even a refusal returns its reservation:
// the caller MUST finish it after attempting the error response.
func (requests *RequestReceiver) Admit(request *Request) (*RequestAdmission, error) {
	base := requests.parent
	if request.Method == MethodCancelRequest {
		base = requests.connection
	}
	work, cancel := context.WithCancel(base)
	entry := &receiverRequest{cancel: cancel}
	requests.mu.Lock()
	if _, active := requests.active[request.ID]; active {
		requests.mu.Unlock()
		cancel()
		return nil, &RequestIDCollision{ID: request.ID}
	}
	requests.active[request.ID] = entry
	requests.mu.Unlock()
	ctx := work
	admission := &RequestAdmission{Context: ctx, Finish: func() { requests.complete(request.ID, entry) }}
	scope, err := requests.scope(request)
	if err != nil {
		return admission, err
	}
	if scope.resource.method == "" {
		return admission, nil
	}
	if request.ID == 0 {
		return admission, errors.New("cancellable request ID must be nonzero")
	}
	if scope.resource.method != request.Method {
		return admission, errors.New("cancellable resource belongs to another operation")
	}
	deadline := scope.deadline
	if !deadline.After(time.Now()) {
		return admission, context.DeadlineExceeded
	}
	ceiling := time.Now().Add(CancellableRequestTimeout)
	if deadline.After(ceiling) {
		deadline = ceiling
	}
	work, scopedCancel := context.WithDeadline(requests.connection, deadline)
	ctx = work
	call := &receiverCall{exclusive: scope.exclusive, resource: scope.resource, done: make(chan struct{})}
	requests.mu.Lock()
	if _, active := requests.resources[scope.resource]; scope.exclusive && active {
		requests.mu.Unlock()
		scopedCancel()
		return admission, errors.New("cancellable resource already has an admitted request")
	}
	entry.cancel = scopedCancel
	entry.selected = call
	if scope.exclusive {
		requests.resources[scope.resource] = request.ID
	}
	requests.mu.Unlock()
	cancel()
	admission.Context = ctx
	return admission, nil
}

func (requests *RequestReceiver) complete(id uint64, entry *receiverRequest) {
	entry.cancel()
	requests.mu.Lock()
	delete(requests.active, id)
	if entry.selected != nil {
		if entry.selected.exclusive {
			delete(requests.resources, entry.selected.resource)
		}
		close(entry.selected.done)
	}
	requests.mu.Unlock()
}

// CancelAll aborts every request for a fatal correlation collision. Ordinary calls
// otherwise retain their server context through EOF as before. No request is
// joined here; the connection owner MUST still join its dispatch WaitGroup.
// On a normal reader exit it MUST also call this after the join to release the
// connection's parent reference without ending ordinary handlers early.
func (requests *RequestReceiver) CancelAll() { requests.abort() }

// CancelAndJoin settles an already validated request on this exact connection.
// The ingress adapter must validate the target method and earlier nonzero ID.
func (requests *RequestReceiver) CancelAndJoin(ctx context.Context, id uint64, method string) (*CancelRequestOutput, error) {
	requests.mu.Lock()
	entry, active := requests.active[id]
	if !active {
		requests.mu.Unlock()
		return &CancelRequestOutput{RequestID: id, Outcome: CancelRequestNotActive}, nil
	}
	call := entry.selected
	if call == nil {
		requests.mu.Unlock()
		return nil, errors.New("cancel-request target has no cancellation admission")
	}
	if call.resource.method != method {
		requests.mu.Unlock()
		return nil, errors.New("cancel-request operation differs from the admitted request")
	}
	if call.joining {
		requests.mu.Unlock()
		return nil, errors.New("cancel-request already has a settlement waiter for this request")
	}
	call.joining = true
	requests.mu.Unlock()
	entry.cancel()
	select {
	case <-call.done:
		return &CancelRequestOutput{RequestID: id, Outcome: CancelRequestSettled}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Stop cancels opt-in work before the connection owner joins its workers.
// Ordinary requests keep their parent lifetime until CancelAll.
func (requests *RequestReceiver) Stop() { requests.stop() }

// Counts reports retained live bookkeeping for lifecycle checks.
func (requests *RequestReceiver) Counts() (int, int) {
	requests.mu.Lock()
	defer requests.mu.Unlock()
	return len(requests.active), len(requests.resources)
}

// RequestForScope reports current enrollment without granting authority.
func (requests *RequestReceiver) RequestForScope(scope RequestScope) uint64 {
	requests.mu.Lock()
	defer requests.mu.Unlock()
	return requests.resources[scope.resource]
}
