// Design: docs/architecture/config/transaction-protocol.md -- a plugin that applies nothing says so

package sdk

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestConfigApplyNothingDeclarationAccepts proves AC-11: a plugin that
// declares it applies nothing accepts a reload's config-apply, opens the
// rollback gate like any accepted apply, and logs at debug a line naming the
// plugin, the changed root and the reason. Method: register the declaration,
// call the config-apply callback directly with a bgp diff, read the answer,
// the gate and the captured log. Not parallel: it swaps the default logger.
func TestConfigApplyNothingDeclarationAccepts(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	p, _ := newTestPair(t)
	p.DeclareAppliesNothing("reads only capability leaves")

	answer, err := p.callbacks[callbackConfigApply](json.RawMessage(`{"sections":[{"root":"bgp","added":"{}"}]}`))
	if err != nil {
		t.Fatalf("config-apply: %v", err)
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(answer, &status); err != nil {
		t.Fatalf("decode answer %s: %v", answer, err)
	}
	if status.Status != rpc.StatusOK {
		t.Fatalf("config-apply answered %s, want accepted", answer)
	}
	if !p.configTx.rollbackOwed() {
		t.Fatal("an accepted apply did not open the rollback gate")
	}
	line := logged.String()
	for _, want := range []string{"applies nothing", "plugin=" + p.name, "roots=[bgp]", "reads only capability leaves"} {
		if !strings.Contains(line, want) {
			t.Fatalf("debug log %q lacks %q", line, want)
		}
	}
}

// TestDeclareAppliesNothingNeedsAReason proves the declaration refuses an
// empty reason, so no plugin applies nothing without saying why. Method:
// recover the panic.
func TestDeclareAppliesNothingNeedsAReason(t *testing.T) {
	p, _ := newTestPair(t)
	defer func() {
		if recover() == nil {
			t.Fatal("an empty reason was accepted")
		}
	}()
	p.DeclareAppliesNothing("")
}
