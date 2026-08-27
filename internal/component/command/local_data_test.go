package command

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// TestServeLocalRefusesInvalidChainBeforeCallingProducer keeps validation in
// front of both source work and payload encoding. The handler deliberately
// returns a value encoding/json cannot marshal, so either operation happening
// before validation would replace the named pipe refusal.
func TestServeLocalRefusesInvalidChainBeforeCallingProducer(t *testing.T) {
	called := 0
	const path = "show test local validation order"
	if err := registry.RegisterLocalData(path, func(_ []string) (any, int) {
		called++
		return func() {}, 0
	}, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
		t.Fatalf("register local data handler: %v", err)
	}

	answer, code, served := ServeLocal(path+" | raw | json", "")
	if !served {
		t.Fatal("registered local data command was not served")
	}
	if code != 1 {
		t.Errorf("refused chain exit code = %d, want 1", code)
	}
	if called != 0 {
		t.Errorf("refused chain called its producer %d times, want none", called)
	}
	if !IsPipeError(answer) || !strings.Contains(answer, "raw") || !strings.Contains(answer, "json") {
		t.Errorf("refusal = %q, want a named raw/json pipe error", answer)
	}
}
