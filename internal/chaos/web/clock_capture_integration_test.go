// Design: docs/architecture/chaos-web-dashboard.md -- deterministic producer-clock capture.
//go:build integration

package web

import (
	"bytes"
	"context"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

// TestChaosCaptureUsesControlledProducerClock proves the actual rendered capture
// boundary, not only golden normalization. Two fresh Chromium views of one run
// clock produce identical pixels; advancing that producer clock changes the
// visible age and capture without dropping events or freezing browser execution.
// MUTATION: A wall-clock read in layout or sidebar events breaks the exact age;
// independent per-render clock samples or unordered peer output break the capture.
func TestChaosCaptureUsesControlledProducerClock(t *testing.T) {
	c := newConsumerClock()
	d, mux := clockConsumerDashboard(t, c)
	base := c.at
	d.state.StartTime = base
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: base.Add(5 * time.Second)})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: base.Add(5 * time.Second), ChaosAction: "tcp-disconnect"})
	server := httptest.NewServer(mux)
	defer server.Close()
	// Diagnostic retention attributes unequal actual captures without changing
	// the journey or comparisons. Remove after the Review Gate has accepted the
	// pixel attribution and the repaired equal-input/negative-control proof.
	var captures []struct {
		name string
		png  []byte
	}
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		root := sessionpath.DefaultScratchRoot()
		if root == "" {
			t.Error("cannot retain capture diagnostics without an owned session scratch root")
			return
		}
		directory, err := os.MkdirTemp(root, "chaos-clock-capture-")
		if err != nil {
			t.Errorf("create owned capture diagnostic directory: %v", err)
			return
		}
		for _, captured := range captures {
			path := filepath.Join(directory, captured.name+".png")
			if err := os.WriteFile(path, captured.png, 0o400); err != nil {
				t.Errorf("retain capture diagnostic %s: %v", path, err)
				continue
			}
			t.Logf("retained actual capture diagnostic: %s", path)
		}
	})
	capture := func(name, age string, elapsed time.Duration) []byte {
		t.Helper()
		c.reset(base.Add(elapsed))
		path := filepath.Join(t.TempDir(), name+".png")
		journey, err := webtesting.ParseWBFile("option=viewport:width=1280:height=900\naction=open:path=/\nexpect=html:contains=" + age + "\naction=screenshot:file=" + path + "\n")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		result := webtesting.RunWBCase(ctx, journey, server.URL, "")
		if !result.Passed {
			t.Fatalf("%s actual capture: %s", name, result.Error)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		captures = append(captures, struct {
			name string
			png  []byte
		}{name: name, png: data})
		configuration, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s is not an actual PNG: %v", name, err)
		}
		if configuration.Width != 1280 {
			t.Errorf("%s width = %d, want authored viewport 1280", name, configuration.Width)
		}
		return data
	}
	first := capture("first", ">15s</span>", 20*time.Second)
	second := capture("same-input", ">15s</span>", 20*time.Second)
	if !bytes.Equal(first, second) {
		t.Error("same run-clock input produced different actual Chromium capture bytes")
	}
	advanced := capture("clock-advanced", ">35s</span>", 40*time.Second)
	if bytes.Equal(first, advanced) {
		t.Error("advancing producer time changed neither rendered age nor actual capture")
	}
}
