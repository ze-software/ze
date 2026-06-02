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

func TestParallelRunnerAddTestWithNickRegistersStableNick(t *testing.T) {
	r := NewParallelRunner[string](NewColorsWithOverride(false))
	rec := r.AddTestWithNick("parse-fixture", "X", "fixture", func(context.Context, string) (bool, error) {
		return true, nil
	})
	if rec.Nick != "X" {
		t.Fatalf("record nick = %q, want X", rec.Nick)
	}
	rec.State = StateFail
	if got := r.display.tests.GetByNick("X"); got != rec {
		t.Fatalf("stable nick lookup returned %p, want %p", got, rec)
	}
	if got := r.display.tests.FailedNicks(); len(got) != 1 || got[0] != "X" {
		t.Fatalf("failed nicks = %v, want [X]", got)
	}
}
