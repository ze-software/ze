package registry_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/localdatacoverage"
	"github.com/ze-software/ze/internal/test/runner"
)

const (
	functionalRunCommand    = "ze-test local-data-coverage"
	functionalTimeoutOption = "option=timeout:value=45s"
	functionalTimeout       = "45s"
)

func TestEveryLocalDataRegistrationHasAFunctionalCase(t *testing.T) {
	root := repositoryRoot(t)
	registered := productionLocalDataCommands(t, root)
	invocations := localdatacoverage.Evidence()
	commands := make(map[string]bool, len(invocations))
	for _, invocation := range invocations {
		if !strings.Contains(invocation.Command, "| ") {
			t.Errorf("functional command has no real pipe: %q", invocation.Command)
		}
		if commands[invocation.Command] {
			t.Errorf("functional command is executed twice: %q", invocation.Command)
		}
		commands[invocation.Command] = true
	}

	covered, resolutionErrors := exactRegistrationCoverage(invocations, registered)
	for _, err := range resolutionErrors {
		t.Error(err)
	}
	for command, source := range registered {
		if !covered[command] {
			t.Errorf("%s registers %q without runtime evidence", source, command)
		}
	}
}

func TestLocalDataCoverageEvidenceIsNonVacuousAndComplete(t *testing.T) {
	invocations := localdatacoverage.Evidence()
	if len(invocations) != 16 {
		t.Fatalf("executable local-data calls = %d, want 16", len(invocations))
	}
	distinct := make(map[string]bool, len(invocations))
	for _, invocation := range invocations {
		distinct[invocation.Evidence] = true
	}
	if len(distinct) != 15 {
		t.Fatalf("distinct registration evidence = %d, want 15: %v", len(distinct), distinct)
	}
	if localdatacoverage.CompletionMarker != "OK: 15/15 local-data commands and local one-shot save" {
		t.Fatalf("completion marker = %q", localdatacoverage.CompletionMarker)
	}
}

func TestCommittedScenarioLaunchesOnlyTheCompiledHelper(t *testing.T) {
	root := repositoryRoot(t)
	path := filepath.Join(root, "test", "ui", "pipe-local-command.ci")
	content, err := os.ReadFile(path) //nolint:gosec // Repository-owned functional scenario.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := validateCompiledScenario(string(content), localdatacoverage.Evidence()); err != nil {
		t.Fatalf("compiled local-data scenario: %v", err)
	}
}

func TestCompiledScenarioEvidenceCannotBeSpoofedOrWeakened(t *testing.T) {
	valid := compiledScenario(localdatacoverage.Evidence())
	first := "expect=stdout:contains=" + localdatacoverage.Marker("show config dump") + "\n"
	completion := "expect=stdout:contains=" + localdatacoverage.CompletionMarker + "\n"
	for _, testCase := range []struct {
		name string
		text string
	}{
		{name: "missing evidence", text: strings.Replace(valid, first, "", 1)},
		{name: "duplicate evidence", text: strings.Replace(valid, first, first+first, 1)},
		{name: "shorter-prefix evidence", text: strings.Replace(valid, localdatacoverage.Marker("show config history"), localdatacoverage.Marker("show config"), 1)},
		{name: "wrong helper", text: strings.Replace(valid, functionalRunCommand, "ze-test text-plugin", 1)},
		{name: "interpreted payload", text: "tmpfs=payload:terminator=END\nfake\nEND\n" + valid},
		{name: "missing completion", text: strings.Replace(valid, completion, "", 1)},
		{
			name: "completion is not final",
			text: strings.Replace(
				strings.Replace(valid, completion, "", 1),
				first,
				completion+first,
				1,
			),
		},
		{name: "missing successful exit", text: strings.Replace(valid, "expect=exit:code=0\n", "", 1)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if err := validateCompiledScenario(testCase.text, localdatacoverage.Evidence()); err == nil {
				t.Fatal("weakened scenario was accepted")
			}
		})
	}
}

func compiledScenario(invocations []localdatacoverage.Invocation) string {
	var text strings.Builder
	text.WriteString(functionalTimeoutOption + "\n")
	text.WriteString("cmd=foreground:seq=1:exec=" + functionalRunCommand + "\n")
	text.WriteString("expect=exit:code=0\n")
	for _, marker := range expectedScenarioMarkers(invocations) {
		text.WriteString("expect=stdout:contains=" + marker + "\n")
	}
	return text.String()
}

