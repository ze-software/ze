package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func init() {
	Register("ui/le-checks-answers", uiDriver(runLEChecksAnswers))
}

type leChecksResult struct {
	stdout string
	stderr string
	code   int
}

type leChecksCase struct {
	name string
	args []string
}

func runLEChecksAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return leChecksFailf("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return leChecksFailf("resolving ZE_REPO_ROOT: %v", err)
	}
	here, err := os.MkdirTemp(filepath.Dir(root), "le-checks-answers-")
	if err != nil {
		return leChecksFailf("creating fixture directory: %v", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	tags, err := uiLEFeatureTags(root)
	if err != nil {
		return err
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return leChecksFailf("finding go: %v", err)
	}
	binary := filepath.Join(here, "le")
	build := exec.CommandContext(ctx, goTool, "build", "-tags", strings.Join(tags, ","), "-o", binary, "./cmd/ze") //nolint:gosec // the fixture chooses the program and its arguments
	build.Dir = root
	build.Env = leChecksEnvironment(map[string]string{envCGOEnabled: "0"})
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		return leChecksFailf("building the full le personality: %v\n%s", err, buildOutput.String())
	}

	runLE := func(env map[string]string, args ...string) (leChecksResult, error) {
		return leChecksRun(ctx, here, env, binary, args...)
	}

	// These are the developer-facing invocations for every ported gate. A clean
	// checkout must pass each one, and these report/check forms must not leak
	// diagnostics to stderr.
	baseline := []leChecksCase{
		{checkConfigClaims, []string{checkConfigClaims}},
		{checkIfaceResolution, []string{checkIfaceResolution}},
		{checkCommandOwnership, []string{checkCommandOwnership}},
		{checkConfigCoercion, []string{checkConfigCoercion, actionCheck}},
		{"config-coercion selftest", []string{checkConfigCoercion, actionSelftest}},
		{checkPortDefaults, []string{checkPortDefaults, actionCheck}},
		{"port-defaults selftest", []string{checkPortDefaults, actionSelftest}},
		{checkCLIGrammar, []string{checkCLIGrammar}},
		{checkFSPersistence, []string{checkFSPersistence, actionCheck}},
		{"fs-persistence selftest", []string{checkFSPersistence, actionSelftest}},
		{checkPluginBoundary, []string{checkPluginBoundary, actionCheck}},
		{"plugin-boundary selftest", []string{checkPluginBoundary, actionSelftest}},
		{"plugin-boundary roots", []string{checkPluginBoundary, "roots"}},
		{"yang-leaf-mentions report", []string{checkYANGLeafMentions, actionReport}},
		{"yang-leaf-mentions selftest", []string{checkYANGLeafMentions, actionSelftest}},
		{"staticcheck-feature-matrix rows", []string{checkStaticcheckFeatureMatrix, fieldRows}},
		{checkDashStdio, []string{checkDashStdio, actionCheck}},
		{"dash-stdio selftest", []string{checkDashStdio, actionSelftest}},
		{checkCIDispatch, []string{checkCIDispatch, actionCheck}},
		{"ci-dispatch selftest", []string{checkCIDispatch, actionSelftest}},
		{"repository-tracked-build selftest", []string{checkRepositoryTrackedBuild, actionSelftest}},
		{"test-sensitivity report", []string{checkTestSensitivity, actionReport}},
		{"test-sensitivity selftest", []string{checkTestSensitivity, actionSelftest}},
	}
	for _, tc := range baseline {
		got, err := runLE(nil, tc.args...)
		if err != nil {
			return err
		}
		if got.code != 0 {
			return leChecksFailf("%s: this checkout does not pass the gate (exit %d): %s%s", tc.name, got.code, got.stdout, got.stderr)
		}
		if got.stderr != "" {
			return leChecksFailf("%s: the command wrote to stderr: %q", tc.name, got.stderr)
		}
	}

	// A config-claims report is one document containing two row sets.
	claimsResult, err := runLE(nil, "config claims", "|", "json")
	if err != nil {
		return err
	}
	if claimsResult.code != 0 {
		return leChecksFailf("`le config claims | json` exited %d: %s", claimsResult.code, claimsResult.stderr)
	}
	claimsValue, err := leChecksJSON(claimsResult.stdout)
	if err != nil {
		return leChecksFailf("`le config claims | json` did not answer JSON: %v\n%s", err, leChecksPrefix(claimsResult.stdout, 400))
	}
	claims, err := leChecksObject(claimsValue, "the claim report")
	if err != nil {
		return err
	}
	if err := leChecksRequireKeys(claims, "the claim report", "roots", "claims", "allowlisted", "findings"); err != nil {
		return err
	}
	roots, err := leChecksInteger(claims["roots"], "claim roots")
	if err != nil {
		return err
	}
	if roots < 25 {
		return leChecksFailf("the gate enumerated %d config roots", roots)
	}
	claimCount, err := leChecksInteger(claims["claims"], "claims")
	if err != nil {
		return err
	}
	if claimCount < 50 {
		return leChecksFailf("the gate enumerated %d claims", claimCount)
	}
	if !leChecksNilOrEmptyRows(claims["findings"]) {
		return leChecksFailf("the gate reported findings: %#v", claims["findings"])
	}

	counted, err := runLE(nil, "config claims", "|", "count")
	if err != nil {
		return err
	}
	if counted.code != 1 {
		return leChecksFailf("`le config claims | count` exited %d, want a refusal", counted.code)
	}
	if !strings.Contains(counted.stderr, "count") {
		return leChecksFailf("the refusal does not name the operator: %q", counted.stderr)
	}
	yaml, err := runLE(nil, "config claims", "|", "yaml")
	if err != nil {
		return err
	}
	if yaml.code != 0 {
		return leChecksFailf("`le config claims | yaml` was refused: %s", yaml.stderr)
	}

	// Gates whose complete answer is a row set accept row operators.
	for _, command := range []string{checkIfaceResolution, checkCommandOwnership} {
		rowsResult, err := runLE(nil, command, "|", "json")
		if err != nil {
			return err
		}
		if rowsResult.code != 0 {
			return leChecksFailf("`le %s | json` exited %d", command, rowsResult.code)
		}
		rowsValue, err := leChecksJSON(rowsResult.stdout)
		if err != nil {
			return leChecksFailf("`le %s | json` answered invalid JSON: %v", command, err)
		}
		if !leChecksNilOrEmptyRows(rowsValue) {
			return leChecksFailf("`le %s | json` answered %q over a clean checkout", command, rowsResult.stdout)
		}
		countResult, err := runLE(nil, command, "|", "count")
		if err != nil {
			return err
		}
		if countResult.code != 0 || strings.TrimSpace(countResult.stdout) != "0" {
			return leChecksFailf("`le %s | count` exited %d and answered %q, want 0", command, countResult.code, countResult.stdout)
		}
	}

	portCases, err := runLE(nil, "port-defaults", "selftest", "|", "json")
	if err != nil {
		return err
	}
	if portCases.code != 0 {
		return leChecksFailf("`le port-defaults selftest | json` exited %d", portCases.code)
	}
	if err := leChecksPassedRows(portCases.stdout, "port-defaults", 8); err != nil {
		return err
	}
	portCount, err := runLE(nil, "port-defaults", "selftest", "|", "count")
	if err != nil {
		return err
	}
	if strings.TrimSpace(portCount.stdout) != "8" {
		return leChecksFailf("`le port-defaults selftest | count` answered %q, want 8", portCount.stdout)
	}

	listing, err := runLE(nil, "config coercion")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return leChecksFailf("`le config coercion` exited %d", listing.code)
	}
	for _, word := range []string{actionCheck, actionSelftest, fieldChecks} {
		if !strings.Contains(listing.stdout, word) {
			return leChecksFailf("the listing does not carry %q:\n%s", word, listing.stdout)
		}
	}

	// The feature scope is handed to a fresh process through a file. The gate
	// must narrow the matrix to four rows and announce that narrowing on stderr.
	scopePath := filepath.Join(here, "scope-tags")
	if leChecksPathWithin(root, scopePath) {
		return leChecksFailf("the scope file %q is inside the checkout", scopePath)
	}
	const scopeContents = "ze_web\nze_ssh\n"
	if err := os.WriteFile(scopePath, []byte(scopeContents), 0o600); err != nil {
		return leChecksFailf("writing the feature scope: %v", err)
	}
	writtenScope, err := os.ReadFile(scopePath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return leChecksFailf("reading back the feature scope: %v", err)
	}
	if string(writtenScope) != scopeContents {
		return leChecksFailf("the feature scope changed on disk: %q", writtenScope)
	}
	scopeEnv := map[string]string{"ZE_VERIFY_SCOPE_TAGS": scopePath}
	scoped, err := runLE(scopeEnv, "staticcheck-feature-matrix", "rows")
	if err != nil {
		return err
	}
	if scoped.code != 0 {
		return leChecksFailf("the scoped matrix exited %d: %s", scoped.code, scoped.stderr)
	}
	if scoped.stdout == "" {
		return leChecksFailf("the scoped matrix wrote no report")
	}
	if !strings.Contains(scoped.stderr, "4 of") {
		return leChecksFailf("the scoped run judged every row, so the variable never reached it: %q", scoped.stderr)
	}
	scopedJSON, err := runLE(scopeEnv, "staticcheck-feature-matrix", "rows", "|", "json")
	if err != nil {
		return err
	}
	if scopedJSON.code != 0 {
		return leChecksFailf("the scoped JSON matrix exited %d: %s", scopedJSON.code, scopedJSON.stderr)
	}
	scopedValue, err := leChecksJSON(scopedJSON.stdout)
	if err != nil {
		return leChecksFailf("the scoped matrix did not answer JSON: %v", err)
	}
	scopedRows, ok := scopedValue.([]any)
	if !ok || len(scopedRows) != 4 {
		return leChecksFailf("the scoped matrix answered %#v, want four rows", scopedValue)
	}

	// cli-grammar is a three-row-set document: row operators are refused while
	// any-shape rendering remains available.
	grammarCount, err := runLE(nil, "cli-grammar", "|", "count")
	if err != nil {
		return err
	}
	if grammarCount.code != 1 {
		return leChecksFailf("`le cli-grammar | count` exited %d, want a refusal", grammarCount.code)
	}
	if !strings.Contains(grammarCount.stderr, "count") {
		return leChecksFailf("the refusal does not name the operator: %q", grammarCount.stderr)
	}
	grammarResult, err := runLE(nil, "cli-grammar", "|", "json")
	if err != nil {
		return err
	}
	if grammarResult.code != 0 {
		return leChecksFailf("`le cli-grammar | json` exited %d", grammarResult.code)
	}
	grammarValue, err := leChecksJSON(grammarResult.stdout)
	if err != nil {
		return leChecksFailf("`le cli-grammar | json` answered invalid JSON: %v", err)
	}
	grammar, err := leChecksObject(grammarValue, "the grammar report")
	if err != nil {
		return err
	}
	if err := leChecksRequireKeys(grammar, "the grammar report", "findings", "flag-in-yang", "demo-launch", "commands-checked", "roots-checked", "demo-scripts-checked", "valid"); err != nil {
		return err
	}
	valid, ok := grammar["valid"].(bool)
	if !ok || !valid {
		return leChecksFailf("this checkout fails the grammar gate: %#v", grammar)
	}
	for _, key := range []string{"commands-checked", "roots-checked", "demo-scripts-checked"} {
		population, err := leChecksInteger(grammar[key], key)
		if err != nil {
			return err
		}
		if population <= 0 {
			return leChecksFailf("the gate read no %s: a population it never walked", key)
		}
	}

	for _, tc := range []leChecksCase{
		{"fs-persistence check", []string{checkFSPersistence, actionCheck}},
		{"plugin-boundary check", []string{checkPluginBoundary, actionCheck}},
		{"dash-stdio check", []string{checkDashStdio, actionCheck}},
	} {
		rowsResult, err := runLE(nil, append(append([]string{}, tc.args...), "|", "json")...)
		if err != nil {
			return err
		}
		if rowsResult.code != 0 {
			return leChecksFailf("`%s | json` exited %d", tc.name, rowsResult.code)
		}
		rowsValue, err := leChecksJSON(rowsResult.stdout)
		if err != nil {
			return leChecksFailf("`%s | json` answered invalid JSON: %v", tc.name, err)
		}
		if !leChecksNilOrEmptyRows(rowsValue) {
			return leChecksFailf("`%s | json` answered %q over a clean checkout", tc.name, rowsResult.stdout)
		}
		countArgs := append(append([]string{}, tc.args...), "|", "count")
		countResult, err := runLE(nil, countArgs...)
		if err != nil {
			return err
		}
		if countResult.code != 0 || strings.TrimSpace(countResult.stdout) != "0" {
			return leChecksFailf("`%s | count` exited %d and answered %q, want 0", tc.name, countResult.code, countResult.stdout)
		}
	}

	dispatchResult, err := runLE(nil, "ci-dispatch", "check", "|", "json")
	if err != nil {
		return err
	}
	if dispatchResult.code != 0 {
		return leChecksFailf("`le ci-dispatch check | json` exited %d", dispatchResult.code)
	}
	dispatchValue, err := leChecksJSON(dispatchResult.stdout)
	if err != nil {
		return leChecksFailf("the dispatch report did not answer JSON: %v", err)
	}
	dispatch, err := leChecksObject(dispatchValue, "the dispatch report")
	if err != nil {
		return err
	}
	if err := leChecksRequireKeys(dispatch, "the dispatch report", "schema-version", "commands-known", "emitters-checked", "pass-through", "findings"); err != nil {
		return err
	}
	commandsKnown, err := leChecksInteger(dispatch["commands-known"], "commands-known")
	if err != nil {
		return err
	}
	emittersChecked, err := leChecksInteger(dispatch["emitters-checked"], "emitters-checked")
	if err != nil {
		return err
	}
	if commandsKnown <= 100 || emittersChecked <= 50 {
		return leChecksFailf("the gate saw %d commands and %d emitters", commandsKnown, emittersChecked)
	}

	leafResult, err := runLE(nil, "yang leaf-mentions", "report", "|", "json")
	if err != nil {
		return err
	}
	if leafResult.code != 0 {
		return leChecksFailf("`le yang leaf-mentions report | json` exited %d", leafResult.code)
	}
	leafValue, err := leChecksJSON(leafResult.stdout)
	if err != nil {
		return leChecksFailf("the leaf report did not answer JSON: %v", err)
	}
	mentions, err := leChecksObject(leafValue, "the leaf report")
	if err != nil {
		return err
	}
	if err := leChecksRequireKeys(mentions, "the leaf report", "modules", "leaves", "findings"); err != nil {
		return err
	}
	modules, err := leChecksInteger(mentions["modules"], "modules")
	if err != nil {
		return err
	}
	leaves, err := leChecksInteger(mentions["leaves"], "leaves")
	if err != nil {
		return err
	}
	if modules <= 0 || leaves <= 0 {
		return leChecksFailf("the report read %d modules and %d leaves", modules, leaves)
	}

	// The sensitivity check's verdict is stdout. Its independently captured
	// stderr is reserved for the ratchet notice.
	sensitivityCheck, err := runLE(nil, "test-sensitivity", "check")
	if err != nil {
		return err
	}
	if sensitivityCheck.code != 0 {
		return leChecksFailf("`le test-sensitivity check` exited %d: %s%s", sensitivityCheck.code, sensitivityCheck.stdout, sensitivityCheck.stderr)
	}
	if strings.TrimSpace(sensitivityCheck.stdout) == "" {
		return leChecksFailf("`le test-sensitivity check` wrote no verdict to stdout")
	}

	trackedResult, err := runLE(nil, "test-sensitivity", "tracked", "|", "json")
	if err != nil {
		return err
	}
	if trackedResult.code != 0 {
		return leChecksFailf("`le test-sensitivity tracked | json` exited %d: %s", trackedResult.code, trackedResult.stderr)
	}
	trackedValue, err := leChecksJSON(trackedResult.stdout)
	if err != nil {
		return leChecksFailf("the tracked-population document did not answer JSON: %v", err)
	}
	tracked, err := leChecksObject(trackedValue, "the tracked-population document")
	if err != nil {
		return err
	}
	filesScanned, err := leChecksInteger(tracked["files-scanned"], "files-scanned")
	if err != nil {
		return err
	}
	testsScanned, err := leChecksInteger(tracked["tests-scanned"], "tests-scanned")
	if err != nil {
		return err
	}
	if filesScanned <= 100 || testsScanned <= 100 {
		return leChecksFailf("the tracked scan read %d files and %d tests", filesScanned, testsScanned)
	}

	trackedBuild, err := runLE(nil, "repository tracked-build", "check")
	if err != nil {
		return err
	}
	if trackedBuild.code != 0 {
		return leChecksFailf("the commit does not compile: %s%s", trackedBuild.stdout, trackedBuild.stderr)
	}
	if strings.TrimSpace(trackedBuild.stdout) == "" {
		return leChecksFailf("the tracked-build check wrote no report")
	}
	if !leChecksHasElapsedColumn(trackedBuild.stdout) {
		return leChecksFailf("the tracked-build report has no elapsed-time column:\n%s", trackedBuild.stdout)
	}

	matrixResult, err := runLE(nil, "repository tracked-build", "matrix", "|", "json")
	if err != nil {
		return err
	}
	if matrixResult.code != 0 {
		return leChecksFailf("the tracked-build matrix exited %d: %s", matrixResult.code, matrixResult.stderr)
	}
	matrixValue, err := leChecksJSON(matrixResult.stdout)
	if err != nil {
		return leChecksFailf("the tracked-build matrix did not answer JSON: %v", err)
	}
	flavors, ok := matrixValue.([]any)
	if !ok || len(flavors) == 0 {
		return leChecksFailf("the tracked-build matrix answered %#v, want flavor rows", matrixValue)
	}
	seenFlavors := make(map[string]struct{}, len(flavors))
	for i, value := range flavors {
		flavor, err := leChecksObject(value, fmt.Sprintf("tracked-build flavor %d", i))
		if err != nil {
			return err
		}
		if err := leChecksRequireKeys(flavor, fmt.Sprintf("tracked-build flavor %d", i), "name", "tags", "anchor-files"); err != nil {
			return err
		}
		name, ok := flavor["name"].(string)
		if !ok || name == "" {
			return leChecksFailf("tracked-build flavor %d has invalid name %#v", i, flavor["name"])
		}
		if _, duplicate := seenFlavors[name]; duplicate {
			return leChecksFailf("the tracked-build matrix repeats flavor %q", name)
		}
		seenFlavors[name] = struct{}{}
		if _, ok := flavor["tags"].([]any); !ok {
			return leChecksFailf("flavor %q has invalid tags %#v", name, flavor["tags"])
		}
		if _, ok := flavor["anchor-files"].([]any); !ok {
			return leChecksFailf("flavor %q has invalid anchor-files %#v", name, flavor["anchor-files"])
		}
		if !strings.Contains(trackedBuild.stdout, name) {
			return leChecksFailf("the tracked-build page does not name matrix flavor %q", name)
		}
	}

	staticMatrixResult, err := runLE(nil, "staticcheck-feature-matrix", "rows", "|", "json")
	if err != nil {
		return err
	}
	if staticMatrixResult.code != 0 {
		return leChecksFailf("`le staticcheck-feature-matrix rows | json` exited %d", staticMatrixResult.code)
	}
	staticMatrixValue, err := leChecksJSON(staticMatrixResult.stdout)
	if err != nil {
		return leChecksFailf("the staticcheck matrix did not answer JSON: %v", err)
	}
	combinations, ok := staticMatrixValue.([]any)
	if !ok || len(combinations) < 3 {
		return leChecksFailf("the matrix answered %#v, want a row per feature plus the two shipped combinations", staticMatrixValue)
	}
	first, err := leChecksObject(combinations[0], "the first staticcheck matrix row")
	if err != nil {
		return err
	}
	second, err := leChecksObject(combinations[1], "the second staticcheck matrix row")
	if err != nil {
		return err
	}
	if first["name"] != "all_features" || second["name"] != "core_only" {
		return leChecksFailf("the first two rows are %q and %q", first["name"], second["name"])
	}

	for _, tc := range []struct {
		command string
		cases   int
	}{
		{checkFSPersistence, 8},
		{checkPluginBoundary, 7},
		{checkYANGLeafMentions, 7},
		{checkPortDefaults, 8},
		{checkDashStdio, 14},
		{checkCIDispatch, 10},
		{checkRepositoryTrackedBuild, 7},
		{checkTestSensitivity, 45},
	} {
		answered, err := runLE(nil, tc.command, "selftest", "|", "json")
		if err != nil {
			return err
		}
		if answered.code != 0 {
			return leChecksFailf("`le %s selftest | json` exited %d", tc.command, answered.code)
		}
		if err := leChecksPassedRows(answered.stdout, tc.command, tc.cases); err != nil {
			return err
		}
	}

	refused, err := runLE(nil, "cli-grammar", "internal")
	if err != nil {
		return err
	}
	if refused.code != 1 {
		return leChecksFailf("`le cli-grammar internal` exited %d, want a refusal", refused.code)
	}
	if !strings.Contains(refused.stderr, "takes no arguments") {
		return leChecksFailf("the refusal does not say why: %q", refused.stderr)
	}

	for _, command := range []string{
		checkConfigCoercion,
		checkFSPersistence,
		checkPluginBoundary,
		checkYANGLeafMentions,
		checkStaticcheckFeatureMatrix,
		checkDashStdio,
		checkCIDispatch,
		checkRepositoryTrackedBuild,
		checkTestSensitivity,
	} {
		unknown, err := runLE(nil, command, "nonesuch")
		if err != nil {
			return err
		}
		if unknown.code != 2 {
			return leChecksFailf("an unknown action of %s answered %d, want 2", command, unknown.code)
		}
	}

	help, err := runLE(nil)
	if err != nil {
		return err
	}
	helpText := help.stdout + help.stderr
	for _, command := range []string{
		checkConfigClaims,
		checkIfaceResolution,
		checkCommandOwnership,
		checkConfigCoercion,
		checkPortDefaults,
		checkCLIGrammar,
		checkFSPersistence,
		checkPluginBoundary,
		checkYANGLeafMentions,
		checkStaticcheckFeatureMatrix,
		checkDashStdio,
		checkCIDispatch,
		checkRepositoryTrackedBuild,
		checkTestSensitivity,
	} {
		if !strings.Contains(helpText, command) {
			return leChecksFailf("`le` does not list %q in its help", command)
		}
	}

	fmt.Println("OK")
	return nil
}

