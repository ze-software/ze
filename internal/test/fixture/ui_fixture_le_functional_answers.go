package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
)

func init() {
	Register("ui/le-functional-answers", uiDriver(leFunctionalAnswers))
}

type uiLeFunctionalAnswersCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func leFunctionalAnswers(ctx context.Context) error {
	root, ok := os.LookupEnv("ZE_REPO_ROOT")
	if !ok || root == "" {
		return uiLeFunctionalAnswersFailf("ZE_REPO_ROOT is not set")
	}

	here, _, err := temporaryLEFixtureWorkspace("le-functional-answers-")
	if err != nil {
		return uiLeFunctionalAnswersFailf("creating the fixture directory: %v", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup
	binary, err := uiLEBinary(root)
	if err != nil {
		return uiLeFunctionalAnswersFailf("%v", err)
	}

	// The suite table and the built personality.
	command, err := uiLeFunctionalAnswersRunCommand(ctx, here, binary, "functional", "list", "|", "json")
	if err != nil {
		return err
	}
	if command.exitCode != 0 {
		return uiLeFunctionalAnswersFailf("`le functional list | json` exited %d", command.exitCode)
	}

	var suites []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(command.stdout), &suites); err != nil {
		return uiLeFunctionalAnswersFailf("`le functional list | json` returned invalid JSON: %v", err)
	}
	if len(suites) <= 20 {
		return uiLeFunctionalAnswersFailf("the command published %d suites, which is too few to mean anything", len(suites))
	}

	// A second answer verifies that the complete row order is deterministic.
	repeated, err := uiLeFunctionalAnswersRunCommand(ctx, here, binary, "functional", "list", "|", "json")
	if err != nil {
		return err
	}
	if repeated.exitCode != 0 {
		return uiLeFunctionalAnswersFailf("the repeated `le functional list | json` exited %d", repeated.exitCode)
	}
	var repeatedSuites []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(repeated.stdout), &repeatedSuites); err != nil {
		return uiLeFunctionalAnswersFailf("the repeated `le functional list | json` returned invalid JSON: %v", err)
	}
	if !reflect.DeepEqual(suites, repeatedSuites) {
		return uiLeFunctionalAnswersFailf("the functional suite table or its ordering changed between answers")
	}

	var gatingNames []string
	seenNames := make(map[string]struct{}, len(suites))
	wantFields := []string{fieldName, "gating", "action", fieldRerun, "budget", "budget-variable", fieldCommand, "why"}
	for i, row := range suites {
		if len(row) != len(wantFields) {
			return uiLeFunctionalAnswersFailf("suite row %d has %d fields, want exactly %v", i, len(row), wantFields)
		}
		for _, key := range wantFields {
			if _, ok := row[key]; !ok {
				return uiLeFunctionalAnswersFailf("suite row %d omitted %s", i, key)
			}
		}

		name, err := requiredString(row, "name", fmt.Sprintf("suite row %d", i))
		if err != nil {
			return err
		}
		action, err := requiredString(row, "action", "suite "+name)
		if err != nil {
			return err
		}
		rerun, err := requiredString(row, "rerun", "suite "+name)
		if err != nil {
			return err
		}
		budget, err := requiredString(row, "budget", "suite "+name)
		if err != nil {
			return err
		}
		budgetVariable, err := requiredString(row, "budget-variable", "suite "+name)
		if err != nil {
			return err
		}
		why, err := requiredString(row, "why", "suite "+name)
		if err != nil {
			return err
		}
		isGating, err := requiredBool(row, "gating", "suite "+name)
		if err != nil {
			return err
		}
		var commandArgv []string
		if err := json.Unmarshal(row["command"], &commandArgv); err != nil || len(commandArgv) == 0 {
			return uiLeFunctionalAnswersFailf("suite %s has an invalid command: %s", name, row["command"])
		}

		if name == "" || action != name {
			return uiLeFunctionalAnswersFailf("suite row %d has name %q and action %q", i, name, action)
		}
		if _, exists := seenNames[name]; exists {
			return uiLeFunctionalAnswersFailf("suite %s appears more than once", name)
		}
		seenNames[name] = struct{}{}
		if rerun != "./le functional "+action {
			return uiLeFunctionalAnswersFailf("suite %s reruns with %q, want its native action", name, rerun)
		}
		if budget == "" || budgetVariable == "" || why == "" {
			return uiLeFunctionalAnswersFailf("suite %s omitted budget or purpose metadata", name)
		}
		if isGating {
			gatingNames = append(gatingNames, name)
		}
	}
	firstName, err := requiredString(suites[0], "name", "first suite")
	if err != nil {
		return err
	}
	if firstName != "encode" {
		return uiLeFunctionalAnswersFailf("the first functional suite is %q, want encode", firstName)
	}

	// The gating set is DERIVED, never counted here. A literal beside a registry
	// is the drift this repository records (ai/rules/principles.md): three suites
	// earned their own names (bfd, dhcp, vrrp) and a hand-written 24 went red for
	// the tree being right. `le functional select` publishes the run list a
	// gating run would start, so the two surfaces are cross-checked against each
	// other, which catches a name moving between them and not only a count.
	plan, err := uiLeFunctionalAnswersRunCommand(ctx, here, binary, "functional", "select", "|", "json")
	if err != nil {
		return err
	}
	if plan.exitCode != 0 {
		return uiLeFunctionalAnswersFailf("`le functional select | json` exited %d", plan.exitCode)
	}
	var selection struct {
		Running  []string `json:"running"`
		RuledOut []string `json:"ruled-out"`
		Skipped  []string `json:"skipped"`
	}
	if err := json.Unmarshal([]byte(plan.stdout), &selection); err != nil {
		return uiLeFunctionalAnswersFailf("`le functional select | json` returned invalid JSON: %v", err)
	}
	planned := slices.Concat(selection.Running, selection.RuledOut, selection.Skipped)
	slices.Sort(planned)
	wantGating := slices.Clone(gatingNames)
	slices.Sort(wantGating)
	if !slices.Equal(planned, wantGating) {
		return uiLeFunctionalAnswersFailf("`list` marks %v gating and `select` plans %v", wantGating, planned)
	}
	// A vacuity floor, not a copy of the count: an empty plan matching an empty
	// gating set would satisfy the comparison above and prove nothing.
	if len(gatingNames) <= 20 {
		return uiLeFunctionalAnswersFailf("the command marks %d suites gating, which is too few to mean anything", len(gatingNames))
	}

	// One payload through every supported rendering used by this contract.
	for _, operator := range []string{renderYAML, renderTable} {
		rendered, err := uiLeFunctionalAnswersRunCommand(ctx, here, binary, "functional", "list", "|", operator)
		if err != nil {
			return err
		}
		if rendered.exitCode != 0 {
			return uiLeFunctionalAnswersFailf("`le functional list | %s` exited %d", operator, rendered.exitCode)
		}
		if !strings.Contains(rendered.stdout, "encode") {
			return uiLeFunctionalAnswersFailf("`le functional list | %s` dropped the first suite", operator)
		}
	}

	// A name not held by this area is a refusal, distinguishable from a suite
	// that ran and failed.
	missing, err := uiLeFunctionalAnswersRunCommand(ctx, here, binary, "functional", "no-such-suite")
	if err != nil {
		return err
	}
	if missing.exitCode != 2 {
		return uiLeFunctionalAnswersFailf("`le functional no-such-suite` exited %d, want 2", missing.exitCode)
	}
	if missing.stdout != "" {
		return uiLeFunctionalAnswersFailf("a refused command wrote to stdout: %q", missing.stdout)
	}

	// Integration has no aggregate run. Its bare answer still names the
	// refusal, and its piped answer still carries the action listing.
	bare, err := uiLeFunctionalAnswersRunCommand(ctx, here, binary, "integration")
	if err != nil {
		return err
	}
	if bare.exitCode != 2 {
		return uiLeFunctionalAnswersFailf("`le integration` exited %d, want the refusal 2", bare.exitCode)
	}
	if !strings.Contains(bare.stderr, "no aggregate run") {
		return uiLeFunctionalAnswersFailf("the refusal said nothing: %q", bare.stderr)
	}

	listing, err := uiLeFunctionalAnswersRunCommand(ctx, here, binary, "integration", "|", "json")
	if err != nil {
		return err
	}
	if listing.exitCode != 2 {
		return uiLeFunctionalAnswersFailf("`le integration | json` exited %d, want 2", listing.exitCode)
	}
	var integration map[string]json.RawMessage
	if err := json.Unmarshal([]byte(listing.stdout), &integration); err != nil {
		return uiLeFunctionalAnswersFailf("`le integration | json` returned invalid JSON: %v", err)
	}
	actionsRaw, ok := integration["actions"]
	if !ok {
		return uiLeFunctionalAnswersFailf("`le integration | json` omitted actions")
	}
	var actions []map[string]json.RawMessage
	if err := json.Unmarshal(actionsRaw, &actions); err != nil {
		return uiLeFunctionalAnswersFailf("the integration actions are invalid: %v", err)
	}
	verbs := make([]string, 0, len(actions))
	for i, action := range actions {
		verb, err := requiredString(action, "verb", fmt.Sprintf("integration action %d", i))
		if err != nil {
			return err
		}
		verbs = append(verbs, verb)
	}
	if !slices.Contains(verbs, "iface") {
		return uiLeFunctionalAnswersFailf("the integration area lost iface")
	}
	if !slices.Contains(verbs, "interop") {
		return uiLeFunctionalAnswersFailf("the integration area lost interop")
	}

	// The action count is DERIVED from the refusal, never written down here. The
	// bare command names every gate it would accept, so the hint and the JSON
	// listing are two publications of one table and a verb that reaches only one
	// of them is the defect. A literal here goes red for the tree being right,
	// which is what a count beside a registry always does
	// (ai/rules/principles.md).
	offered := uiLeFunctionalAnswersOfferedGates(bare.stderr)
	if len(offered) <= 10 {
		return uiLeFunctionalAnswersFailf("the refusal offered %d gates, which is too few to mean anything: %q", len(offered), bare.stderr)
	}
	slices.Sort(offered)
	listed := slices.Clone(verbs)
	slices.Sort(listed)
	if !slices.Equal(offered, listed) {
		return uiLeFunctionalAnswersFailf("the refusal offers %v and the listing carries %v", offered, listed)
	}

	fmt.Println("OK")
	return nil
}

