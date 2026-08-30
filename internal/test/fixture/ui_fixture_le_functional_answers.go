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

	gating := 0
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
			gating++
		}
	}
	firstName, err := requiredString(suites[0], "name", "first suite")
	if err != nil {
		return err
	}
	if firstName != "encode" {
		return uiLeFunctionalAnswersFailf("the first functional suite is %q, want encode", firstName)
	}
	if gating != 24 {
		return uiLeFunctionalAnswersFailf("the command marks %d suites gating, want 24", gating)
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
	if len(actions) != 13 {
		return uiLeFunctionalAnswersFailf("the integration area listed %d actions, want 13", len(actions))
	}
	verbs := make(map[string]struct{}, len(actions))
	for i, action := range actions {
		verb, err := requiredString(action, "verb", fmt.Sprintf("integration action %d", i))
		if err != nil {
			return err
		}
		verbs[verb] = struct{}{}
	}
	if _, ok := verbs["iface"]; !ok {
		return uiLeFunctionalAnswersFailf("the integration area lost iface")
	}
	if _, ok := verbs["interop"]; !ok {
		return uiLeFunctionalAnswersFailf("the integration area lost interop")
	}

	fmt.Println("OK")
	return nil
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