func leChecksRun(ctx context.Context, dir string, overrides map[string]string, program string, args ...string) (leChecksResult, error) {
	if err := ctx.Err(); err != nil {
		return leChecksResult{}, leChecksFailf("fixture context ended before %q ran: %v", program, err)
	}
	cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = leChecksEnvironment(overrides)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := leChecksResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, leChecksFailf("%q was interrupted: %v", program, ctxErr)
	}
	return result, leChecksFailf("starting %q: %v", program, err)
}

func leChecksEnvironment(overrides map[string]string) []string {
	values := make(map[string]string)
	for _, item := range os.Environ() {
		if key, value, found := strings.Cut(item, "="); found {
			values[key] = value
		}
	}
	maps.Copy(values, overrides)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

func leChecksJSON(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("more than one JSON value")
		}
		return nil, fmt.Errorf("trailing data: %w", err)
	}
	return value, nil
}

func leChecksObject(value any, description string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, leChecksFailf("%s answered %#v, want an object", description, value)
	}
	return object, nil
}

func leChecksRequireKeys(object map[string]any, description string, required ...string) error {
	for _, key := range required {
		if _, exists := object[key]; !exists {
			keys := make([]string, 0, len(object))
			for existing := range object {
				keys = append(keys, existing)
			}
			sort.Strings(keys)
			return leChecksFailf("%s answered no %q key: %v", description, key, keys)
		}
	}
	return nil
}

