// Design: docs/functional-tests.md -- child deadlines follow runner contention.

package fixture

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// This reaches the native command caller and its real process-limit diagnostic.
// Cancellation prevents a subprocess or a wall-clock wait. Dropping the scaling
// at either the helper or this caller makes the factor-three case go red.
func TestNativeInstallCommandReportsRunnerScaledLimit(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, factor, want string
	}{
		{"unpublished", "", "command limit 45 seconds"},
		{"single selector", "1", "command limit 45 seconds"},
		{"concurrent selection", "3", "command limit 135 seconds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := env.Get(runner.ParallelFactorEnv)
			if err := env.Set(runner.ParallelFactorEnv, tc.factor); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := env.Set(runner.ParallelFactorEnv, previous); err != nil {
					t.Fatal(err)
				}
			})
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			result := nativeInstallCommand(ctx, time.Now(), nil, "", binary)
			if result.code != 124 || !strings.Contains(result.stderr, tc.want) {
				t.Fatalf("canceled native command = %+v, want exit 124 and %q", result, tc.want)
			}
		})
	}
}
