// VALIDATES: every declared gate is native and every verification stage resolves through le.
// PREVENTS: the composition reporting full parity while a requested action is absent.
package le

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/deployment"
	"github.com/ze-software/ze/internal/le/functional"
	"github.com/ze-software/ze/internal/le/integration"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/parity"
	"github.com/ze-software/ze/internal/le/verify"
)

var formerForkGates = []string{
	"ze-deployment-docker-l2tp-ppp-test",
	"ze-deployment-docker-pppoe-accel-test",
	"ze-doc-wiring-check",
	"ze-functional-docker-exec-check",
	"ze-functional-docker-exec-selftest",
	"ze-interop-ipsec-test",
	"ze-interop-test",
	"ze-stress-bird-test",
}

var requestedNativeGates = []string{
	"ze-host-build", "ze-installer-build-amd64", "ze-installer-build-arm64",
	"ze-platform-vet-darwin", "ze-platform-vet-freebsd", "ze-verify-worktree",
	"ze-htmx-upgrade-check", "ze-htmx-upgrade-report", "ze-journal-report",
	"ze-scratch-links-ensure", "ze-scratch-migrate", "ze-site-facts-check",
	"ze-site-facts-update", "ze-spec-citation-check", "ze-terminal-demo-binaries-build-ze",
	"ze-terminal-demo-binaries-build-ze-test", "ze-terminal-demo-check-all",
	"ze-terminal-demo-release-check-all", "ze-terminal-demo-render-all",
	"ze-terminal-demo-validation-check-all", "ze-chaos-cli-unit-test", "ze-chaos-lint",
	"ze-chaos-unit-test", "ze-test-health-check", "ze-test-health-record",
	"ze-test-health-update", "ze-test-weakened-check", "ze-test-weakened-selftest",
	"ze-unit-bgp-test", "ze-unit-cli-test", "ze-unit-config-test", "ze-unit-core-test",
	"ze-unit-plugins-test", "ze-deployment-docker-l2tp-ppp-test",
	"ze-deployment-docker-pppoe-accel-test", "ze-doc-wiring-check",
	"ze-functional-docker-exec-check", "ze-functional-docker-exec-selftest",
	"ze-interop-ipsec-test", "ze-interop-test", "ze-stress-bird-test",
}

var processStarters = [...]string{
	"gaterun.Run(",
	"gaterun.Stream(",
	"exec.Command(",
	"exec.CommandContext(",
}

const sessionScratchHelper = "session-scratch.sh"

func census(t *testing.T) parity.Census {
	t.Helper()
	payload, _ := parity.Answer(nil)
	counted, ok := payload.(parity.Census)
	if !ok {
		t.Fatalf("le parity answered %T rather than a census", payload)
	}
	return counted
}

func TestCensusReportsNoUnportedForkedUnknownOrUnwiredGate(t *testing.T) {
	counted := census(t)
	if counted.Unported != 0 || len(counted.UnportedGates) != 0 {
		t.Errorf("unported = %d, gates = %v", counted.Unported, counted.UnportedGates)
	}
	if counted.Forked != 0 || len(counted.ForkedGates) != 0 {
		t.Errorf("forked = %d, gates = %v", counted.Forked, counted.ForkedGates)
	}
	if len(counted.UnknownClaims) != 0 {
		t.Errorf("unknown claims = %v", counted.UnknownClaims)
	}
	if len(counted.UnwiredClaims) != 0 {
		t.Errorf("unwired claims = %v", counted.UnwiredClaims)
	}
	if counted.Converted != counted.Gates {
		t.Errorf("converted = %d, declared gates = %d", counted.Converted, counted.Gates)
	}
}

func TestAllRequestedGatesAreServedNatively(t *testing.T) {
	if len(requestedNativeGates) != 41 {
		t.Fatalf("requested gate population = %d, want 41", len(requestedNativeGates))
	}
	counted := census(t)
	for _, gate := range requestedNativeGates {
		if slices.Contains(counted.UnportedGates, gate) {
			t.Errorf("%s is still unported", gate)
		}
		if slices.Contains(counted.ForkedGates, gate) {
			t.Errorf("%s is still forked", gate)
		}
	}
}

