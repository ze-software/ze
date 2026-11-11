// Design: docs/contributing/dependency-approval.md -- declared modules permit
// full use while circl and undeclared imports remain subject to approval.

package deps

import (
	"os/exec"
	"slices"
	"testing"
)

func TestDependencyPolicyCheckUsesDeclaredModulesAndPreservesCirclApproval(t *testing.T) {
	root := t.TempDir()
	command := exec.CommandContext(t.Context(), "git", "init", "-q")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	writeCheckFile(t, root, "go.mod", "module fixture\n\nrequire (\n\tgolang.org/x/crypto v0.58.0\n\tgithub.com/cloudflare/circl v1.6.5\n)\n")
	writeCheckFile(t, root, VendorModulesRel, "# golang.org/x/crypto v0.58.0\n## explicit\ngolang.org/x/crypto/ssh/knownhosts\n# github.com/cloudflare/circl v1.6.5\n## explicit\ngithub.com/cloudflare/circl/sign/ed448\n")
	writeCheckFile(t, root, LedgerRel, "github.com/cloudflare/circl/sign/ed448\tinternal/approved\tapproved use\n")
	for _, file := range []struct{ path, pkg string }{
		{"internal/fresh/hosts.go", "golang.org/x/crypto/ssh/knownhosts"},
		{"internal/fresh/hosts_test.go", "golang.org/x/crypto/ssh/knownhosts"},
		{"internal/approved/signer.go", "github.com/cloudflare/circl/sign/ed448"},
		{"internal/fresh/signer.go", "github.com/cloudflare/circl/sign/ed448"},
		{"internal/fresh/unknown.go", "example.org/not-declared/pkg"},
	} {
		writeCheckFile(t, root, file.path, "package test\n\nimport _ \""+file.pkg+"\"\n")
	}
	report, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Use{
		{Package: "github.com/cloudflare/circl/sign/ed448", Importer: "internal/fresh", File: "internal/fresh/signer.go"},
		{Package: "example.org/not-declared/pkg", Importer: "internal/fresh", File: "internal/fresh/unknown.go"},
	}
	if !slices.Equal(report.Unapproved, want) {
		t.Fatalf("approval required for wrong imports: got %+v, want %+v", report.Unapproved, want)
	}
	if report.Files != 5 || len(report.Stale) != 0 {
		t.Fatalf("check skipped a source or lost an approved use: %+v", report)
	}
}
