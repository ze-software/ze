// Design: docs/architecture/config/system-update.md -- unit tests for update checker

package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/report"
)

func TestUpdateCheckConfigParse(t *testing.T) {
	t.Run("url and default interval", func(t *testing.T) {
		tree := config.NewTree()
		sys := tree.GetOrCreateContainer("system")
		uc := sys.GetOrCreateContainer("update-check")
		uc.Set("url", "https://example.com/version.json")

		sc := ExtractSystemConfig(tree)
		if sc.UpdateCheckURL != "https://example.com/version.json" {
			t.Errorf("UpdateCheckURL = %q, want %q", sc.UpdateCheckURL, "https://example.com/version.json")
		}
		if sc.UpdateCheckInterval != 86400 {
			t.Errorf("UpdateCheckInterval = %d, want %d", sc.UpdateCheckInterval, 86400)
		}
	})

	t.Run("url and custom interval", func(t *testing.T) {
		tree := config.NewTree()
		sys := tree.GetOrCreateContainer("system")
		uc := sys.GetOrCreateContainer("update-check")
		uc.Set("url", "https://example.com/v.json")
		uc.Set("interval", "3600")

		sc := ExtractSystemConfig(tree)
		if sc.UpdateCheckURL != "https://example.com/v.json" {
			t.Errorf("UpdateCheckURL = %q, want %q", sc.UpdateCheckURL, "https://example.com/v.json")
		}
		if sc.UpdateCheckInterval != 3600 {
			t.Errorf("UpdateCheckInterval = %d, want %d", sc.UpdateCheckInterval, 3600)
		}
	})

	t.Run("no update-check block", func(t *testing.T) {
		tree := config.NewTree()
		sys := tree.GetOrCreateContainer("system")
		sys.Set("host", "router1")

		sc := ExtractSystemConfig(tree)
		if sc.UpdateCheckURL != "" {
			t.Errorf("UpdateCheckURL = %q, want empty", sc.UpdateCheckURL)
		}
		if sc.UpdateCheckInterval != 0 {
			t.Errorf("UpdateCheckInterval = %d, want 0", sc.UpdateCheckInterval)
		}
	})
}

func writeVersionJSON(w http.ResponseWriter, ver string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(versionManifest{Version: ver})
}

func TestUpdateCheckFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeVersionJSON(w, "26.06.01")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	ver, err := uc.fetchVersion(context.Background())
	if err != nil {
		t.Fatalf("fetchVersion() error = %v", err)
	}
	if ver != "26.06.01" {
		t.Errorf("fetchVersion() = %q, want %q", ver, "26.06.01")
	}
}

func TestUpdateCheckFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprint(w, "server error"); err != nil {
			panic(err)
		}
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	_, err := uc.fetchVersion(context.Background())
	if err == nil {
		t.Fatal("fetchVersion() expected error for 500 response with invalid JSON")
	}
}

func TestUpdateCheckFetchInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := fmt.Fprint(w, "not json at all"); err != nil {
			panic(err)
		}
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	_, err := uc.fetchVersion(context.Background())
	if err == nil {
		t.Fatal("fetchVersion() expected error for invalid JSON")
	}
}

func TestUpdateCheckEvent(t *testing.T) {
	report.ResetForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeVersionJSON(w, "99.99.99")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	uc.running = "26.05.17"
	uc.check(context.Background())

	st := uc.Status()
	if !st.UpdateAvailable {
		t.Error("expected UpdateAvailable = true for version 99.99.99")
	}
	if st.RemoteVersion != "99.99.99" {
		t.Errorf("RemoteVersion = %q, want %q", st.RemoteVersion, "99.99.99")
	}

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Source == "system" && w.Code == "update-available" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected report bus warning with source=system, code=update-available")
	}
}

func TestUpdateCheckNoEvent(t *testing.T) {
	report.ResetForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeVersionJSON(w, "00.00.01")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	uc.running = "26.05.17"
	uc.check(context.Background())

	st := uc.Status()
	if st.UpdateAvailable {
		t.Error("expected UpdateAvailable = false for old version")
	}

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Source == "system" && w.Code == "update-available" {
			t.Error("unexpected update-available warning for old version")
		}
	}
}

func TestUpdateCheckStartStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeVersionJSON(w, "99.99.99")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 1)
	ctx, cancel := context.WithCancel(context.Background())
	uc.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	st := uc.Status()
	if st.LastCheck.IsZero() {
		t.Error("expected at least one check after Start")
	}

	cancel()
	uc.Stop()
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		remote  string
		running string
		want    bool
	}{
		{"26.06.01", "26.05.17", true},
		{"26.05.17", "26.05.17", false},
		{"26.04.01", "26.05.17", false},
		{"dev", "26.05.17", false},
		{"99.99.99", "dev", false},
		{"27.01.01", "26.12.31", true},
	}
	for _, tt := range tests {
		t.Run(tt.remote+"_vs_"+tt.running, func(t *testing.T) {
			if got := isNewer(tt.remote, tt.running); got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.remote, tt.running, got, tt.want)
			}
		})
	}
}
