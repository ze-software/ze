package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSSHFeatureGateFeedsDependencyAudit pins the composition declaration that
// dep_audit.py loads. TestEnginePlacement runs the real --check gate over the
// repository, so this test only proves that its disableable map includes SSH
// rather than duplicating the expensive audit process.
func TestSSHFeatureGateFeedsDependencyAudit(t *testing.T) {
	manifest := filepath.Join("..", "..", "feature-gates.txt")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read %s: %v", manifest, err)
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "ze_ssh" && fields[1] == "internal/component/ssh" {
			return
		}
	}
	t.Fatal("feature-gates.txt must declare ze_ssh internal/component/ssh so dep_audit.py checks always-on imports")
}
