package runner

import (
	"context"
	"errors"
	"testing"
)

func TestParallelRunnerFailureLinesAppearWithoutVerboseWhenVerifyMode(t *testing.T) {
	t.Setenv("ZE_VERIFY_MODE", "1")
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetLabel("parse")
	r.SetQuiet(true)
	called := false
	r.SetOnFail(func(_ string, err error) {
		called = true
		if err == nil || err.Error() != "broken" {
			t.Fatalf("unexpected callback error: %v", err)
		}
	})
	r.AddTest("broken-test", "fixture", func(context.Context, string) (bool, error) {
		return false, errors.New("broken")
	})
	if r.Run(context.Background()) {
		t.Fatalf("expected runner failure")
	}
	if !called {
		t.Fatalf("verify mode did not emit failure callback without verbose")
	}
}
