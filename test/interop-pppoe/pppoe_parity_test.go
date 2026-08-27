package interop_pppoe_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	pppoeleaf "github.com/ze-software/ze/internal/le/interoplab/pppoe"
)

var _ func(context.Context, pppoeleaf.Options) interoplab.SuiteReport = pppoeleaf.Run
var _ func(context.Context, string, pppoeleaf.Options) interoplab.SuiteReport = pppoeleaf.RunAt

func TestNativeScenarioPopulationMatchesPythonProducer(t *testing.T) {
	// VALIDATES: The native selector exposes every Python-produced PPPoE scenario
	// under the same exact name and lexical order.
	// PREVENTS: A sampled port that silently omits one checker directory.
	root := repositoryRoot(t)
	producerScenarios := producerScenarioNames(t, filepath.Join(root, "test", "interop-pppoe", "scenarios"))
	nativeScenarios := nativeCheckerNames(t, filepath.Join(root, "internal/le", "interoplab", "pppoe", "pppoe.go"))
	if !reflect.DeepEqual(nativeScenarios, producerScenarios) {
		t.Fatalf("native checkers = %v, producer scenarios = %v", nativeScenarios, producerScenarios)
	}

	runner := readFile(t, filepath.Join(root, "test", "interop-pppoe", "run.py"))
	if !strings.Contains(runner, "for scenario_name in sorted(os.listdir(scenarios_dir)):") {
		t.Fatal("Python producer no longer declares lexical scenario order")
	}
	report := pppoeleaf.RunAt(context.Background(), root, pppoeleaf.Options{
		Scenario: "not-a-pppoe-scenario",
		NoBuild:  true,
		Suffix:   "parity",
	})
	if report.Code != 1 || report.SetupError != "no scenario matching 'not-a-pppoe-scenario' found" {
		t.Fatalf("native missing-selector result = %#v", report)
	}
}

func TestNativeImagesConfigsAndPPPDArgvMatchPythonProducer(t *testing.T) {
	// VALIDATES: The native suite consumes the producer's three Dockerfiles and
	// scenario configs, and preserves pppd's exact ordered negotiation argv.
	// PREVENTS: Peer substitution, copied config drift, or a weakened auth dial.
	root := repositoryRoot(t)
	pythonRunner := readFile(t, filepath.Join(root, "test", "interop-pppoe", "run.py"))
	pythonLab := readFile(t, filepath.Join(root, "test", "interop-pppoe", "lab.py"))
	goSuitePath := filepath.Join(root, "internal/le", "interoplab", "pppoe", "pppoe.go")
	goSuite := readFile(t, goSuitePath)
	goScenarios := readFile(t, filepath.Join(root, "internal/le", "interoplab", "pppoe", "scenarios.go"))
	goCheckerPath := filepath.Join(root, "internal/le", "interoplab", "pppoe", "check_ac.go")

	constants := nativeStringConstants(t, goSuitePath)
	for name, want := range map[string]string{
		"zeImageTag":     "ze-pppoe-interop",
		"accelImageTag":  "ze-pppoe-accel",
		"clientImageTag": "ze-pppoe-client",
	} {
		if constants[name] != want {
			t.Fatalf("%s = %q, want %q", name, constants[name], want)
		}
		if !strings.Contains(pythonRunner, `"`+want+`"`) {
			t.Fatalf("Python image producer does not build %q", want)
		}
	}
	for _, dockerfile := range []string{"Dockerfile.ze", "Dockerfile.accel", "Dockerfile.client"} {
		if !strings.Contains(pythonRunner, `"`+dockerfile+`"`) ||
			!strings.Contains(goSuite, `"`+dockerfile+`"`) {
			t.Fatalf("Dockerfile %s is not shared by both producers", dockerfile)
		}
	}
	if strings.Count(pythonRunner, "timeout=600") != 2 ||
		strings.Count(goSuite, "Timeout:    10 * time.Minute") != 2 ||
		!strings.Contains(pythonRunner, "timeout=900") ||
		!strings.Contains(goSuite, "Timeout:    15 * time.Minute") {
		t.Fatal("native image build timeouts do not match the Python producer")
	}

	for _, contract := range []string{
		"accel-ppp.conf",
		"/etc/accel-ppp.conf",
		"chap-secrets",
		"/etc/accel-ppp/chap-secrets",
		"ze.conf",
		"/etc/ze/ze.conf",
	} {
		nativeContains := strings.Contains(goScenarios, contract)
		if !nativeContains {
			for _, value := range constants {
				if value == contract {
					nativeContains = true
					break
				}
			}
		}
		if !strings.Contains(pythonLab, contract) || !nativeContains {
			t.Fatalf("config/mount contract %q is not present in both producers", contract)
		}
	}

	pythonArgv := pythonPPPDArguments(t, pythonLab)
	if len(pythonArgv) != 29 ||
		pythonArgv[0] != "pppd" ||
		pythonArgv[5] != "<username>" ||
		pythonArgv[7] != "<password>" ||
		pythonArgv[28] != "debug" {
		t.Fatalf("Python pppd argv extractor returned a non-discriminating population: %v", pythonArgv)
	}
	goArgv := goPPPDArguments(t, goCheckerPath, constants)
	if !reflect.DeepEqual(goArgv, pythonArgv) {
		t.Fatalf("native pppd argv = %v\nPython pppd argv = %v", goArgv, pythonArgv)
	}
	for _, exact := range []string{
		`args.extend(["rp_pppoe_service", service_name])`,
		`arguments = append(arguments, "rp_pppoe_service", service)`,
		`"docker",`,
		`ExecDetached(`,
	} {
		if !strings.Contains(pythonLab+readFile(t, goCheckerPath), exact) {
			t.Fatalf("detached/service argv contract %q disappeared", exact)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(working, "..", ".."))
}

func producerScenarioNames(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(directory, entry.Name(), "check.py")); err != nil {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func nativeCheckerNames(t *testing.T, path string) []string {
	t.Helper()
	parsed := parseGo(t, path)
	names := make([]string, 0)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "checkers" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			pair, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			literal, ok := pair.Key.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			name, err := strconv.Unquote(literal.Value)
			if err == nil {
				names = append(names, name)
			}
			return true
		})
	}
	sort.Strings(names)
	return names
}

