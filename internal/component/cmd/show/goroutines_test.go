package show

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestParseGoroutineStacks(t *testing.T) {
	input := `goroutine 1 [running]:
main.main()
	/tmp/main.go:10 +0x44

goroutine 42 [chan receive, 5 minutes]:
runtime.gopark(...)
	/usr/local/go/src/runtime/proc.go:401

goroutine 99 [select]:
net.(*netFD).Read(...)
	/usr/local/go/src/net/fd_posix.go:55`

	goroutines := parseGoroutineStacks(input)
	if len(goroutines) != 3 {
		t.Fatalf("got %d goroutines, want 3", len(goroutines))
	}

	if goroutines[0].ID != "1" || goroutines[0].State != "running" {
		t.Errorf("goroutine 0: id=%q state=%q, want id=1 state=running", goroutines[0].ID, goroutines[0].State)
	}
	if goroutines[1].ID != "42" || goroutines[1].State != "chan receive" {
		t.Errorf("goroutine 1: id=%q state=%q, want id=42 state='chan receive'", goroutines[1].ID, goroutines[1].State)
	}
	if goroutines[2].ID != "99" || goroutines[2].State != "select" {
		t.Errorf("goroutine 2: id=%q state=%q, want id=99 state=select", goroutines[2].ID, goroutines[2].State)
	}
}

func TestParseGoroutineHeader(t *testing.T) {
	tests := []struct {
		header    string
		wantID    string
		wantState string
	}{
		{"goroutine 1 [running]:", "1", "running"},
		{"goroutine 42 [chan receive, 5 minutes]:", "42", "chan receive"},
		{"goroutine 100 [IO wait]:", "100", "IO wait"},
		{"goroutine 7 [semacquire]:", "7", "semacquire"},
		{"not a goroutine", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			id, state := parseGoroutineHeader(tt.header)
			if id != tt.wantID || state != tt.wantState {
				t.Errorf("parseGoroutineHeader(%q) = (%q, %q), want (%q, %q)", tt.header, id, state, tt.wantID, tt.wantState)
			}
		})
	}
}

func TestGoroutineSingleflight(t *testing.T) {
	resp, err := goroutinesFull()
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	stacks, ok := data["stacks"].(string)
	if !ok || !strings.Contains(stacks, "goroutine") {
		t.Error("expected goroutine stacks in output")
	}
}

func TestShowSystemGoroutines_Wiring(t *testing.T) {
	resp, err := handleShowSystemGoroutines(nil, []string{"summary"})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	if _, exists := data["total"]; !exists {
		t.Error("missing total field")
	}
	if _, exists := data["by-state"]; !exists {
		t.Error("missing by-state field")
	}
}

func TestGoroutinesBufferGrowth(t *testing.T) {
	resp, err := goroutinesFiltered(false)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	total, ok := data["total"].(int)
	if !ok || total < 1 {
		t.Errorf("total = %v, expected at least 1 goroutine", data["total"])
	}
}

func TestGoroutinesBlockedMode(t *testing.T) {
	resp, err := handleShowSystemGoroutines(nil, []string{goroutineModeBlocked})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	if data["mode"] != goroutineModeBlocked {
		t.Errorf("mode = %v, want blocked", data["mode"])
	}
	if _, exists := data["blocked-count"]; !exists {
		t.Error("missing blocked-count field")
	}
}
