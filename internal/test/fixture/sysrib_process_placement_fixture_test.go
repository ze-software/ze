// Design: docs/architecture/testing/interop.md -- native process placement proof.

package fixture

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestSysribProcessPlacementUsesNativeCensus(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native procfs census requires Linux")
	}
	pid := strconv.Itoa(os.Getpid())
	const subject = "sysrib-placement-unit-control"
	if err := sysribProcessPlacementDriver(t.Context(), []string{pid, subject, "internal"}); err != nil {
		t.Fatalf("real native process with no matching child refused: %v", err)
	}
	if err := sysribProcessPlacementDriver(t.Context(), []string{pid, subject, "forked"}); err == nil || !strings.Contains(err.Error(), "want 1") {
		t.Fatalf("forked placement without a matching child returned %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := sysribProcessPlacementDriver(ctx, []string{pid, subject, "internal"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled native observation returned %v", err)
	}
}

func TestSysribProcessPlacementRefusesInvalidInput(t *testing.T) {
	for _, args := range [][]string{nil, {"1", "rib"}, {"invalid", "rib", "internal"}, {"0", "rib", "internal"}, {"1", "rib", "unknown"}} {
		if err := sysribProcessPlacementDriver(t.Context(), args); err == nil {
			t.Fatalf("accepted invalid observer arguments %q", args)
		}
	}
}
