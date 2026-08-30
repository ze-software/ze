package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
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

// TestTreeCommandFormsShareOptionSemantics drives both the printed root
// command and the local-data handler over the same option words, then compares
// what each selected.
//
// The two forms share parseTreeOptions and buildUnifiedTree, and differ only
// in their formatter. The check is therefore that the printed form printed
// every top-level node the data answer carries. Both once answered JSON, and
// the comparison was byte-for-byte. The printed form renders text alone now,
// because `show yang tree | json` renders the answer.
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
			rows, ok := local.([]any)
			if !ok || len(rows) == 0 {
				t.Fatalf("dataTree answered %T with no rows", local)
			}

			printed, printedCode := captureText(t, func() int { return cmdTree(tt.args) })
			if printedCode != 0 {
				t.Fatalf("cmdTree exited %d", printedCode)
			}
			for _, row := range rows {
				node, isRecord := row.(map[string]any)
				if !isRecord {
					t.Fatalf("a tree row is %T, want a record", row)
				}
				name, _ := node["name"].(string)
				if name == "" {
					t.Fatalf("a tree row carries no name: %v", node)
				}
				if !strings.Contains(printed, name) {
					t.Errorf("the printed tree omits %q, which the data answer carries", name)
				}
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
// every range boundary, and compares the counts each form reports at the valid
// ones. The printed form ends with a Summary line. It carries the two numbers
// the data answer carries as fields, so a drift between the forms shows there.
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

			if tt.code != 0 {
				if code := cmdCompletion(args); code != tt.code {
					t.Errorf("cmdCompletion exited %d, want %d", code, tt.code)
				}
				return
			}

			printed, printedCode := captureText(t, func() int { return cmdCompletion(args) })
			if printedCode != tt.code {
				t.Fatalf("cmdCompletion exited %d, want %d", printedCode, tt.code)
			}
			envelope, ok := local.(map[string]any)
			if !ok {
				t.Fatalf("dataCompletion answers %T, want an envelope", local)
			}
			summary, _ := envelope["summary"].(map[string]any)
			groups, _ := summary["total-groups"].(float64)
			affected, _ := summary["total-affected"].(float64)
			want := fmt.Sprintf("Summary: %d collision groups, %d affected nodes",
				int(groups), int(affected))
			if !strings.Contains(printed, want) {
				t.Errorf("the printed form does not report %q", want)
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

// TestBothFormsRefuseTheDeletedRenderingOption proves `--json` is REFUSED on
// both forms rather than dropped in silence. Rendering belongs to the pipe
// layer. A token a parser drops would print a tree the caller cannot parse,
// while the exit code reported that the request was honored.
func TestBothFormsRefuseTheDeletedRenderingOption(t *testing.T) {
	if code := cmdTree([]string{"--json"}); code != 1 {
		t.Errorf("cmdTree --json exited %d, want 1", code)
	}
	if _, code := dataTree([]string{"--json"}); code != 1 {
		t.Errorf("dataTree --json exited %d, want 1", code)
	}
	if code := cmdCompletion([]string{"--json"}); code != 1 {
		t.Errorf("cmdCompletion --json exited %d, want 1", code)
	}
	if _, code := dataCompletion([]string{"--json"}); code != 1 {
		t.Errorf("dataCompletion --json exited %d, want 1", code)
	}
}

// TestYANGAnswersRenderThroughThePipeLayer proves both answers are served in
// this process and reach `| json`, which is what replaced the deleted option.
func TestYANGAnswersRenderThroughThePipeLayer(t *testing.T) {
	answer, code, served := command.ServeLocal("show yang tree "+flagCommands+" | json", "")
	if !served {
		t.Fatal("show yang tree was not served in this process")
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (answer: %q)", code, answer)
	}
	var nodes []map[string]any
	if err := json.Unmarshal([]byte(answer), &nodes); err != nil {
		t.Fatalf("| json answered something no JSON decoder takes: %v (answer: %q)", err, answer)
	}
	if len(nodes) == 0 {
		t.Fatalf("| json answered no nodes: %q", answer)
	}

	answer, code, served = command.ServeLocal("show yang completion | json", "")
	if !served {
		t.Fatal("show yang completion was not served in this process")
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (answer: %q)", code, answer)
	}
	// The completion answer keeps its envelope through `| json` because it
	// carries two keys: the collision rows and the summary counted over them.
	// The tree above is unwrapped because its answer is the rows alone.
	var envelope map[string]any
	if err := json.Unmarshal([]byte(answer), &envelope); err != nil {
		t.Fatalf("| json answered something no JSON decoder takes: %v (answer: %q)", err, answer)
	}
	if _, has := envelope["collisions"]; !has {
		t.Errorf("the rendered answer carries no collisions key: %q", answer)
	}
	if _, has := envelope["summary"]; !has {
		t.Errorf("the rendered answer carries no summary key: %q", answer)
	}
}

// TestShowYANGTreeRefusesAddressOperatorsByName proves the declared shape is
// load-bearing. No field of a tree node holds an address, so `| resolve` is
// refused by name.
func TestShowYANGTreeRefusesAddressOperatorsByName(t *testing.T) {
	answer, code, served := command.ServeLocal("show yang tree | resolve", "")
	if !served {
		t.Fatal("show yang tree was not served in this process")
	}
	if code == 0 {
		t.Fatalf("| resolve was accepted over an answer holding no address (answer: %q)",
			strings.SplitN(answer, "\n", 2)[0])
	}
	if !strings.Contains(answer, "resolve") {
		t.Errorf("the refusal does not name the operator: %q", answer)
	}
}

// captureText runs a printing command and answers what it wrote to stdout.
func captureText(t *testing.T, run func() int) (string, int) {
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

	written, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	return string(written), code
}
