// Design: docs/architecture/testing/ci-workflows.md -- workflow bootstrap regression coverage.
// Related: contract_test.go -- development tools execute compiled fixtures.
package le

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDevelopmentToolGoRunIgnoresSourceData distinguishes descriptions,
// matcher inputs and opaque runtime values from statically proven compiler
// execution. An opaque Go verb is unproven, not certified safe.
func TestDevelopmentToolGoRunIgnoresSourceData(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{
			name: "comment describing the regression",
			source: `package fixture
// MUTATION: Bootstrap with a direct go run before the launcher places cache/.
func fixture() {}
`,
		},
		{
			name: "workflow matcher data",
			source: `package fixture
import "strings"
const generator = "go run ./internal/le/go/stdpatch/goroot"
func fixture(run string) bool { return strings.Contains(run, generator) }
`,
		},
		{
			name: "direct argv stored as data",
			source: `package fixture
var forbidden = []string{"go", "run", "./tool"}
`,
		},
		{
			name: "shell prints command data",
			source: `package fixture
import "os/exec"
func fixture() { exec.Command("sh", "-c", "printf '%s\\n' 'go run ./tool'") }
`,
		},
		{
			name: "shell comment",
			source: `package fixture
import "os/exec"
func fixture() { exec.Command("bash", "-c", "# go run ./tool\ntrue") }
`,
		},
		{
			name: "subtest named go",
			source: `package fixture
import "testing"
func fixture(t *testing.T) { t.Run("go", func(t *testing.T) {}) }
`,
		},
		{
			name: "command describes eval",
			source: `package fixture
import "os/exec"
func fixture() { exec.Command("bash", "-c", "command -v eval") }
`,
		},
		{
			name: "shell body loaded by another producer",
			source: `package fixture
import "os/exec"
func fixture(body string) { exec.Command("bash", "-c", body) }
`,
		},
		{
			name:   "unknown go arguments",
			source: `package fixture; import "os/exec"; func fixture(args []string) { exec.Command("go", args...) }`,
		},
		{
			name:   "subcommand constant shadowed by parameter",
			source: `package fixture; import "os/exec"; const subcommand = "version"; func fixture(subcommand string) { exec.Command("go", subcommand, "./tool") }`,
		},
		{
			name:   "dynamic shell redirection target",
			source: `package fixture; import "os/exec"; func fixture(path string) { exec.Command("sh", "-c", ": > " + path) }`,
		},
		{
			name:   "dynamic shell command prefix",
			source: `package fixture; import "os/exec"; func fixture(prefix string) { exec.Command("sh", "-c", prefix + "; printf x") }`,
		},
		{
			name:   "dynamic background command and redirection",
			source: `package fixture; import "os/exec"; func fixture(command, path string) { exec.Command("sh", "-c", command + " & echo $! > " + path) }`,
		},
		{
			name: "compiled test process",
			source: `package fixture
import "os"
import "os/exec"
func fixture() {
 binary, err := os.Executable()
 if err != nil { panic(err) }
 exec.Command(binary, "-test.run=^TestFixture$", "--", "run", "./tool")
}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := developmentToolGoRun(tc.source); err != nil {
				t.Fatalf("source data or compiled fixture was treated as compiler execution: %v", err)
			}
		})
	}
}

// TestDevelopmentToolGoRunAcceptsBootstrapRegressionSources checks the actual
// producers that exposed the false positives, without exempting their paths.
func TestDevelopmentToolGoRunAcceptsBootstrapRegressionSources(t *testing.T) {
	root := checkoutRoot(t)
	for _, path := range []string{
		"internal/le/repo/codeql/bootstrap_cache_test.go",
		"internal/le/workflowcheck/stdpatch_cache_link_test.go",
	} {
		t.Run(path, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			if err := developmentToolGoRun(string(body)); err != nil {
				t.Fatalf("bootstrap regression source was treated as compiler execution: %v", err)
			}
		})
	}
}

// TestDevelopmentToolGoRunRefusesCompilerArgv keeps the original refusal for
// actual process argv, including import aliases and executable paths.
func TestDevelopmentToolGoRunRefusesCompilerArgv(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{"command", `package fixture; import "os/exec"; func fixture() { exec.Command("go", "run", "./tool") }`},
		{"command context", `package fixture; import "context"; import "os/exec"; func fixture() { exec.CommandContext(context.Background(), "go", "run", "./tool") }`},
		{"absolute compiler", `package fixture; import "os/exec"; func fixture() { exec.Command("/opt/go/bin/go", "run", "./tool") }`},
		{"import alias", `package fixture; import process "os/exec"; func fixture() { process.Command("go", "run", "./tool") }`},
		{"constant compiler", `package fixture; import "os/exec"; const compiler = "go"; func fixture() { exec.Command(compiler, "run", "./tool") }`},
		{"constant run subcommand", `package fixture; import "os/exec"; const subcommand = "run"; func fixture() { exec.Command("go", subcommand, "./tool") }`},
		{"compiler option", `package fixture; import "os/exec"; func fixture() { exec.Command("go", "-C", "tools", "run", "./tool") }`},
		{"environment wrapper", `package fixture; import "os/exec"; func fixture() { exec.Command("env", "CGO_ENABLED=0", "go", "run", "./tool") }`},
		{"command construction", `package fixture; import "os/exec"; func fixture() { _ = exec.Cmd{Path: "go", Args: []string{"go", "run", "./tool"}} }`},
		{"constructor alias", `package fixture; import "os/exec"; func fixture() { run := exec.Command; run("go", "run", "./tool") }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := developmentToolGoRun(tc.source); err == nil {
				t.Fatal("compiler execution was accepted")
			}
		})
	}
}

