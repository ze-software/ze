// Design: docs/architecture/core-design.md -- native Go gates run through le
// Related: runtime.go -- Run, whose pretool-bash action ProbeBash drives

package hookruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// probeBashAction is the hook ProbeBash asks: the one a main-thread Bash call
// meets before it runs.
const probeBashAction = "pretool-bash"

// probeBashInput is the tool_input key a Bash payload carries its command
// under, the one every pretool-bash check reads.
const probeBashInput = "command"

// probeBashCleared names the variables a pretool-bash check reads from the
// hook's own environment rather than from the payload: GOFLAGS and CDPATH
// change what bashRootBuild resolves (goFlags, cdTarget in gobuild.go), and
// CLAUDE_CODE_FORK_SUBAGENT makes bashAgentIdentity judge a call with no agent
// id as a fork subagent's. ProbeBash clears each one, so a verdict does not
// depend on the shell that ran the probe.
var probeBashCleared = [...]string{"GOFLAGS", "CDPATH", "CLAUDE_CODE_FORK_SUBAGENT"}

// probeBashRoot is the variable Run resolves the checkout root from when the
// payload gives none (newContext). ProbeBash points it at the probe tree.
const probeBashRoot = "CLAUDE_PROJECT_DIR"

// probeBashLock serializes ProbeBash: the environment it pins is the
// process's, so two probes at once would read each other's root.
var probeBashLock sync.Mutex

// ProbeBash runs the hook's whole pretool-bash action, through Run, over a
// main-thread payload (no agent id, no session id) for command, with root as
// the checkout, and answers the exit code Run returned and everything it wrote.
//
// It exists so the bash parity golden in internal/le/ai/hooks asks the hook
// for each row's verdict instead of a hand model of it. A model stays green
// when the hook moves: the `go build ./...` row stayed at 0 after the build
// guard began refusing a whole-tree build.
//
// The call budget (bashCallBudget) runs as the action's last check, as it does
// for every call, and counts nothing: a main-thread payload carries no agent
// id. root MUST be a throwaway tree the caller built, never the checkout a
// session works in, because a check may write a marker under it
// (bashDocumentRead) or rebuild a derived file in it (preMaterializeDerived).
//
// Not safe for concurrent use with code that reads or sets the variables in
// probeBashCleared or probeBashRoot: for the duration of the call it sets them
// in the process environment, and restores them before it returns. Calls of
// ProbeBash itself are serialized.
func ProbeBash(command, root string) (code int, output string, err error) {
	if root == "" {
		return 0, "", errors.New("ProbeBash needs the root of a probe tree, and was given none")
	}
	body, err := json.Marshal(Payload{ToolName: toolBash, ToolInput: map[string]any{probeBashInput: command}})
	if err != nil {
		return 0, "", fmt.Errorf("encode the bash payload: %w", err)
	}

	probeBashLock.Lock()
	defer probeBashLock.Unlock()

	restore, err := pinProbeEnvironment(root)
	defer func() {
		if restoreErr := restore(); restoreErr != nil && err == nil {
			err = restoreErr
		}
	}()
	if err != nil {
		return 0, "", err
	}

	var out, errOut bytes.Buffer
	code = Run(probeBashAction, bytes.NewReader(body), &out, &errOut)
	// Run fails open on a panic and answers 0, which a probe would read as an
	// allow. Its recover writes this prefix, so a check that panicked answers
	// as an error rather than as a verdict.
	if strings.HasPrefix(errOut.String(), "["+probeBashAction+"] internal error") {
		return code, out.String() + errOut.String(), errors.New("a pretool-bash check panicked: " + strings.TrimSpace(errOut.String()))
	}
	return code, out.String() + errOut.String(), nil
}

// pinProbeEnvironment clears probeBashCleared and points probeBashRoot at
// root, and answers the function that puts back what was there. The restore
// function is never nil, and it restores every variable it changed even when
// pinning stopped part way.
func pinProbeEnvironment(root string) (func() error, error) {
	type saved struct {
		name    string
		value   string
		present bool
	}
	previous := make([]saved, 0, len(probeBashCleared)+1)
	restore := func() error {
		var failures []error
		for _, entry := range previous {
			if entry.present {
				failures = append(failures, os.Setenv(entry.name, entry.value))
				continue
			}
			failures = append(failures, os.Unsetenv(entry.name))
		}
		return errors.Join(failures...)
	}
	for _, name := range probeBashCleared {
		value, present := os.LookupEnv(name)
		previous = append(previous, saved{name: name, value: value, present: present})
		if err := os.Unsetenv(name); err != nil {
			return restore, fmt.Errorf("clear %s for the probe: %w", name, err)
		}
	}
	value, present := os.LookupEnv(probeBashRoot)
	previous = append(previous, saved{name: probeBashRoot, value: value, present: present})
	if err := os.Setenv(probeBashRoot, root); err != nil {
		return restore, fmt.Errorf("point %s at the probe tree: %w", probeBashRoot, err)
	}
	return restore, nil
}