func evidenceMarkers(invocations []localdatacoverage.Invocation) []string {
	set := make(map[string]bool, len(invocations))
	for _, invocation := range invocations {
		set[localdatacoverage.Marker(invocation.Evidence)] = true
	}
	markers := make([]string, 0, len(set))
	for marker := range set {
		markers = append(markers, marker)
	}
	slices.Sort(markers)
	return markers
}

func expectedScenarioMarkers(invocations []localdatacoverage.Invocation) []string {
	markers := evidenceMarkers(invocations)
	return append(markers, localdatacoverage.CompletionMarker)
}

func validateCompiledScenario(content string, invocations []localdatacoverage.Invocation) error {
	if strings.Contains(content, "tmpfs=") {
		return fmt.Errorf("scenario carries an interpreted source payload")
	}
	record, err := compiledScenarioRecord(content)
	if err != nil {
		return err
	}

	options, commands, exits := 0, 0, 0
	markers := make([]string, 0)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case line == functionalTimeoutOption:
			options++
		case line == "cmd=foreground:seq=1:exec="+functionalRunCommand:
			commands++
		case line == "expect=exit:code=0":
			exits++
		case strings.HasPrefix(line, "expect=stdout:contains="):
			markers = append(markers, strings.TrimPrefix(line, "expect=stdout:contains="))
		default:
			return fmt.Errorf("unexpected directive %q", line)
		}
	}
	if options != 1 || commands != 1 || exits != 1 {
		return fmt.Errorf("options=%d commands=%d successful-exits=%d, want one each", options, commands, exits)
	}

	wantMarkers := expectedScenarioMarkers(invocations)
	if !slices.Equal(markers, wantMarkers) {
		return fmt.Errorf("stdout evidence = %q, want exact ordered markers %q", markers, wantMarkers)
	}
	return validateCompiledScenarioRecord(record, wantMarkers)
}

func compiledScenarioRecord(content string) (*runner.Record, error) {
	dir, err := os.MkdirTemp("", "ze-compiled-local-data-")
	if err != nil {
		return nil, fmt.Errorf("create isolated compiled scenario directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module functional.coverage\n"), 0o600); err != nil {
		return nil, fmt.Errorf("materialize isolated functional module: %w", err)
	}
	candidate := filepath.Join(dir, "candidate.ci")
	if err := os.WriteFile(candidate, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("materialize compiled scenario: %w", err)
	}

	discovered := runner.NewEncodingTests(dir)
	if err := discovered.Discover(dir); err != nil {
		return nil, fmt.Errorf("discover compiled scenario with production runner: %w", err)
	}
	records := discovered.Registered()
	if len(records) != 1 {
		return nil, fmt.Errorf("production runner discovered %d compiled scenarios, want exactly 1", len(records))
	}
	record := records[0]
	if record.ParseFailed {
		if record.Error != nil {
			return nil, fmt.Errorf("production runner rejected compiled scenario: %w", record.Error)
		}
		return nil, fmt.Errorf("production runner rejected compiled scenario without an error")
	}
	if record.Error != nil {
		return nil, fmt.Errorf("production runner recorded a compiled scenario error: %w", record.Error)
	}
	return record, nil
}

