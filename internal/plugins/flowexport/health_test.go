package flowexport

import (
	"testing"

	"github.com/ze-software/ze/internal/core/health"
)

func TestFlowExportHealthNotConfigured(t *testing.T) {
	// No active exporter: healthy "not configured".
	prev := activeExporter.Swap(nil)
	defer activeExporter.Store(prev)

	status, msg := checkFlowExportHealth()
	if status != health.StatusHealthy {
		t.Errorf("status = %q, want healthy", status)
	}
	if msg != "not configured" {
		t.Errorf("msg = %q, want \"not configured\"", msg)
	}
}

func TestFlowExportHealthNoCollectors(t *testing.T) {
	exp := &exporter{}
	prev := activeExporter.Swap(exp)
	defer activeExporter.Store(prev)

	status, msg := checkFlowExportHealth()
	if status != health.StatusHealthy {
		t.Errorf("status = %q, want healthy", status)
	}
	if msg != "no collectors" {
		t.Errorf("msg = %q, want \"no collectors\"", msg)
	}
}
