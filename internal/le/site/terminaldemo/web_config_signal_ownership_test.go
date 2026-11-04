// Design: docs/contributing/terminal-demos.md -- signal-canceled journey startup.
package siteterminaldemo

import (
	"bytes"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestWebConfigJourneySignalOwnsStartup delivers SIGTERM only after the actual
// journey's init child rendezvouses. A long authored deadline cannot hide a root
// context that startup ignores, and the returned entry must have joined its child.
func TestWebConfigJourneySignalOwnsStartup(t *testing.T) {
	held := make(chan os.Signal, 1)
	signal.Notify(held, syscall.SIGTERM)
	defer signal.Stop(held)
	root := webJourneyFixture(t, "init", "option=timeout:value=90s\naction=wait:ms=1\n")
	result := make(chan string, 1)
	go func() {
		var output, diagnostic bytes.Buffer
		RuntimeMain([]string{"validate", "web-config"}, &output, &diagnostic)
		result <- diagnostic.String()
	}()
	pid := webJourneyChildPID(t, root, "stalled")
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case diagnostic := <-result:
		if !strings.Contains(diagnostic, syscall.SIGTERM.String()+" signal received") {
			t.Errorf("lost startup cancellation cause: %s", diagnostic)
		}
	case <-time.After(2500 * time.Millisecond):
		t.Error("SIGTERM did not interrupt actual journey startup")
		<-result
	}
	webJourneyAssertJoined(t, pid)
}
