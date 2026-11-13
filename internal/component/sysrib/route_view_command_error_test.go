// Design: docs/architecture/rib/unified-locrib.md -- system routing command ownership.
// VALIDATES: command refusals name a usable command without disturbing valid reads.

package sysrib

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

func TestRouteViewUnknownCommandNamesRecovery(t *testing.T) {
	handler := routeViewCommandHandler(context.Background(), newSysRIB(), nil)
	status, _, err := handler("", "show rib", nil, "")
	if err != nil || status != "done" {
		t.Fatalf("registered read control: status=%q err=%v", status, err)
	}
	status, _, err = handler("", "show unknown-routing-view", nil, "")
	if status != "error" || err == nil {
		t.Fatalf("unknown command was not refused: status=%q err=%v", status, err)
	}
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("command refusal carries no operator recovery: %v", err)
	}
	if !strings.Contains(refusal.Problem(), "show unknown-routing-view") || refusal.Why() == "" {
		t.Fatalf("refusal does not identify the requested command: %v", err)
	}
	if refusal.Fix() != "ze show rib" || !strings.HasSuffix(err.Error(), "fix: ze show rib") {
		t.Fatalf("refusal names no registered recovery command: %v", err)
	}
}
