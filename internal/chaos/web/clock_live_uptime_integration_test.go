//go:build integration

// Design: docs/architecture/chaos-web-dashboard.md -- one producer clock on an open page.
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

// TestChaosOpenPageUsesProducerClock keeps one actual Chromium page open through
// unequal real waits. A fixed producer clock must keep visible uptime and ages
// fixed, including equal capture bytes; advancing only that clock must update
// the same page through existing dashboard polling/streaming and change pixels.
// It never freezes browser time, edits page scripts, or opens a replacement page.
func TestChaosOpenPageUsesProducerClock(t *testing.T) {
	clock := newConsumerClock()
	dashboard, mux := clockConsumerDashboard(t, clock)
	base := clock.at
	dashboard.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: base.Add(5 * time.Second)})
	clock.reset(base.Add(20 * time.Second))
	server := httptest.NewServer(mux)
	defer server.Close()
	root := sessionpath.DefaultScratchRoot()
	if root == "" {
		t.Fatal("live clock capture proof requires an owned session scratch root")
	}
	directory, err := os.MkdirTemp(root, "chaos-live-clock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("retained live producer-clock captures: %s", directory)
			return
		}
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	firstPath := filepath.Join(directory, "first.png")
	secondPath := filepath.Join(directory, "unequal-real-wait.png")
	advancedPath := filepath.Join(directory, "producer-advanced.png")
	journey, err := webtesting.ParseWBFile("option=timeout:value=30s\noption=viewport:width=1280:height=900\n" +
		"action=open:path=/\nexpect=element:id=uptime\n" +
		"action=wait:ms=1700\nexpect=html:contains=>20s</span>\nexpect=html:contains=>15s</span>\n" +
		"action=screenshot:file=" + firstPath + "\n" +
		"action=wait:ms=3300\nexpect=html:contains=>20s</span>\nexpect=html:contains=>15s</span>\n" +
		"action=screenshot:file=" + secondPath + "\n" +
		"expect=html:contains=>40s</span>\nexpect=html:contains=>35s</span>\n" +
		"action=screenshot:file=" + advancedPath + "\n")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	advanced := make(chan error, 1)
	// The already-authored screenshot provides a same-page synchronization
	// boundary. This bounded file observer advances only producer time after
	// the second image exists; it adds no browser request, timer or DOM change.
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Stat(secondPath); err == nil {
				clock.reset(base.Add(40 * time.Second))
				advanced <- nil
				return
			} else if !os.IsNotExist(err) {
				advanced <- err
				return
			}
			select {
			case <-ctx.Done():
				advanced <- context.Cause(ctx)
				return
			case <-ticker.C:
			}
		}
	}()
	result := webtesting.RunWBCase(ctx, journey, server.URL, "")
	cancel()
	advanceErr := <-advanced
	if !result.Passed {
		t.Fatalf("same open page producer-clock proof: %s", result.Error)
	}
	if advanceErr != nil {
		t.Fatalf("advance producer clock after equal-input captures: %v", advanceErr)
	}
	readCapture := func(path string) []byte {
		t.Helper()
		image, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		configuration, err := png.DecodeConfig(bytes.NewReader(image))
		if err != nil {
			t.Fatalf("actual capture %s: %v", path, err)
		}
		if configuration.Width != 1280 || configuration.Height != 900 {
			t.Fatalf("actual capture %s has unexpected dimensions %dx%d", path, configuration.Width, configuration.Height)
		}
		if err := os.Chmod(path, 0o400); err != nil {
			t.Fatal(err)
		}
		t.Logf("actual live producer-clock capture: %s", path)
		return image
	}
	first := readCapture(firstPath)
	second := readCapture(secondPath)
	if !bytes.Equal(first, second) {
		t.Error("one fixed producer clock yielded different captures after unequal real waits")
	}
	changed := readCapture(advancedPath)
	if bytes.Equal(second, changed) {
		t.Error("advancing only producer time changed neither the same page nor its capture")
	}
}