// TestDevelopmentToolGoRunRefusesShellExecution checks literal shell command
// positions through the shared launcher grammar, including these quoting,
// control-flow and substitution forms. It does not interpret arbitrary shell.
func TestDevelopmentToolGoRunRefusesShellExecution(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{"shell command", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "go run ./tool") }`},
		{"quoted compiler", `package fixture; import "os/exec"; func fixture() { exec.Command("bash", "-c", "'go' 'run' ./tool") }`},
		{"command sequence", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "true; go run ./tool") }`},
		{"conditional command", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "if true; then go run ./tool; fi") }`},
		{"command substitution", `package fixture; import "os/exec"; func fixture() { exec.Command("bash", "-c", "printf '%s' \"$(go run ./tool)\"") }`},
		{"exec builtin", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "exec go run ./tool") }`},
		{"command builtin", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "command go run ./tool") }`},
		{"environment assignment", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "CGO_ENABLED=0 go run ./tool") }`},
		{"eval builtin", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "eval 'go run ./tool'") }`},
		{"environment unset operand", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "env -u GOOS go run ./tool") }`},
		{"eval dynamic target", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "eval \"go run $TOOL\"") }`},
		{"exec display name", `package fixture; import "os/exec"; func fixture() { exec.Command("bash", "-c", "exec -a display go run ./tool") }`},
		{"environment directory operand", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "env -C tools go run ./tool") }`},
		{"environment split string", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "env -S 'go run ./tool'") }`},
		{"builtin exec", `package fixture; import "os/exec"; func fixture() { exec.Command("bash", "-c", "builtin exec go run ./tool") }`},
		{"builtin eval", `package fixture; import "os/exec"; func fixture() { exec.Command("bash", "-c", "builtin eval 'go run ./tool'") }`},
		{"command eval", `package fixture; import "os/exec"; func fixture() { exec.Command("bash", "-c", "command eval 'go run ./tool'") }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := developmentToolGoRun(tc.source); err == nil {
				t.Fatal("shell compiler execution was accepted")
			}
		})
	}
}

// TestDevelopmentToolGoRunRefusesUnresolvedCompilerIntent refuses explicit
// run intent even when its executable or target cannot be read. The contract
// covers static process argv and literal shell bodies; it does not claim to
// prove arbitrary Go dataflow or the contents of a shell body loaded at runtime.
func TestDevelopmentToolGoRunRefusesUnresolvedCompilerIntent(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{"unknown executable with run argv", `package fixture; import "os/exec"; func fixture(binary string) { exec.Command(binary, "run", "./tool") }`},
		{"compiler constant shadowed by parameter", `package fixture; import "os/exec"; const compiler = "echo"; func fixture(compiler string) { exec.Command(compiler, "run", "./tool") }`},
		{"expanded shell executable", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "$GO run ./tool") }`},
		{"malformed shell", `package fixture; import "os/exec"; func fixture() { exec.Command("sh", "-c", "go 'run ./tool") }`},
		{"partial dynamic shell body", `package fixture; import "os/exec"; func fixture(target string) { exec.Command("sh", "-c", "go run " + target) }`},
		{"dynamic executable fragments cannot become static echo", `package fixture; import "os/exec"; func fixture(part string) { exec.Command("sh", "-c", "ec" + part + "ho run ./tool") }`},
		{"malformed Go", `package fixture; func fixture( {`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := developmentToolGoRun(tc.source); err == nil {
				t.Fatal("unresolved compiler execution was accepted")
			}
		})
	}
}
