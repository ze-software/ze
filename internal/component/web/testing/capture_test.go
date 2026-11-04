// Design: docs/architecture/testing/runner-architecture.md -- settled browser captures.
package webtesting

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBrowserScreenshotSettles proves a capture waits for the render predicate
// to become true, and refuses capture when that predicate never settles.
func TestBrowserScreenshotSettles(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "settles", true: "deadline"}[pending], func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "commands")
			polls := filepath.Join(dir, "polls")
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$CAPTURE_LOG"
case "$1" in
  eval)
    case "$2" in
      *getAnimations*requestAnimationFrame*|*requestAnimationFrame*getAnimations*)
        echo poll >> "$CAPTURE_POLLS"
        if [ "$CAPTURE_PENDING" = yes ]; then echo false
        elif [ "$(wc -l < "$CAPTURE_POLLS")" -lt 2 ]; then echo false
        else echo true; fi ;;
      *) echo true ;;
    esac ;;
  screenshot) touch "$2" ;;
esac
`
			if err := os.WriteFile(filepath.Join(dir, "agent-browser"), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CAPTURE_LOG", log)
			t.Setenv("CAPTURE_POLLS", polls)
			if pending {
				t.Setenv("CAPTURE_PENDING", "yes")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 750*time.Millisecond)
			defer cancel()
			browser := newBrowser("http://owned.invalid")
			browser.ctx = ctx
			capture := filepath.Join(dir, "capture.png")
			err := browser.Screenshot(capture)
			commands, readErr := os.ReadFile(log)
			if readErr != nil {
				t.Fatal(readErr)
			}
			_, statErr := os.Stat(capture)
			if pending {
				if err == nil || !os.IsNotExist(statErr) {
					t.Fatalf("unsettled screenshot: error=%v, capture=%v, commands=%s", err, statErr, commands)
				}
				return
			}
			if err != nil || statErr != nil {
				t.Fatalf("settled screenshot: error=%v, capture=%v", err, statErr)
			}
			observations, err := os.ReadFile(polls)
			if err != nil {
				t.Fatalf("capture never observed render settling: %v; commands=%s", err, commands)
			}
			if strings.Count(string(observations), "poll") < 2 {
				t.Fatalf("captured before render settled: %s", commands)
			}
			if !strings.Contains(string(commands), "fonts") {
				t.Fatalf("capture did not observe font loading: %s", commands)
			}
		})
	}
}
