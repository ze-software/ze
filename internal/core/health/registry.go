// Design: plan/spec-diag-0-umbrella.md -- component health aggregation (diag-6)

package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Status represents a component's health state.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
)

// CheckFunc is a health check that returns the component's status and
// an optional reason string. Implementations must be safe for concurrent
// use and must not block for more than 1 second.
type CheckFunc func() (Status, string)

// ComponentHealth is the result of one component's health check.
type ComponentHealth struct {
	Name      string `json:"name"`
	Status    Status `json:"status"`
	Reason    string `json:"reason,omitempty"`
	CheckedAt string `json:"checked-at"`
}

// Report is the aggregated health of all registered components.
type Report struct {
	Status     Status            `json:"status"`
	Components []ComponentHealth `json:"components"`
	CheckedAt  string            `json:"checked-at"`
}

type registration struct {
	name  string
	check CheckFunc
}

// Registry holds named health check functions. Safe for concurrent use.
type Registry struct {
	mu     sync.RWMutex
	checks []registration
}

// DefaultRegistry is the process-wide health registry.
var DefaultRegistry = &Registry{}

// Register adds a named health check. Components call this at startup.
func (r *Registry) Register(name string, check CheckFunc) {
	r.mu.Lock()
	r.checks = append(r.checks, registration{name: name, check: check})
	r.mu.Unlock()
}

// Check runs all registered health checks and returns an aggregated report.
func (r *Registry) Check() Report {
	r.mu.RLock()
	checks := make([]registration, len(r.checks))
	copy(checks, r.checks)
	r.mu.RUnlock()

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	components := make([]ComponentHealth, 0, len(checks))
	overall := StatusHealthy

	for _, reg := range checks {
		status, reason := invokeCheck(reg)
		components = append(components, ComponentHealth{
			Name:      reg.name,
			Status:    status,
			Reason:    reason,
			CheckedAt: now,
		})
		if status == StatusDown {
			overall = StatusDown
		} else if status == StatusDegraded && overall != StatusDown {
			overall = StatusDegraded
		}
	}

	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	return Report{
		Status:     overall,
		Components: components,
		CheckedAt:  now,
	}
}

// Handler returns an http.Handler that serves /health as JSON.
// Returns HTTP 200 for healthy/degraded, 503 for down.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		report := r.Check()
		w.Header().Set("Content-Type", "application/json")
		if report.Status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	})
}

func invokeCheck(reg registration) (status Status, reason string) {
	defer func() {
		if r := recover(); r != nil {
			status = StatusDown
			reason = fmt.Sprintf("check panicked: %v", r)
		}
	}()
	return reg.check()
}

// Register adds a health check to the default registry.
func Register(name string, check CheckFunc) {
	DefaultRegistry.Register(name, check)
}

// Check runs all checks on the default registry.
func Check() Report {
	return DefaultRegistry.Check()
}
