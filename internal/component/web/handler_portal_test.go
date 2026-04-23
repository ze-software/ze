package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractPortalKey(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/portal/", ""},
		{"/portal/gokrazy", "gokrazy"},
		{"/portal/gokrazy/", "gokrazy"},
		{"/portal/health", "health"},
		{"/portal/l2tp", "l2tp"},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", tt.path, http.NoBody)
		got := extractPortalKey(r)
		if got != tt.want {
			t.Errorf("extractPortalKey(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestHandlePortal_UnknownService(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	handler := HandlePortal(renderer)

	r := httptest.NewRequest("GET", "/portal/nonexistent", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlePortal_RegisteredService(t *testing.T) {
	RegisterPortalService(PortalService{
		Key: "test-svc", Title: "Test Service", Path: "/test-svc/",
	})
	defer func() {
		portalMu.Lock()
		filtered := portalServices[:0]
		for _, s := range portalServices {
			if s.Key != "test-svc" {
				filtered = append(filtered, s)
			}
		}
		portalServices = filtered
		portalMu.Unlock()
	}()

	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	handler := HandlePortal(renderer)

	r := httptest.NewRequest("GET", "/portal/test-svc", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, `<iframe`) {
		t.Error("response missing iframe element")
	}
	if !strings.Contains(body, `src="/test-svc/"`) {
		t.Errorf("iframe missing correct src, got:\n%s", body)
	}
	if !strings.Contains(body, `class="portal-frame"`) {
		t.Error("iframe missing portal-frame class")
	}
}

func TestHandlePortal_EmptyKey_Redirects(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	handler := HandlePortal(renderer)

	r := httptest.NewRequest("GET", "/portal/", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusFound)
	}
}

func TestPortalServices_Isolation(t *testing.T) {
	before := len(PortalServices())
	RegisterPortalService(PortalService{Key: "iso-test", Title: "Isolation", Path: "/iso/"})
	after := PortalServices()
	if len(after) != before+1 {
		t.Errorf("expected %d services, got %d", before+1, len(after))
	}

	// Mutating the returned slice must not affect the registry.
	after[0].Key = "mutated"
	current := PortalServices()
	for _, s := range current {
		if s.Key == "mutated" {
			t.Error("PortalServices returned a reference instead of a copy")
		}
	}

	// Cleanup.
	portalMu.Lock()
	filtered := portalServices[:0]
	for _, s := range portalServices {
		if s.Key != "iso-test" {
			filtered = append(filtered, s)
		}
	}
	portalServices = filtered
	portalMu.Unlock()
}
