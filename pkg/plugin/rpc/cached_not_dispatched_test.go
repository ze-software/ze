// Design: docs/architecture/api/process-protocol.md -- refusal evidence requires a cause.
package rpc

import (
	"errors"
	"testing"
)

// TestCachedWriteNotDispatchedRequiresActualCause exercises the public
// constructor: missing evidence cannot panic or claim either settlement class,
// while an actual local refusal keeps its original cause and classification.
func TestCachedWriteNotDispatchedRequiresActualCause(t *testing.T) {
	missing := CachedWriteNotDispatched(nil)
	if missing == nil {
		t.Fatal("missing local refusal evidence became success")
	}
	if missing.Error() == "" {
		t.Fatal("missing local refusal evidence has no diagnostic")
	}
	if _, known := errors.AsType[*CachedWriteNotDispatchedError](missing); known {
		t.Fatal("missing cause became proof that no dispatch occurred")
	}
	if _, joined := errors.AsType[*CachedWriteJoinedError](missing); joined {
		t.Fatal("missing cause became proof that target writers joined")
	}

	cause := errors.New("original local shape refusal")
	refused := CachedWriteNotDispatched(cause)
	if !errors.Is(refused, cause) {
		t.Fatal("actual local refusal lost its original cause")
	}
	if refused.Error() != cause.Error() {
		t.Fatal("actual local refusal changed its diagnostic")
	}
	if _, known := errors.AsType[*CachedWriteNotDispatchedError](refused); !known {
		t.Fatal("actual local refusal lost its classification")
	}
	if _, joined := errors.AsType[*CachedWriteJoinedError](refused); joined {
		t.Fatal("actual local refusal became remote writer completion")
	}
}
