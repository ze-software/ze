package dockerkernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallUnstartedStepDoesNotClaimChanges runs the actual step consumer
// under a canceled root and checks both its diagnostic and the absent write.
// It invokes no sudo, host installation, boot configuration or reboot.
func TestInstallUnstartedStepDoesNotClaimChanges(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "step-started")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := runInstallStep(ctx, installStep{Why: "write the controlled marker", Argv: []string{
		"sh", "-c", `printf started > "$1"`, "install-progress", marker,
	}})
	if err == nil {
		t.Fatal("a step under a canceled root was accepted")
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the canceled step started: %v", statErr)
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("unstarted step diagnostic does not state that it never started: %v", err)
	}
	if strings.Contains(err.Error(), "partly made") {
		t.Errorf("unstarted step diagnostic claims a partial host change: %v", err)
	}
}
