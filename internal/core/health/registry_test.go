package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistryHealthy(t *testing.T) {
	r := &Registry{}
	r.Register("bgp", func() (Status, string) { return StatusHealthy, "" })
	r.Register("l2tp", func() (Status, string) { return StatusHealthy, "" })

	report := r.Check()
	if report.Status != StatusHealthy {
		t.Errorf("status = %s, want healthy", report.Status)
	}
	if len(report.Components) != 2 {
		t.Fatalf("components = %d, want 2", len(report.Components))
	}
}

func TestRegistryDegraded(t *testing.T) {
	r := &Registry{}
	r.Register("bgp", func() (Status, string) { return StatusHealthy, "" })
	r.Register("l2tp", func() (Status, string) { return StatusDegraded, "high echo loss" })

	report := r.Check()
	if report.Status != StatusDegraded {
		t.Errorf("status = %s, want degraded", report.Status)
	}
}

func TestRegistryDown(t *testing.T) {
	r := &Registry{}
	r.Register("bgp", func() (Status, string) { return StatusHealthy, "" })
	r.Register("vpp", func() (Status, string) { return StatusDown, "not connected" })

	report := r.Check()
	if report.Status != StatusDown {
		t.Errorf("status = %s, want down", report.Status)
	}
}

func TestRegistryDownOverridesDegraded(t *testing.T) {
	r := &Registry{}
	r.Register("bgp", func() (Status, string) { return StatusDegraded, "flapping" })
	r.Register("vpp", func() (Status, string) { return StatusDown, "not connected" })

	report := r.Check()
	if report.Status != StatusDown {
		t.Errorf("status = %s, want down (down overrides degraded)", report.Status)
	}
}

func TestRegistryEmpty(t *testing.T) {
	r := &Registry{}
	report := r.Check()
	if report.Status != StatusHealthy {
		t.Errorf("status = %s, want healthy (no checks = healthy)", report.Status)
	}
	if len(report.Components) != 0 {
		t.Errorf("components = %d, want 0", len(report.Components))
	}
}

func TestRegistrySorted(t *testing.T) {
	r := &Registry{}
	r.Register("vpp", func() (Status, string) { return StatusHealthy, "" })
	r.Register("bgp", func() (Status, string) { return StatusHealthy, "" })
	r.Register("l2tp", func() (Status, string) { return StatusHealthy, "" })

	report := r.Check()
	if report.Components[0].Name != "bgp" {
		t.Errorf("first = %s, want bgp", report.Components[0].Name)
	}
	if report.Components[2].Name != "vpp" {
		t.Errorf("last = %s, want vpp", report.Components[2].Name)
	}
}

func TestHandler200(t *testing.T) {
	r := &Registry{}
	r.Register("bgp", func() (Status, string) { return StatusHealthy, "" })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	var report Report
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report.Status != StatusHealthy {
		t.Errorf("status = %s, want healthy", report.Status)
	}
}

func TestHandler503(t *testing.T) {
	r := &Registry{}
	r.Register("vpp", func() (Status, string) { return StatusDown, "not connected" })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", rec.Code)
	}
}
