package show

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestShowSystemProfile_Wiring(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, []string{profileTypeHeap})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("expected StatusDone, got %v", resp.Status)
	}
	data, ok := resp.Data.(plugin.Map)
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
	tests := []struct {
		name string
		dur  string
	}{
		{"too long", "61s"},
		{"too short", "500ms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleShowSystemProfile(nil, []string{profileTypeCPU, "duration", tt.dur})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Status != plugin.StatusError {
				t.Errorf("expected StatusError for duration %s, got %v", tt.dur, resp.Status)
			}
		})
	}
}

func TestProfileCPUDurationInvalid(t *testing.T) {
	resp, err := handleShowSystemProfile(nil, []string{profileTypeCPU, "duration", "notaduration"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("expected StatusError for invalid duration, got %v", resp.Status)
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
