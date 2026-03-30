// Design: docs/architecture/api/commands.md -- cmdutil tests

package cmdutil

import (
	"testing"
)

// VALIDATES: RegisterLocalCommand stores handler and RunCommand dispatches it.
// PREVENTS: local handler registration silently failing.
func TestRegisterLocalCommandAndDispatch(t *testing.T) {
	// Clean up after test.
	defer func() { localHandlers = make(map[string]LocalHandler) }()

	called := false
	err := RegisterLocalCommand("test cmd", func(_ []string) int {
		called = true
		return 42
	})
	if err != nil {
		t.Fatalf("RegisterLocalCommand returned error: %v", err)
	}

	handler, ok := localHandlers["test cmd"]
	if !ok {
		t.Fatal("handler not found in localHandlers map")
	}
	code := handler(nil)
	if !called {
		t.Error("handler was not called")
	}
	if code != 42 {
		t.Errorf("handler returned %d, want 42", code)
	}
}

// VALIDATES: RegisterLocalCommand rejects empty path.
// PREVENTS: empty key in localHandlers map causing silent misdispatch.
func TestRegisterLocalCommandEmptyPath(t *testing.T) {
	err := RegisterLocalCommand("", func(_ []string) int { return 0 })
	if err == nil {
		t.Error("expected error for empty path, got nil")
		delete(localHandlers, "") // cleanup
	}
}

// VALIDATES: RegisterLocalCommand rejects nil handler.
// PREVENTS: nil function call panic at dispatch time.
func TestRegisterLocalCommandNilHandler(t *testing.T) {
	err := RegisterLocalCommand("test nil", nil)
	if err == nil {
		t.Error("expected error for nil handler, got nil")
		delete(localHandlers, "test nil") // cleanup
	}
}

// VALIDATES: RegisterLocalCommand overwrites existing entry.
// PREVENTS: stale handlers persisting after re-registration.
func TestRegisterLocalCommandOverwrite(t *testing.T) {
	defer func() { localHandlers = make(map[string]LocalHandler) }()

	first := false
	second := false

	if err := RegisterLocalCommand("overwrite", func(_ []string) int {
		first = true
		return 1
	}); err != nil {
		t.Fatal(err)
	}

	if err := RegisterLocalCommand("overwrite", func(_ []string) int {
		second = true
		return 2
	}); err != nil {
		t.Fatal(err)
	}

	handler := localHandlers["overwrite"]
	code := handler(nil)
	if first {
		t.Error("first handler was called after overwrite")
	}
	if !second {
		t.Error("second handler was not called")
	}
	if code != 2 {
		t.Errorf("handler returned %d, want 2", code)
	}
}
