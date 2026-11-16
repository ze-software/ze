// Design: docs/architecture/api/process-protocol.md -- connection-scoped request cancellation.
// Related: dispatch.go -- the single reader enrolls before concurrent dispatch.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runtimeRequests owns the correlation identities of live requests on one
// authenticated connection, including refusal replies. Opt-in work additionally
// has at most one admission per authenticated resource. Completed entries
// disappear; this is not a selected-route or final-result inventory.
// The connection reader MUST call stop before joining dispatch goroutines.
type runtimeRequests struct {
	mu         sync.Mutex
	parent     context.Context
	connection context.Context
	stop       context.CancelFunc
	abort      context.CancelFunc
	scope      func(*engineOp, json.RawMessage) (requestScope, error)
	active     map[uint64]*runtimeRequest
	resources  map[requestResource]uint64
}

type runtimeCall struct {
	resource requestResource
	done     chan struct{}
	joining  bool
}

// runtimeRequest always owns its context cancel. selected is an optional
// cancellation capability, created only after resource validation; nil grants
// no cancel-request authority. Ordinary calls still join through the existing
// dispatch WaitGroup and allocate no extra completion channel.
type runtimeRequest struct {
	cancel   context.CancelFunc
	selected *runtimeCall
}

type requestIDCollision struct{ id uint64 }

func (collision *requestIDCollision) Error() string {
	return fmt.Sprintf("request ID %d is already live on this connection", collision.id)
}

// requestScope is returned only by an operation's admission validator after it
// authenticates a resource its existing owner already holds. It grants no route
// authority; the handler rechecks its own live rights. The generic constructor
// requires a comparable type; producers MUST use a concrete immutable owner
// key (a parsed destination or issued receipt), never an interface payload.
type requestScope struct {
	resource requestResource
	deadline time.Time
}

type requestResource struct {
	method   string
	identity any
}

func requestScopeFor[T comparable](method string, identity T, deadline time.Time) requestScope {
	return requestScope{resource: requestResource{method: method, identity: identity}, deadline: deadline}
}

type runtimeRequestKey struct{}

type runtimeRequestContext struct {
	requests *runtimeRequests
	id       uint64
}

// socketAdmission is constructed only after its context and completion pair
// exist. The dispatch owner MUST invoke finish exactly once after its handler
// and response attempt; cancellation receipts MUST wait for that completion.
type socketAdmission struct {
	ctx    context.Context
	finish func()
}

func newRuntimeRequests(parent context.Context, scope func(*engineOp, json.RawMessage) (requestScope, error)) *runtimeRequests {
	parent, abort := context.WithCancel(parent)
	connection, stop := context.WithCancel(parent)
	return &runtimeRequests{parent: parent, connection: connection, stop: stop, abort: abort, scope: scope,
		active: make(map[uint64]*runtimeRequest), resources: make(map[requestResource]uint64)}
}

// admit is called only by the single request reader. Its map mutations remain
// synchronized with dispatch completion and cancellation handlers. An opt-in
// request MUST be enrolled before the reader dispatches any later cancellation.
// Except for a fatal requestIDCollision, even a refusal returns its reservation:
// the caller MUST finish it after attempting the error response.
func (requests *runtimeRequests) admit(request *rpc.Request) (*socketAdmission, error) {
	base := requests.parent
	if request.Method == rpc.MethodCancelRequest {
		base = requests.connection
	}
	work, cancel := context.WithCancel(base)
	entry := &runtimeRequest{cancel: cancel}
	requests.mu.Lock()
	if _, active := requests.active[request.ID]; active {
		requests.mu.Unlock()
		cancel()
		return nil, &requestIDCollision{id: request.ID}
	}
	requests.active[request.ID] = entry
	requests.mu.Unlock()
	ctx := context.WithValue(work, runtimeRequestKey{}, runtimeRequestContext{requests, request.ID})
	admission := &socketAdmission{ctx: ctx, finish: func() { requests.complete(request.ID, entry) }}
	operation := lookupEngineOp(request.Method)
	if operation == nil {
		return admission, nil
	}
	if operation.cancellationScope == nil {
		return admission, nil
	}
	if request.ID == 0 {
		return admission, errors.New("cancellable request ID must be nonzero")
	}
	scope, err := requests.scope(operation, request.Params)
	if err != nil {
		return admission, err
	}
	if scope.resource.method != request.Method {
		return admission, errors.New("cancellable resource belongs to another operation")
	}
	deadline := scope.deadline
	if !deadline.After(time.Now()) {
		return admission, context.DeadlineExceeded
	}
	ceiling := time.Now().Add(rpc.CancellableRequestTimeout)
	if deadline.After(ceiling) {
		deadline = ceiling
	}
	work, scopedCancel := context.WithDeadline(requests.connection, deadline)
	ctx = context.WithValue(work, runtimeRequestKey{}, runtimeRequestContext{requests, request.ID})
	call := &runtimeCall{resource: scope.resource, done: make(chan struct{})}
	requests.mu.Lock()
	if _, active := requests.resources[scope.resource]; active {
		requests.mu.Unlock()
		scopedCancel()
		return admission, errors.New("cancellable resource already has an admitted request")
	}
	entry.cancel = scopedCancel
	entry.selected = call
	requests.resources[scope.resource] = request.ID
	requests.mu.Unlock()
	cancel()
	admission.ctx = ctx
	return admission, nil
}

