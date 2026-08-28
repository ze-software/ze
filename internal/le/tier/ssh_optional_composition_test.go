package tier

import (
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// TestSSHFeatureGateFeedsTierAudit pins the composition declaration the native
// tier audit loads. The audit's integration coverage runs the complete check;
// this test only proves that its disableable map includes SSH rather than
// duplicating that expensive scan.
func TestSSHFeatureGateFeedsTierAudit(t *testing.T) {
	root, rootErr := lepath.Root()
	if rootErr != nil {
		t.Fatalf("resolve repository root: %v", rootErr)
	}
	gates, err := loadFeatureGates(root)
	if err != nil {
		t.Fatalf("load %s: %v", FeatureGatesManifest, err)
	}
	if tag := gates["internal/component/ssh"]; tag != "ze_ssh" {
		t.Fatalf("%s must map internal/component/ssh to ze_ssh so the native tier audit checks always-on imports; got %q",
			FeatureGatesManifest, tag)
	}
}
