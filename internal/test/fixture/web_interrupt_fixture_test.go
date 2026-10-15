package fixture

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// TestWebInterruptNeverClosedMessage checks the error the driver gives when
// the listener never refused. Method: the test process stands in for a daemon
// that is still alive, so the message names the open listener, once with no
// context error (Poll used every attempt) and once with an expired context.
func TestWebInterruptNeverClosedMessage(t *testing.T) {
	address := "127.0.0.1:8443"

	err := webInterruptNeverClosed(os.Getpid(), address, time.Now(), nil)
	if err == nil {
		t.Fatal("no error for a listener that never refused")
	}
	message := err.Error()
	if strings.Contains(message, "%!") {
		t.Errorf("message carries a formatting verb with no operand: %q", message)
	}
	for _, want := range []string{address, "dialed it 100 times", "no dial was refused"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q does not contain %q", message, want)
		}
	}

	err = webInterruptNeverClosed(os.Getpid(), address, time.Now(), context.DeadlineExceeded)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap the driver context error", err)
	}
	if strings.Contains(err.Error(), "%!") {
		t.Errorf("message carries a formatting verb with no operand: %q", err.Error())
	}
}
