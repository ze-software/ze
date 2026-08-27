package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// TestDataTreeAnswersRows pins the shape the command DECLARES against the shape
// it actually answers. It was declared `doc` first, on the reading that a tree
// is one document; formatTreeJSON emits a top-level array, so its rows are the
// top-level nodes. A declaration that disagrees with the answer publishes a
// refusal the product does not make.
func TestDataTreeAnswersRows(t *testing.T) {
	payload, code := dataTree(nil)
	if code != 0 {
		t.Fatalf("dataTree exited %d", code)
	}
	rows, ok := payload.([]any)
	if !ok {
		t.Fatalf("dataTree answers %T, want a list of top-level nodes", payload)
	}
	if len(rows) == 0 {
		t.Fatal("dataTree answered no nodes")
	}
	if _, isRecord := rows[0].(map[string]any); !isRecord {
		t.Errorf("a tree row is %T, want a record", rows[0])
	}
}

// TestDataTreeFilterNarrows proves the flags reach the builder. Without this
// the flag words would be accepted and silently ignored, which is the shape of
// defect this whole spec exists to end.
func TestDataTreeFilterNarrows(t *testing.T) {
	all, code := dataTree(nil)
	if code != 0 {
		t.Fatalf("dataTree exited %d", code)
	}
	commands, code := dataTree([]string{flagCommands})
	if code != 0 {
		t.Fatalf("dataTree --commands exited %d", code)
	}
	allRows, _ := all.([]any)
	cmdRows, _ := commands.([]any)
	if len(cmdRows) == 0 {
		t.Fatal("--commands answered nothing")
	}
	if len(cmdRows) >= len(allRows) {
		t.Errorf("--commands kept %d of %d nodes; the filter did not narrow", len(cmdRows), len(allRows))
	}
}

// TestDataCompletionAnswersCollisions covers the second converted command.
func TestDataCompletionAnswersCollisions(t *testing.T) {
	payload, code := dataCompletion(nil)
	if code != 0 {
		t.Fatalf("dataCompletion exited %d", code)
	}
	envelope, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("dataCompletion answers %T, want an envelope", payload)
	}
	if _, has := envelope["collisions"]; !has {
		t.Errorf("the answer carries no collisions key: %v", envelope)
	}
}

// TestTreeCommandFormsShareOptionSemantics drives both the printed root command
// and local-data handler, then compares their selected JSON data and failures.
func TestTreeCommandFormsShareOptionSemantics(t *testing.T) {
	valid := []struct {
		name string
		args []string
	}{
		{name: "commands", args: []string{flagCommands}},
		{name: "config", args: []string{flagConfig}},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			local, localCode := dataTree(tt.args)
			if localCode != 0 {
				t.Fatalf("dataTree exited %d", localCode)
			}

			printedArgs := append([]string(nil), tt.args...)
			printedArgs = append(printedArgs, "--json")
			printed, printedCode := captureJSONOutput(t, func() int {
				return cmdTree(printedArgs)
			})
			if printedCode != 0 {
				t.Fatalf("cmdTree exited %d", printedCode)
			}
			if !reflect.DeepEqual(printed, local) {
				t.Errorf("printed and local tree selections differ\nprinted: %v\nlocal: %v",
					printed, local)
			}
		})
	}

	invalid := []struct {
		name string
		args []string
	}{
		{name: "unknown option", args: []string{"--unknown"}},
		{name: "conflicting filters", args: []string{flagCommands, flagConfig}},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if code := cmdTree(tt.args); code != 1 {
				t.Errorf("cmdTree exited %d, want 1", code)
			}
			if _, code := dataTree(tt.args); code != 1 {
				t.Errorf("dataTree exited %d, want 1", code)
			}
		})
	}
}

// TestCompletionCommandFormsShareOptionSemantics drives both command forms at
// every range boundary and compares the structured data at valid boundaries.
func TestCompletionCommandFormsShareOptionSemantics(t *testing.T) {
	tests := []struct {
		name  string
		value string
		code  int
	}{
		{name: "below minimum", value: "0", code: 1},
		{name: "minimum", value: "1", code: 0},
		{name: "maximum", value: "10", code: 0},
		{name: "above maximum", value: "11", code: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"--min-prefix", tt.value}
			local, localCode := dataCompletion(args)
			if localCode != tt.code {
				t.Errorf("dataCompletion exited %d, want %d", localCode, tt.code)
			}

			printedArgs := append([]string(nil), args...)
			printedArgs = append(printedArgs, "--json")
			if tt.code != 0 {
				if code := cmdCompletion(printedArgs); code != tt.code {
					t.Errorf("cmdCompletion exited %d, want %d", code, tt.code)
				}
				return
			}

			printed, printedCode := captureJSONOutput(t, func() int {
				return cmdCompletion(printedArgs)
			})
			if printedCode != tt.code {
				t.Fatalf("cmdCompletion exited %d, want %d", printedCode, tt.code)
			}
			if !reflect.DeepEqual(printed, local) {
				t.Errorf("printed and local completion selections differ\nprinted: %v\nlocal: %v",
					printed, local)
			}
		})
	}

	args := []string{"--unknown"}
	if code := cmdCompletion(args); code != 1 {
		t.Errorf("cmdCompletion with unknown option exited %d, want 1", code)
	}
	if _, code := dataCompletion(args); code != 1 {
		t.Errorf("dataCompletion with unknown option exited %d, want 1", code)
	}
}

// TestLocalDataRejectsPrintedRenderingFlag proves the structured handlers do
// not silently ignore a rendering flag that only the printed form can honor.
func TestLocalDataRejectsPrintedRenderingFlag(t *testing.T) {
	if _, code := dataTree([]string{"--json"}); code != 1 {
		t.Errorf("dataTree --json exited %d, want 1", code)
	}
	if _, code := dataCompletion([]string{"--json"}); code != 1 {
		t.Errorf("dataCompletion --json exited %d, want 1", code)
	}
}

func captureJSONOutput(t *testing.T, run func() int) (any, int) {
	t.Helper()

	output, err := os.CreateTemp(t.TempDir(), "yang-command-output")
	if err != nil {
		t.Fatalf("create output file: %v", err)
	}

	stdout := os.Stdout
	os.Stdout = output
	code := run()
	os.Stdout = stdout
	if err := output.Close(); err != nil {
		t.Fatalf("close output file: %v", err)
	}

	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode command output: %v\noutput: %s", err, data)
	}
	return payload, code
}
