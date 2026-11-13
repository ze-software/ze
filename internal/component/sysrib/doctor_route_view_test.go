// Design: docs/architecture/doctor-and-health-checks.md -- routing-view runtime health.
package sysrib

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewDoctorRegisteredCallbackUsesAlias(t *testing.T) {
	h := newRouteViewPluginHarness(t, nil)
	// Hold the first actual selected read before it returns a snapshot.
	h.initial = false
	h.start(t)
	h.await(t, h.reading, "initial selected read")
	answer, err := h.engine.CallRPC(h.ctx, "ze-plugin-callback:doctor-check", &rpc.DoctorCheckInput{Name: routeViewDoctorName})
	if err != nil {
		t.Fatal(err)
	}
	var out rpc.DoctorCheckOutput
	if err := json.Unmarshal(answer, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Diagnostics) != 1 {
		t.Fatalf("initialization diagnostics = %v", out.Diagnostics)
	}
	if out.Diagnostics[0].Fix != "ze request plugin restart forwarding-west" {
		t.Fatalf("registered callback fix = %q", out.Diagnostics[0].Fix)
	}
}

func TestRouteViewDoctorRegisteredCallbackHealthy(t *testing.T) {
	h := newRouteViewPluginHarness(t, nil)
	h.start(t)
	h.await(t, h.reading, "synchronized selected read")
	answer, err := h.engine.CallRPC(h.ctx, "ze-plugin-callback:doctor-check", &rpc.DoctorCheckInput{Name: routeViewDoctorName})
	if err != nil {
		t.Fatal(err)
	}
	var out rpc.DoctorCheckOutput
	if err := json.Unmarshal(answer, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Diagnostics) != 0 {
		t.Fatalf("healthy external runtime diagnostics = %v", out.Diagnostics)
	}
}

func TestRouteViewDoctorCodeOwned(t *testing.T) {
	if meta := diagnostic.Lookup(routeViewDoctorCode); meta == nil {
		t.Fatal("system-RIB owner did not register its routing-view diagnostic")
	}
}

func TestRouteViewDoctorDeclaredOnce(t *testing.T) {
	h := newRouteViewPluginHarness(t, nil)
	h.start(t)
	count := 0
	for _, check := range h.registration.DoctorChecks {
		if check.Name != routeViewDoctorName {
			continue
		}
		count++
		if check.Phase != rpc.DoctorPhaseRuntime {
			t.Fatalf("phase = %q", check.Phase)
		}
		if len(check.Codes) != 1 || check.Codes[0] != routeViewDoctorCode {
			t.Fatalf("codes = %v", check.Codes)
		}
	}
	if count != 1 {
		t.Fatalf("routing-view runtime declarations = %d; want exactly one", count)
	}
	for _, reg := range registry.All() {
		if reg.Name != configRootRIB {
			continue
		}
		for _, check := range reg.DoctorChecks {
			if check.Name == routeViewDoctorName {
				t.Fatal("runtime health also registered as a config callback")
			}
		}
	}
}

func TestRouteViewDoctorInitializationAndAlias(t *testing.T) {
	health := newRouteViewHealth(metrics.NopRegistry{}, func() string { return "forwarding-west" })
	findings, err := health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("initial findings = %v", findings)
	}
	if findings[0].Fix != "ze request plugin restart forwarding-west" {
		t.Fatalf("fix = %q", findings[0].Fix)
	}
	if !strings.Contains(findings[0].Message, "initializing") {
		t.Fatalf("message = %q", findings[0].Message)
	}
}

func TestRouteViewDoctorTransitionsAndMeasuredMetrics(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	health := newRouteViewHealth(reg, func() string { return "forwarding-west" })
	check := func(want string) {
		t.Helper()
		findings, err := health.findings(routeViewDoctorName)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			if len(findings) != 0 {
				t.Fatalf("healthy findings = %v", findings)
			}
			return
		}
		if len(findings) != 1 || !strings.Contains(findings[0].Message, want) {
			t.Fatalf("findings = %v; want %q", findings, want)
		}
	}
	health.markSynchronized()
	check("")
	health.beginResync()
	health.setPending(3)
	check("resynchronizing")
	assertRouteViewMetric(t, reg, "ze_rib_view_resync_total 1")
	assertRouteViewMetric(t, reg, "ze_rib_view_pending_prefixes 3")
	assertRouteViewMetric(t, reg, "ze_rib_view_ready 0")
	health.fail(errors.New("fixture-view-lost"))
	check("fixture-view-lost")
	health.beginResync()
	health.markSynchronized()
	check("")
	assertRouteViewMetric(t, reg, "ze_rib_view_resync_total 2")
	assertRouteViewMetric(t, reg, "ze_rib_view_pending_prefixes 0")
	assertRouteViewMetric(t, reg, "ze_rib_view_ready 1")
}

func assertRouteViewMetric(t *testing.T, reg *metrics.PrometheusRegistry, want string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	reg.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	response := recorder.Result()
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n"+want+"\n") {
		t.Fatalf("metric %q missing from %s", want, data)
	}
}
