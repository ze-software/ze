package show

import (
	"testing"
	"time"
)

// TestHandleShowSystemMemory asserts that the handler returns the expected
// kebab-case keys and that `alloc` is a positive integer (the test process
// has allocated memory by this point).
//
// VALIDATES: AC-2 of spec-op-1-easy-wins.md -- `show system memory` reply
//
//	contains runtime MemStats fields with kebab-case keys.
func TestHandleShowSystemMemory(t *testing.T) {
	resp, err := handleShowSystemMemory(nil, nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resp.Data)
	}
	for _, key := range []string{
		"alloc", "total-alloc", "sys",
		"heap-alloc", "heap-sys", "heap-in-use", "heap-objects",
		"stack-in-use", "num-gc", "gc-cpu-pct",
	} {
		if _, present := data[key]; !present {
			t.Errorf("missing key %q in response", key)
		}
	}
	v, ok := data["alloc"].(uint64)
	if !ok {
		t.Fatalf("alloc has wrong type: %T", data["alloc"])
	}
	if v == 0 {
		t.Error("expected non-zero alloc in a running process")
	}
}

// TestHandleShowSystemCPU asserts num-cpu, num-goroutines, max-procs are
// all positive integers and go-version is non-empty.
//
// VALIDATES: AC-3 of spec-op-1-easy-wins.md.
func TestHandleShowSystemCPU(t *testing.T) {
	resp, err := handleShowSystemCPU(nil, nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resp.Data)
	}
	for _, key := range []string{"num-cpu", "num-goroutines", "max-procs"} {
		raw, present := data[key]
		if !present {
			t.Errorf("missing key %q", key)
			continue
		}
		v, ok := raw.(int)
		if !ok {
			t.Errorf("%s has wrong type: %T", key, raw)
			continue
		}
		if v <= 0 {
			t.Errorf("%s should be > 0, got %d", key, v)
		}
	}
	gv, ok := data["go-version"].(string)
	if !ok {
		t.Fatalf("go-version has wrong type: %T", data["go-version"])
	}
	if gv == "" {
		t.Error("go-version should be non-empty")
	}
}

// TestHandleShowSystemDate asserts `time` parses as RFC3339 and is close
// to the wall clock (within one second) at call time.
//
// VALIDATES: AC-4 of spec-op-1-easy-wins.md.
func TestHandleShowSystemDate(t *testing.T) {
	before := time.Now()
	resp, err := handleShowSystemDate(nil, nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	after := time.Now()
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resp.Data)
	}
	ts, ok := data["time"].(string)
	if !ok {
		t.Fatalf("time missing or wrong type: %v", data["time"])
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("time not RFC3339: %v", err)
	}
	if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("time %v outside [%v, %v]", parsed, before, after)
	}
	if _, ok := data["unix"].(int64); !ok {
		t.Errorf("unix field missing or wrong type: %T", data["unix"])
	}
	if _, ok := data["timezone"].(string); !ok {
		t.Errorf("timezone field missing or wrong type: %T", data["timezone"])
	}
}
