//go:build linux

package set

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestHandleSetSystemFD_Max(t *testing.T) {
	resp, err := handleSetSystemFD(nil, []string{"max"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Errorf("status = %v, want done", resp.Status)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", resp.Data)
	}
	current, _ := data["current"].(uint64)
	hard, _ := data["hard-limit"].(uint64)
	if current != hard {
		t.Errorf("current = %d, want %d (hard limit)", current, hard)
	}
}
