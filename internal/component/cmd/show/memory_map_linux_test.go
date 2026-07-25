//go:build linux

package show

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestParseProcSelfStatus(t *testing.T) {
	status, err := parseProcSelfStatus()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"vm-rss-kb", "vm-size-kb", "threads"} {
		if _, exists := status[key]; !exists {
			t.Errorf("missing key %q", key)
		}
	}
	if threads, ok := status["threads"].(int); !ok || threads < 1 {
		t.Errorf("threads = %v, expected >= 1", status["threads"])
	}
}

func TestShowSystemMemoryMap_Wiring(t *testing.T) {
	resp, err := handleShowSystemMemoryMap(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	if _, exists := data["vm-rss-kb"]; !exists {
		t.Error("missing vm-rss-kb field")
	}
}
