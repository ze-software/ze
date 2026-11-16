// Design: third_party/vpp-patches/README.md -- the registered generation entry.
// VALIDATES: the actual action table reaches generation and preserves its
// structured answer, with dependencies replaced only inside a child process.
package buildvpp

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

const apiActionFixture = "ZE_VPP_API_ACTION_FIXTURE"

func TestVPPAPIGenerateActionDispatches(t *testing.T) {
	if os.Getenv(apiActionFixture) == "entry" {
		called := false
		newAPIGeneration = func(ctx context.Context, root string) (APIReport, error) {
			called = true
			if ctx.Done() == nil {
				t.Fatal("generation has no signal root")
			}
			if root == "" {
				t.Fatal("generation has no checkout")
			}
			return APIReport{Generator: "fixture-generator", Files: []string{"fixture-output"}}, nil
		}
		payload, code := Answer([]string{"update"})
		if code != 0 {
			t.Fatalf("registered generate action exited %d", code)
		}
		if !called {
			t.Fatal("the declared generate action did not reach its implementation")
		}
		report, ok := payload.(APIReport)
		if !ok {
			t.Fatalf("registered generate action returned %T", payload)
		}
		if report.Generator != "fixture-generator" {
			t.Fatalf("generator identity lost: %#v", report)
		}
		if len(report.Files) != 1 {
			t.Fatalf("published outputs lost: %#v", report)
		}
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, self, "-test.run=^TestVPPAPIGenerateActionDispatches$")
	command.Env = append(os.Environ(), apiActionFixture+"=entry")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generate entrypoint fixture: %v\n%s", err, output)
	}
}

func TestVPPAPIGenerateRefusesOutputArguments(t *testing.T) {
	if _, code := Answer([]string{"update", "output", "/unowned"}); code == 0 {
		t.Fatal("generation accepted an arbitrary output directory")
	}
}