func TestEveryFormerForkActionPublishesNoForks(t *testing.T) {
	rows := []leaction.List{deployment.Actions(), functional.Actions(), integration.Actions()}
	counted := census(t)
	for _, gate := range formerForkGates {
		if slices.Contains(counted.ForkedGates, gate) {
			t.Errorf("%s remains in the parity fork population", gate)
		}
	}
	found := map[string]bool{}
	for _, listing := range rows {
		for _, row := range listing.Actions {
			if !slices.Contains(formerForkGates, row.Gate) {
				continue
			}
			found[row.Gate] = true
			if len(row.Forks) != 0 {
				t.Errorf("%s still publishes a fork: %v", row.Gate, row.Forks)
			}
		}
	}
	for _, gate := range formerForkGates {
		if gate != "ze-doc-wiring-check" && !found[gate] {
			t.Errorf("former fork action %s is absent", gate)
		}
	}
}

func namesAScript(starter string, call *ast.CallExpr) bool {
	for _, literal := range scriptLiterals(call) {
		if literal == "make" && strings.HasPrefix(starter, "gaterun.") {
			return true
		}
		if (strings.HasSuffix(literal, ".py") || strings.HasSuffix(literal, ".sh")) &&
			!strings.Contains(literal, sessionScratchHelper) {
			return true
		}
	}
	return false
}

func scriptLiterals(call *ast.CallExpr) []string {
	var scripts []string
	for _, argument := range call.Args {
		ast.Inspect(argument, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err == nil {
				scripts = append(scripts, value)
			}
			return true
		})
	}
	return scripts
}

func processStartsRepositoryScript(body string) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "", body, 0)
	if err != nil {
		return false, err
	}
	starts := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		starter := qualifier.Name + "." + selector.Sel.Name + "("
		if slices.Contains(processStarters[:], starter) && namesAScript(starter, call) {
			starts = true
			return false
		}
		return true
	})
	return starts, nil
}

func TestProcessStartBindsScriptLiteralToTheSameCall(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"direct script", `package p; func f() { exec.CommandContext(ctx, "python3", "scripts/x.py") }`, true},
		{"gaterun make driver", `package p; func f() { gaterun.Run(ctx, []string{"make", "ze-check"}) }`, true},
		{"unrelated script literal", `package p; const producer = "scripts/x.py"; func f() { exec.Command("git", "status") }`, false},
		{"native make command", `package p; func f() { exec.CommandContext(ctx, "make", argv...) }`, false},
		{"session scratch helper", `package p; func f() { exec.Command("bash", "scripts/dev/session-scratch.sh") }`, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := processStartsRepositoryScript(test.body)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if got != test.want {
				t.Errorf("processStartsRepositoryScript = %v, want %v", got, test.want)
			}
		})
	}
}

func TestEveryAreaThatStartsAProcessDeclaresWhatItStarts(t *testing.T) {
	claims := map[string]bool{}
	starts := map[string]bool{}
	declares := map[string]bool{}

	walkLeGo(t, func(rel, body string) {
		if strings.HasSuffix(rel, "_test.go") {
			return
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		if pkg == "internal/le/gaterun" {
			return
		}
		if strings.Contains(body, "parity.Claim(") || strings.Contains(body, "parity.ClaimForked(") {
			claims[pkg] = true
		}
		started, err := processStartsRepositoryScript(body)
		if err != nil {
			t.Errorf("parse %s: %v", rel, err)
			return
		}
		if started {
			starts[pkg] = true
		}
		if strings.Contains(body, "Forks:") || strings.Contains(body, "func Forks()") {
			declares[pkg] = true
		}
	})

	if len(claims) == 0 {
		t.Fatal("the production scan found no parity-claiming package")
	}

	for pkg := range starts {
		if claims[pkg] && !declares[pkg] {
			t.Errorf("%s starts a repository script but declares no Forks argv", pkg)
		}
	}
}

func TestEveryVerifyStageToolIsOwnedAndRegistered(t *testing.T) {
	stages := verify.FullStages()
	if len(stages) != 44 {
		t.Fatalf("verify stage population = %d, want 44", len(stages))
	}
	for _, stage := range stages {
		tool := stage.Identity.Command
		if !leroot.Owns(tool) {
			t.Errorf("verify stage %s names unowned tool %s", stage.Identity.Gate, tool)
			continue
		}
		words := [2]string{"le", tool}
		handler, trailing := registry.LookupLocalData(words[:])
		if handler == nil || len(trailing) != 0 {
			t.Errorf("verify stage %s tool %s has no exact local-data handler", stage.Identity.Gate, tool)
		}
	}
}
