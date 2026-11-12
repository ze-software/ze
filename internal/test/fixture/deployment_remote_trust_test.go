// Design: docs/guide/operations.md -- standalone CLI with verified remote trust.

package fixture

import (
	"context"
	"strings"
	"testing"
)

func TestDeploymentRemoteTrustFixtureIsRegistered(t *testing.T) {
	driversMu.RLock()
	driver := drivers["ui/deployment-remote-trust"]
	driversMu.RUnlock()
	if driver == nil {
		t.Fatal("the real-binary remote trust fixture is not registered")
	}
	for _, args := range [][]string{nil, {"0"}, {"not-a-port"}, {"65536"}} {
		if err := driver(context.Background(), args); err == nil || !strings.Contains(err.Error(), "runner") {
			t.Fatalf("fixture must reject invalid runner port before starting processes: args=%q err=%v", args, err)
		}
	}
}
