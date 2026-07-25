//go:build linux

package show

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestCategorizeFDTarget(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"socket:[12345]", "socket"},
		{"pipe:[67890]", "pipe"},
		{"anon_inode:[eventpoll]", "anon_inode"},
		{"/dev/null", "device"},
		{"/etc/ze/config.conf", "file"},
		{"(unknown)", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := categorizeFDTarget(tt.target)
			if got != tt.want {
				t.Errorf("categorizeFDTarget(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestParseProcSelfLimits(t *testing.T) {
	soft, hard := readFDLimits()
	if soft <= 0 {
		t.Errorf("soft limit = %d, expected > 0", soft)
	}
	if hard <= 0 {
		t.Errorf("hard limit = %d, expected > 0", hard)
	}
	if hard < soft {
		t.Errorf("hard limit %d < soft limit %d", hard, soft)
	}
}

func TestShowSystemFD_Wiring(t *testing.T) {
	resp, err := handleShowSystemFD(nil, []string{"summary"})
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
	if _, exists := data["by-type"]; !exists {
		t.Error("missing by-type field")
	}
	if _, exists := data["soft-limit"]; !exists {
		t.Error("missing soft-limit field")
	}
}
