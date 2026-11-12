package testqemu

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestApplianceOTAPushFailureKeepsSafeProgress exercises the complete OTA
// command owner. A failed host command must retain its last known update phase
// without reproducing arbitrary stderr that can contain updater credentials.
func TestApplianceOTAPushFailureKeepsSafeProgress(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		assertApplianceOTAPushFailureKeepsSafeProgress(t, 0, context.DeadlineExceeded)
	})
	t.Run("nonzero exit", func(t *testing.T) {
		assertApplianceOTAPushFailureKeepsSafeProgress(t, 23, nil)
	})
}

func assertApplianceOTAPushFailureKeepsSafeProgress(t *testing.T, code int, runErr error) {
	t.Helper()
	fixture := newOTAProtocolFixture(t, false)
	original := fixture.installer.ops.Run
	fixture.installer.ops.Run = func(ctx context.Context, spec commandSpec) (commandResult, error) {
		if len(spec.Args) > 1 {
			if spec.Args[1] == "push" {
				return commandResult{Code: code, Stderr: "streaming root image...\nstreaming boot image...\nmarking testboot...\nrebooting...\nhttps://gokrazy:DUMMY-UNSAFE-URL@fixture.invalid/\n"}, runErr
			}
		}
		return original(ctx, spec)
	}
	_, err := fixture.installer.executeApplianceOTA(t.Context())
	if err == nil {
		t.Fatal("failed host push returned no error")
	}
	if runErr != nil {
		if !errors.Is(err, runErr) {
			t.Fatalf("failed host push lost its deadline cause: %v", err)
		}
	}
	if code != 0 {
		if !strings.Contains(err.Error(), "exited 23") {
			t.Fatalf("failed host push lost its exit code: %v", err)
		}
	}
	if !strings.Contains(err.Error(), "rebooting") {
		t.Fatalf("failed host push lost its last known phase: %v", err)
	}
	if strings.Contains(err.Error(), "DUMMY-UNSAFE-URL") {
		t.Fatalf("failed host push reproduced arbitrary credential-bearing stderr: %v", err)
	}
	if strings.Contains(err.Error(), "fixture.invalid") {
		t.Fatalf("failed host push copied unrecognized stderr into its diagnostic: %v", err)
	}
}
