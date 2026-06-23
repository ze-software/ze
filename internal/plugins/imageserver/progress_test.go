// VALIDATES: the image server logs transfer start and completion (with
// throughput) for large downloads.
// PREVENTS: regression to the silent multi-GB transfer that left an operator
// watching the install unable to tell whether the image was being served.

package imageserver

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImageServerLogsTransferProgress(t *testing.T) {
	log, buf := captureServerLog()
	prev := loggerPtr.Load()
	loggerPtr.Store(log)
	t.Cleanup(func() { loggerPtr.Store(prev) })

	prevThresh := progressThreshold
	progressThreshold = 1 // any non-empty file counts as "large" here
	t.Cleanup(func() { progressThreshold = prevThresh })

	imageDir := t.TempDir()
	if werr := os.WriteFile(filepath.Join(imageDir, "disk.img"), make([]byte, 4096), 0o600); werr != nil {
		t.Fatal(werr)
	}
	mux := newMux(imageConfig{Enabled: true, ImageDirectory: imageDir}, "", "127.0.0.1")

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	url := "http://" + ln.Addr().String() + "/install/image/disk.img"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil {
		t.Fatalf("drain body: %v", copyErr)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close body: %v", closeErr)
	}

	out := buf.String()
	for _, want := range []string{"imageserver: sending", "imageserver: sent", "disk.img"} {
		if !strings.Contains(out, want) {
			t.Errorf("progress log missing %q:\n%s", want, out)
		}
	}
}