func validateCompiledScenarioRecord(record *runner.Record, wantMarkers []string) error {
	if record.SkipReason != "" {
		return fmt.Errorf("compiled scenario is skipped: %s", record.SkipReason)
	}
	if got := record.Extra["timeout"]; got != functionalTimeout {
		return fmt.Errorf("production runner timeout is %q, want %q", got, functionalTimeout)
	}
	if len(record.Extra) != 1 {
		return fmt.Errorf("production runner recorded %d runtime options, want only timeout", len(record.Extra))
	}
	if len(record.RunCommands) != 1 {
		return fmt.Errorf("compiled scenario has %d run commands, want exactly 1", len(record.RunCommands))
	}
	runCommand := record.RunCommands[0]
	if runCommand.Mode != "foreground" || runCommand.Seq != 1 || runCommand.Exec != functionalRunCommand {
		return fmt.Errorf("compiled scenario run command = mode %q seq %d exec %q", runCommand.Mode, runCommand.Seq, runCommand.Exec)
	}
	if runCommand.Stdin != "" || runCommand.Name != "" || runCommand.Signal != "" ||
		runCommand.ExitCode != nil || runCommand.Timeout != "" {
		return fmt.Errorf("compiled scenario run command carries extra orchestration")
	}
	if len(record.TmpfsFiles) != 0 {
		return fmt.Errorf("compiled scenario carries tmpfs files")
	}
	if len(record.StdinBlocks) != 0 {
		return fmt.Errorf("compiled scenario has stdin orchestration blocks")
	}
	if len(record.Messages) != 0 || len(record.Expects) != 0 ||
		len(record.HTTPChecks) != 0 || len(record.HTTPWaits) != 0 ||
		len(record.EngineSteps) != 0 || len(record.FileChecks) != 0 {
		return fmt.Errorf("compiled scenario has unrelated runner steps")
	}
	if record.ExpectExitCode == nil || *record.ExpectExitCode != 0 {
		return fmt.Errorf("compiled scenario has no exact file-level successful exit assertion")
	}
	if !slices.Equal(record.ExpectStdoutMatch, wantMarkers) {
		return fmt.Errorf("production runner stdout evidence = %q, want %q", record.ExpectStdoutMatch, wantMarkers)
	}
	if len(record.ExpectStdoutNotMatch) != 0 ||
		len(record.ExpectStdoutRegex) != 0 || len(record.RejectStdoutRegex) != 0 ||
		len(record.ExpectStderrMatch) != 0 || len(record.ExpectStderr) != 0 ||
		len(record.RejectStderr) != 0 || len(record.ExpectSyslog) != 0 ||
		len(record.RejectSyslog) != 0 || record.AwaitStderr != "" ||
		record.AwaitStderrTimeout != "" {
		return fmt.Errorf("compiled scenario has unrelated output expectations")
	}
	return nil
}

func exactRegistrationCoverage(invocations []localdatacoverage.Invocation, registered map[string]string) (map[string]bool, []error) {
	covered := make(map[string]bool, len(registered))
	var errors []error
	for _, invocation := range invocations {
		resolved := longestProductionRegistrationPrefix(invocation.Command, registered)
		if invocation.Evidence == "" || resolved != invocation.Evidence {
			errors = append(errors, fmt.Errorf(
				"functional command %q resolves production registration %q, want exact evidence %q",
				invocation.Command, resolved, invocation.Evidence,
			))
			continue
		}
		covered[resolved] = true
	}
	return covered, errors
}

func longestProductionRegistrationPrefix(command string, registered map[string]string) string {
	longest := ""
	for path := range registered {
		if command != path && !strings.HasPrefix(command, path+" ") {
			continue
		}
		if len(path) > len(longest) {
			longest = path
		}
	}
	return longest
}

func TestNewShorterRegistrationCannotBorrowExistingChildMarker(t *testing.T) {
	const (
		shorter = "show config"
		child   = "show config history"
	)
	registered := map[string]string{
		shorter: "config.go",
		child:   "history.go",
	}
	invocations := []localdatacoverage.Invocation{{
		Command:  child + " pipe-local.conf | json compact",
		Evidence: child,
	}}

	covered, resolutionErrors := exactRegistrationCoverage(invocations, registered)
	if len(resolutionErrors) != 0 {
		t.Fatalf("resolve child evidence: %v", resolutionErrors)
	}
	spoofed := []localdatacoverage.Invocation{{
		Command:  child + " pipe-local.conf | json compact",
		Evidence: shorter,
	}}
	if _, errors := exactRegistrationCoverage(spoofed, registered); len(errors) == 0 {
		t.Fatalf("shorter evidence %q matched child command %q", shorter, spoofed[0].Command)
	}
	if !covered[child] {
		t.Fatalf("child registration %q was not covered", child)
	}
	if covered[shorter] {
		t.Fatalf("new shorter registration %q borrowed child evidence %q", shorter, child)
	}
	wantMarkers := []string{localdatacoverage.Marker(child), localdatacoverage.CompletionMarker}
	if got := expectedScenarioMarkers(invocations); !slices.Equal(got, wantMarkers) {
		t.Fatalf("scenario markers = %q, want exact child marker and completion %q", got, wantMarkers)
	}
}

