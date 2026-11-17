// Design: docs/architecture/api/process-protocol.md -- joined mutating callbacks.
package rpc

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestReceiverCancellationJoinsReplyCompletion(t *testing.T) {
	const method = "ze-plugin-callback:receiver-test"
	receiver := NewRequestReceiver(t.Context(), func(request *Request) (RequestScope, error) {
		if request.Method != method {
			return RequestScope{}, nil
		}
		return NewRequestScope(method, uint64(3), time.Now().Add(time.Second)), nil
	})
	defer receiver.CancelAll()
	defer receiver.Stop()
	admission, err := receiver.Admit(&Request{ID: 7, Method: method})
	require.NoError(t, err)
	duplicate, err := receiver.Admit(&Request{ID: 8, Method: method})
	require.Error(t, err)
	duplicate.Finish()
	joining := make(chan error, 1)
	go func() {
		result, err := receiver.CancelAndJoin(t.Context(), 7, method)
		if err == nil && result.Outcome != CancelRequestSettled {
			err = errors.New("missing settled receipt")
		}
		joining <- err
	}()
	select {
	case <-admission.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("request was not canceled")
	}
	select {
	case err := <-joining:
		t.Fatalf("unfinished request acknowledged: %v", err)
	default:
	}
	// Handler exit alone does not finish: the reply attempt belongs to admission.
	admission.Finish()
	require.NoError(t, <-joining)
	active, resources := receiver.Counts()
	require.Zero(t, active)
	require.Zero(t, resources)
}

func TestRequestReceiverScopeAndRequestIdentityAreNotInterchangeable(t *testing.T) {
	const method = "ze-plugin-callback:receiver-test"
	receiver := NewRequestReceiver(t.Context(), func(request *Request) (RequestScope, error) {
		return NewRequestScope(method, request.ID, time.Now().Add(time.Second)), nil
	})
	defer receiver.CancelAll()
	admission, err := receiver.Admit(&Request{ID: 1, Method: method})
	require.NoError(t, err)
	defer admission.Finish()
	_, err = receiver.Admit(&Request{ID: 1, Method: method})
	var collision *RequestIDCollision
	require.ErrorAs(t, err, &collision)
	_, err = receiver.CancelAndJoin(t.Context(), 1, "other-method")
	require.Error(t, err)
	require.NoError(t, admission.Context.Err())
}

func TestRequestReceiverCorrelatedCancellationKeepsParallelCommands(t *testing.T) {
	const method = "ze-plugin-engine:update-route"
	receiver := NewRequestReceiver(t.Context(), func(*Request) (RequestScope, error) {
		return NewRequestCancellationScope(method, time.Now().Add(time.Second)), nil
	})
	defer receiver.CancelAll()
	first, err := receiver.Admit(&Request{ID: 7, Method: method})
	require.NoError(t, err)
	second, err := receiver.Admit(&Request{ID: 8, Method: method})
	require.NoError(t, err)
	defer second.Finish()
	active, resources := receiver.Counts()
	require.Equal(t, 2, active)
	require.Zero(t, resources)
	joined := make(chan error, 1)
	go func() { _, err := receiver.CancelAndJoin(t.Context(), 7, method); joined <- err }()
	select {
	case <-first.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("first command did not cancel")
	}
	require.NoError(t, second.Context.Err(), "one correlation canceled a parallel command")
	select {
	case err := <-joined:
		t.Fatalf("unfinished first command settled: %v", err)
	default:
	}
	first.Finish()
	require.NoError(t, <-joined)
	require.NoError(t, second.Context.Err())
}
