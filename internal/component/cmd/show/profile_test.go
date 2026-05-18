package show

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func TestShowSystemProfile_Wiring(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, []string{profileTypeHeap})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("expected StatusDone, got %v", resp.Status)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map response")
	}
	if data["type"] != profileTypeHeap {
		t.Errorf("expected type=heap, got %v", data["type"])
	}
	if _, exists := data["data"]; !exists {
		t.Error("missing data field")
	}
}

func TestProfileCPUMutex(t *testing.T) {
	cpuProfileMu.Lock()

	resp, err := handleShowSystemProfile(nil, []string{profileTypeCPU})
	if err != nil {
		cpuProfileMu.Unlock()
		t.Fatal(err)
	}

	cpuProfileMu.Unlock()

	if resp.Status != plugin.StatusError {
		t.Errorf("expected StatusError when mutex held, got %v", resp.Status)
	}
}

func TestProfileCPUDurationBoundary(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, []string{profileTypeCPU, "duration", "61s"})
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected map response")
	}
	dur, ok := data["duration"].(string)
	if !ok {
		t.Fatal("expected string duration")
	}
	if dur != "10s" {
		t.Errorf("duration = %q (should use default 10s when >60s is rejected)", dur)
	}
}

func TestProfileUnknownArgIgnored(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, []string{"invalid-type"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusDone {
		t.Errorf("expected StatusDone (unknown args ignored, defaults to heap), got %v", resp.Status)
	}
}