func TestProductionLocalDataCommandsSkipTestdataAndLERootAdapter(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "cmd", "testdata", "malformed"),
		filepath.Join(root, "internal", "le", "leroot"),
		filepath.Join(root, "pkg"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create fixture directory %s: %v", dir, err)
		}
	}
	livePath := filepath.Join(root, "cmd", "live.go")
	if err := os.WriteFile(livePath, []byte("package cmd\nfunc register() {\n"+
		"registry.MustRegisterLocalData(\"show live | json compact\")\n}\n"), 0o600); err != nil {
		t.Fatalf("write live registration fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "testdata", "malformed", "broken.go"), []byte("package malformed\nfunc {"), 0o600); err != nil {
		t.Fatalf("write malformed testdata fixture: %v", err)
	}
	adapterPath := filepath.Join(root, "internal", "le", "leroot", "leroot.go")
	if err := os.WriteFile(adapterPath, []byte("package leroot\nfunc Register(name string) {\n"+
		"registry.MustRegisterLocalData(CommandPath(name))\n}\n"), 0o600); err != nil {
		t.Fatalf("write leroot adapter fixture: %v", err)
	}
	commands := productionLocalDataCommands(t, root)
	if len(commands) != 1 || commands["show live | json compact"] != filepath.Join("cmd", "live.go") {
		t.Fatalf("production registrations = %v, want only live literal", commands)
	}
}

func TestLERootLocalDataAdapterExclusionIsExact(t *testing.T) {
	commandPath := func(argumentCount int) ast.Expr {
		arguments := make([]ast.Expr, argumentCount)
		for index := range arguments {
			arguments[index] = &ast.Ident{Name: "name"}
		}
		return &ast.CallExpr{
			Fun:  &ast.Ident{Name: "CommandPath"},
			Args: arguments,
		}
	}
	tests := []struct {
		name     string
		path     string
		argument ast.Expr
		want     bool
	}{
		{
			name:     "exact leroot adapter",
			path:     filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: commandPath(1),
			want:     true,
		},
		{
			name:     "same call elsewhere",
			path:     filepath.Join("internal", "component", "other.go"),
			argument: commandPath(1),
		},
		{
			name:     "other dynamic expression in leroot",
			path:     filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: &ast.Ident{Name: "path"},
		},
		{
			name: "different command path argument",
			path: filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: &ast.CallExpr{
				Fun:  &ast.Ident{Name: "CommandPath"},
				Args: []ast.Expr{&ast.Ident{Name: "other"}},
			},
		},
		{
			name:     "different command path arity",
			path:     filepath.Join("internal", "le", "leroot", "leroot.go"),
			argument: commandPath(2),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isLERootLocalDataAdapter(test.path, test.argument); got != test.want {
				t.Fatalf("isLERootLocalDataAdapter() = %t, want %t", got, test.want)
			}
		})
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller did not report this test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func productionLocalDataCommands(t *testing.T, root string) map[string]string {
	t.Helper()
	commands := make(map[string]string)
	for _, sourceRoot := range []string{"cmd", "internal", "pkg"} {
		path := filepath.Join(root, sourceRoot)
		err := filepath.WalkDir(path, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			collectLocalDataRegistrations(t, root, path, commands)
			return nil
		})
		if err != nil {
			t.Fatalf("walk production Go under %s: %v", sourceRoot, err)
		}
	}
	if len(commands) == 0 {
		t.Fatal("derived no production MustRegisterLocalData commands")
	}
	return commands
}

func collectLocalDataRegistrations(t *testing.T, root, path string, commands map[string]string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "MustRegisterLocalData" {
			return true
		}
		if len(call.Args) == 0 {
			t.Errorf("%s has MustRegisterLocalData without a path", path)
			return true
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("make %s relative: %v", path, relErr)
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			if isLERootLocalDataAdapter(relative, call.Args[0]) {
				return true
			}
			t.Errorf("%s has a non-literal MustRegisterLocalData path", path)
			return true
		}
		command, unquoteErr := strconv.Unquote(literal.Value)
		if unquoteErr != nil {
			t.Errorf("%s has invalid path: %v", path, unquoteErr)
			return true
		}
		if previous, exists := commands[command]; exists {
			t.Errorf("%q registered by %s and %s", command, previous, relative)
			return true
		}
		commands[command] = relative
		return true
	})
}

func isLERootLocalDataAdapter(relative string, argument ast.Expr) bool {
	if filepath.ToSlash(relative) != "internal/le/leroot/leroot.go" {
		return false
	}
	call, ok := argument.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	function, ok := call.Fun.(*ast.Ident)
	if !ok || function.Name != "CommandPath" {
		return false
	}
	name, ok := call.Args[0].(*ast.Ident)
	return ok && name.Name == "name"
}