// uiLeFunctionalAnswersOfferedGates reads the gate names out of the refusal
// `le integration` writes with no action. The names are one comma-separated
// line under the sentence, which is the only indented line the refusal writes.
func uiLeFunctionalAnswersOfferedGates(refusal string) []string {
	var names []string
	for line := range strings.SplitSeq(refusal, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		for name := range strings.SplitSeq(strings.TrimSpace(line), ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func uiLeFunctionalAnswersRunCommand(ctx context.Context, dir, program string, args ...string) (uiLeFunctionalAnswersCommandResult, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := uiLeFunctionalAnswersCommandResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return uiLeFunctionalAnswersCommandResult{}, uiLeFunctionalAnswersFailf("running %s: %v", program, ctxErr)
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}
	return uiLeFunctionalAnswersCommandResult{}, uiLeFunctionalAnswersFailf("starting %s: %v", program, err)
}

func requiredString(row map[string]json.RawMessage, key, subject string) (string, error) {
	raw, ok := row[key]
	if !ok {
		return "", uiLeFunctionalAnswersFailf("%s omitted %s", subject, key)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", uiLeFunctionalAnswersFailf("%s has a non-string %s", subject, key)
	}
	return value, nil
}

func requiredBool(row map[string]json.RawMessage, key, subject string) (bool, error) {
	raw, ok := row[key]
	if !ok {
		return false, uiLeFunctionalAnswersFailf("%s omitted %s", subject, key)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, uiLeFunctionalAnswersFailf("%s has a non-boolean %s", subject, key)
	}
	return value, nil
}

func uiLeFunctionalAnswersFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
