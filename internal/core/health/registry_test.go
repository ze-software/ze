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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", rec.Code)
	}
}

// TestRegisterWhenOmitsARowThatDoesNotApply proves a gated check that does not
// apply produces no row and does not run, so it cannot raise or lower the
// overall status. Method: one gated-off check that would answer down, one
// gated-on check, one ungated check.
func TestRegisterWhenOmitsARowThatDoesNotApply(t *testing.T) {
	r := &Registry{}
	ran := false
	r.RegisterWhen("vpp", func() bool { return false }, func() (Status, string) {
		ran = true
		return StatusDown, "not connected"
	})
	r.RegisterWhen("l2tp", func() bool { return true }, func() (Status, string) { return StatusDegraded, "no kernel" })
	r.Register("bgp", func() (Status, string) { return StatusHealthy, "" })

	report := r.Check()
	if ran {
		t.Error("a check whose gate answered false ran")
	}
	if len(report.Components) != 2 {
		t.Fatalf("components = %d, want 2 (the gated-off row omitted): %+v", len(report.Components), report.Components)
	}
	for _, c := range report.Components {
		if c.Name == "vpp" {
			t.Errorf("gated-off row present: %+v", c)
		}
	}
	if report.Status != StatusDegraded {
		t.Errorf("status = %s, want degraded from the gated-on row", report.Status)
	}
}

// TestRegisterWhenGatePanicIsDown proves a gate that panics reports its row
// down rather than vanishing. Method: a gate that panics.
func TestRegisterWhenGatePanicIsDown(t *testing.T) {
	r := &Registry{}
	r.RegisterWhen("vpp", func() bool { panic("gate") }, func() (Status, string) { return StatusHealthy, "" })

	report := r.Check()
	if len(report.Components) != 1 {
		t.Fatalf("components = %d, want 1", len(report.Components))
	}
	if report.Components[0].Status != StatusDown {
		t.Errorf("status = %s, want down", report.Components[0].Status)
	}
}