func nativeStringConstants(t *testing.T, path string) map[string]string {
	t.Helper()
	parsed := parseGo(t, path)
	constants := make(map[string]string)
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, specification := range general.Specs {
			value, ok := specification.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			literal, ok := value.Values[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			text, err := strconv.Unquote(literal.Value)
			if err == nil {
				constants[value.Names[0].Name] = text
			}
		}
	}
	return constants
}

func pythonPPPDArguments(t *testing.T, source string) []string {
	t.Helper()
	start := strings.Index(source, "\ndef pppd_dial(")
	if start < 0 {
		t.Fatal("cannot find Python pppd_dial function")
	}
	endRelative := strings.Index(source[start:], "\n\ndef pppd_log(")
	if endRelative < 0 {
		t.Fatal("cannot find end of Python pppd_dial function")
	}
	function := source[start : start+endRelative]
	blockPattern := regexp.MustCompile(`(?s)\n {4}args = \[\n(.*?)\n {4}\]`)
	block := blockPattern.FindStringSubmatch(function)
	if len(block) != 2 {
		t.Fatal("cannot find Python pppd argv list inside pppd_dial")
	}
	tokenPattern := regexp.MustCompile(`"([^"]*)"|\b(username|password)\b`)
	matches := tokenPattern.FindAllStringSubmatch(block[1], -1)
	arguments := make([]string, 0, len(matches))
	for _, match := range matches {
		if match[1] != "" {
			arguments = append(arguments, match[1])
			continue
		}
		arguments = append(arguments, "<"+match[2]+">")
	}
	return arguments
}

func goPPPDArguments(t *testing.T, path string, constants map[string]string) []string {
	t.Helper()
	parsed := parseGo(t, path)
	arguments := make([]string, 0)
	ast.Inspect(parsed, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		name, ok := assignment.Lhs[0].(*ast.Ident)
		if !ok || name.Name != "arguments" {
			return true
		}
		literal, ok := assignment.Rhs[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, element := range literal.Elts {
			switch value := element.(type) {
			case *ast.BasicLit:
				text, err := strconv.Unquote(value.Value)
				if err == nil {
					arguments = append(arguments, text)
				}
			case *ast.Ident:
				if constant, found := constants[value.Name]; found {
					arguments = append(arguments, constant)
					continue
				}
				arguments = append(arguments, "<"+value.Name+">")
			}
		}
		return false
	})
	return arguments
}

func parseGo(t *testing.T, path string) *ast.File {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
