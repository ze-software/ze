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

// VALIDATES: matchLocalHandler finds longest prefix and passes remaining args.
// PREVENTS: wrong prefix matching or lost arguments.
func TestMatchLocalHandler(t *testing.T) {
	defer func() { localHandlers = make(map[string]LocalHandler) }()

	// Register handlers for testing.
	short := func(_ []string) int { return 1 }
	long := func(_ []string) int { return 2 }
	if err := RegisterLocalCommand("show bgp", short); err != nil {
		t.Fatal(err)
	}
	if err := RegisterLocalCommand("show bgp decode", long); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		words    []string
		selector string
		wantCode int // -1 means no match (nil handler)
		wantArgs []string
	}{
		{"exact match", []string{"show", "bgp", "decode"}, "", 2, nil},
		{"prefix with remaining args", []string{"show", "bgp", "decode", "--update", "FF"}, "", 2, []string{"--update", "FF"}},
		{"shorter prefix", []string{"show", "bgp", "foo"}, "", 1, []string{"foo"}},
		{"longest wins over shorter", []string{"show", "bgp", "decode", "x"}, "", 2, []string{"x"}},
		{"with selector", []string{"show", "bgp", "decode"}, "1.2.3.4", 2, []string{"1.2.3.4"}},
		{"args and selector", []string{"show", "bgp", "decode", "x"}, "1.2.3.4", 2, []string{"x", "1.2.3.4"}},
		{"no match", []string{"unknown", "cmd"}, "", -1, nil},
		{"empty words", nil, "", -1, nil},
		{"single word no match", []string{"version"}, "", -1, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, args := matchLocalHandler(tt.words, tt.selector)
			if tt.wantCode == -1 {
				if handler != nil {
					t.Error("expected nil handler, got non-nil")
				}
				return
			}
			if handler == nil {
				t.Fatal("expected handler, got nil")
			}
			code := handler(nil)
			if code != tt.wantCode {
				t.Errorf("handler returned %d, want %d", code, tt.wantCode)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, a, tt.wantArgs[i])
				}
			}
		})
	}
}