func leChecksInteger(value any, description string) (int, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, leChecksFailf("%s is %#v, want an integer", description, value)
	}
	integer, err := strconv.Atoi(number.String())
	if err != nil {
		return 0, leChecksFailf("%s is %q, want an integer", description, number.String())
	}
	return integer, nil
}

func leChecksNilOrEmptyRows(value any) bool {
	if value == nil {
		return true
	}
	rows, ok := value.([]any)
	return ok && len(rows) == 0
}

func leChecksPassedRows(text, command string, want int) error {
	value, err := leChecksJSON(text)
	if err != nil {
		return leChecksFailf("`le %s selftest | json` answered invalid JSON: %v", command, err)
	}
	rows, ok := value.([]any)
	if !ok {
		return leChecksFailf("`le %s selftest` answered %#v, want %d case rows", command, value, want)
	}
	if len(rows) != want {
		return leChecksFailf("`le %s selftest` answered %d rows, want %d", command, len(rows), want)
	}
	for i, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return leChecksFailf("%s selftest row %d is %#v, want an object", command, i, value)
		}
		passed, ok := row["passed"].(bool)
		if !ok || !passed {
			return leChecksFailf("a %s selftest case failed: %#v", command, rows)
		}
	}
	return nil
}

func leChecksHasElapsedColumn(page string) bool {
	for i := 0; i+4 <= len(page); i++ {
		if page[i] < '0' || page[i] > '9' {
			continue
		}
		j := i
		for j < len(page) && page[j] >= '0' && page[j] <= '9' {
			j++
		}
		if j >= len(page) || page[j] != '.' {
			continue
		}
		j++
		if j >= len(page) || page[j] < '0' || page[j] > '9' {
			continue
		}
		j++
		if j < len(page) && page[j] == 's' && i > 0 && (page[i-1] == ' ' || page[i-1] == '\t') {
			return true
		}
	}
	return false
}

func leChecksPathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func leChecksPrefix(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func leChecksFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