func (requests *runtimeRequests) complete(id uint64, entry *runtimeRequest) {
	entry.cancel()
	requests.mu.Lock()
	delete(requests.active, id)
	if entry.selected != nil {
		delete(requests.resources, entry.selected.resource)
		close(entry.selected.done)
	}
	requests.mu.Unlock()
}

// cancelAll is used only for a fatal correlation collision. Ordinary calls
// otherwise retain their server context through EOF as before. No request is
// joined here; the connection owner MUST still join its dispatch WaitGroup.
// On a normal reader exit it MUST also call this after the join to release the
// connection's parent reference without ending ordinary handlers early.
func (requests *runtimeRequests) cancelAll() { requests.abort() }

func selectedCancellationScope(s *Server, proc *process.Process, params json.RawMessage) (requestScope, error) {
	destination, deadline, err := selectedCancellationHeader(params)
	if err != nil {
		return requestScope{}, err
	}
	if !s.DeliveryGraph().MaySend(destination.String(), proc.Name(), "update") {
		return requestScope{}, fmt.Errorf("select-routes: process has no update send grant for destination %s", destination)
	}
	return requestScopeFor(rpc.MethodSelectRoutes, destination, deadline), nil
}

func selectedCancellationHeader(params json.RawMessage) (netip.Addr, time.Time, error) {
	// Decode the scalar envelope only; route arrays remain raw for the shared
	// selected constructor. The transport already bounds the complete message.
	var envelope struct {
		Destination string `json:"destination"`
		DeadlineMS  int64  `json:"deadline-ms"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return netip.Addr{}, time.Time{}, err
	}
	destination, err := netip.ParseAddr(envelope.Destination)
	if err != nil {
		return netip.Addr{}, time.Time{}, err
	}
	if envelope.DeadlineMS <= 0 {
		return netip.Addr{}, time.Time{}, errors.New("select-routes: positive deadline-ms is required")
	}
	return destination, time.UnixMilli(envelope.DeadlineMS), nil
}

func (s *Server) opCancelRequest(ctx context.Context, _ *process.Process, params json.RawMessage) (any, error) {
	var input rpc.CancelRequestInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, err
	}
	current, found := ctx.Value(runtimeRequestKey{}).(runtimeRequestContext)
	if !found {
		return nil, errors.New("cancel-request requires its authenticated socket connection")
	}
	if input.RequestID == 0 {
		return nil, errors.New("cancel-request requires a nonzero request-id")
	}
	if input.RequestID >= current.id {
		return nil, errors.New("cancel-request cannot target itself or a future request")
	}
	operation := lookupEngineOp(input.Method)
	if operation == nil {
		return nil, errors.New("cancel-request names an unknown operation")
	}
	if operation.cancellationScope == nil {
		return nil, errors.New("cancel-request operation does not support acknowledged cancellation")
	}
	return current.requests.cancelAndJoin(ctx, input.RequestID, input.Method)
}

func (requests *runtimeRequests) cancelAndJoin(ctx context.Context, id uint64, method string) (*rpc.CancelRequestOutput, error) {
	requests.mu.Lock()
	entry, active := requests.active[id]
	if !active {
		requests.mu.Unlock()
		return &rpc.CancelRequestOutput{RequestID: id, Outcome: rpc.CancelRequestNotActive}, nil
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
		return &rpc.CancelRequestOutput{RequestID: id, Outcome: rpc.CancelRequestSettled}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
