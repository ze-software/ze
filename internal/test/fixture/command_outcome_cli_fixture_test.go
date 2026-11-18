// Design: docs/architecture/testing/ci-format.md -- compiled fixture verdicts.
// Related: command_outcome_cli_fixture.go -- actual daemon-to-CLI assertions.

package fixture

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/test/externalyang"
)

// Goal: failure status must mean that the real provider answered, not merely
// that the CLI failed to connect. Method: literal valid responses are paired
// with missing data, wrong status, lost fault causes and lost diagnostics.
func TestCommandOutcomeCLIVerdictDiscriminatesCompletedFailure(t *testing.T) {
	const rows = `{"items":[{"ordinal":0},{"ordinal":1}]}` + "\n"
	const partial = `{"items":[{"ordinal":0},{"ordinal":1}],"errors":[{"cause":"fixture acknowledgement refused"}]}` + "\n"
	for _, question := range []struct {
		name    string
		outcome string
		result  fixture10ProcessResult
		accept  bool
	}{
		{name: "healthy control", outcome: "healthy", result: fixture10ProcessResult{stdout: rows}, accept: true},
		{name: "partial complete", outcome: "partial", result: fixture10ProcessResult{code: 1, stdout: partial, stderr: "error: Process exited with status 1\n"}, accept: true},
		{name: "failed complete", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: rows, stderr: "error: unknown error\n"}, accept: true},
		{name: "empty disconnected failure", outcome: "partial", result: fixture10ProcessResult{code: 1, stderr: "error: connection refused\n"}},
		{name: "false successful partial", outcome: "partial", result: fixture10ProcessResult{stdout: partial, stderr: "error: Process exited with status 1\n"}},
		{name: "missing fault row", outcome: "partial", result: fixture10ProcessResult{code: 1, stdout: rows, stderr: "error: Process exited with status 1\n"}},
		{name: "changed cause", outcome: "partial", result: fixture10ProcessResult{code: 1, stdout: `{"items":[{"ordinal":0},{"ordinal":1}],"errors":[{"cause":"different"}]}` + "\n", stderr: "error: Process exited with status 1\n"}},
		{name: "missing item", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: `{"items":[{"ordinal":0}]}` + "\n", stderr: "error: unknown error\n"}},
		{name: "missing ordinal", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: `{"items":[{"ordinal":0},{}]}` + "\n", stderr: "error: unknown error\n"}},
		{name: "changed ordinal", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: `{"items":[{"ordinal":0},{"ordinal":2}]}` + "\n", stderr: "error: unknown error\n"}},
		{name: "missing failure cause", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: rows}},
		{name: "missing final newline", outcome: "healthy", result: fixture10ProcessResult{stdout: `{"items":[{"ordinal":0},{"ordinal":1}]}`}},
		{name: "healthy diagnostic", outcome: "healthy", result: fixture10ProcessResult{stdout: rows, stderr: "unexpected warning\n"}},
	} {
		t.Run(question.name, func(t *testing.T) {
			err := commandOutcomeCLIVerdict(question.result, externalyang.OutcomeQuestion{Command: "show answer-outcome " + question.outcome + " short", Outcome: question.outcome, Count: 2})
			if question.accept {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("fixture accepted an answer that did not prove the completed command outcome")
				}
			}
		})
	}
}

// Goal: an existing transcript file must contain this actual command and body
// once. Method: accept a literal recorded answer and refuse absent, empty,
// unrelated, duplicated or truncated records in private test directories.
func TestCommandOutcomeCLITranscriptDiscriminatesMissingProgress(t *testing.T) {
	const input = "show answer-outcome failed short | raw"
	const body = `{"items":[{"ordinal":0},{"ordinal":1}]}`
	const record = "# Ze CLI Transcript\n[12:00:00] > " + input + "\n" + body + "\n\n"
	for _, question := range []struct {
		name   string
		files  []string
		accept bool
	}{
		{name: "recorded completed failure", files: []string{record}, accept: true},
		{name: "missing file"},
		{name: "empty file", files: []string{""}},
		{name: "header only", files: []string{"# Ze CLI Transcript\n"}},
		{name: "wrong command", files: []string{"[12:00:00] > show version\n" + body + "\n\n"}},
		{name: "lost body", files: []string{"[12:00:00] > " + input + "\n\n"}},
		{name: "changed body", files: []string{"[12:00:00] > " + input + "\n{}\n\n"}},
		{name: "duplicate command", files: []string{record + record}},
		{name: "extra transcript", files: []string{record, record}},
	} {
		t.Run(question.name, func(t *testing.T) {
			dataHome := t.TempDir()
			directory := filepath.Join(dataHome, "ze", "transcripts")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			for index, content := range question.files {
				path := filepath.Join(directory, string(rune('a'+index))+".log")
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := commandOutcomeCLITranscript(dataHome, input, body)
			if question.accept {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("fixture accepted an absent or different command transcript")
				}
			}
		})
	}
}

// Goal: selected CLI output must prove a real row and the completed original
// verdict. Method: literals pair each valid outcome with broken rows and tails.
func TestCommandOutcomeCLIFirstVerdictDiscriminatesLateFailure(t *testing.T) {
	const row = `{"items":[{"ordinal":0}],"pipe":[{"arg":"1","op":"first"}]}` + "\n"
	for _, question := range []struct {
		name    string
		outcome string
		result  fixture10ProcessResult
		accept  bool
	}{
		{name: "healthy", outcome: "healthy", result: fixture10ProcessResult{stdout: row}, accept: true},
		{name: "partial", outcome: "partial", result: fixture10ProcessResult{code: 1, stdout: row, stderr: "error: Process exited with status 1\n"}, accept: true},
		{name: "failed", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: row, stderr: "error: unknown error\n"}, accept: true},
		{name: "false successful partial", outcome: "partial", result: fixture10ProcessResult{stdout: row}},
		{name: "false successful failed", outcome: "failed", result: fixture10ProcessResult{stdout: row}},
		{name: "connection failure", outcome: "failed", result: fixture10ProcessResult{code: 1, stderr: "error: connection refused\n"}},
		{name: "missing ordinal", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: `{"items":[{}],"pipe":[{"arg":"1","op":"first"}]}` + "\n", stderr: "error: unknown error\n"}},
		{name: "changed ordinal", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: `{"items":[{"ordinal":1}],"pipe":[{"arg":"1","op":"first"}]}` + "\n", stderr: "error: unknown error\n"}},
		{name: "missing pipe metadata", outcome: "healthy", result: fixture10ProcessResult{stdout: `{"items":[{"ordinal":0}]}` + "\n"}},
		{name: "wrong pipe metadata", outcome: "healthy", result: fixture10ProcessResult{stdout: `{"items":[{"ordinal":0}],"pipe":[{"arg":"2","op":"first"}]}` + "\n"}},
		{name: "wrong pipe operation", outcome: "healthy", result: fixture10ProcessResult{stdout: `{"items":[{"ordinal":0}],"pipe":[{"arg":"1","op":"last"}]}` + "\n"}},
		{name: "extra item", outcome: "healthy", result: fixture10ProcessResult{stdout: `{"items":[{"ordinal":0},{"ordinal":1}],"pipe":[{"arg":"1","op":"first"}]}` + "\n"}},
		{name: "unselected fault", outcome: "partial", result: fixture10ProcessResult{code: 1, stdout: `{"items":[{"ordinal":0}],"errors":[{"cause":"fixture acknowledgement refused"}],"pipe":[{"arg":"1","op":"first"}]}` + "\n", stderr: "error: Process exited with status 1\n"}},
		{name: "extra row", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: row + row, stderr: "error: unknown error\n"}},
		{name: "missing failure", outcome: "failed", result: fixture10ProcessResult{code: 1, stdout: row}},
		{name: "missing final newline", outcome: "healthy", result: fixture10ProcessResult{stdout: `{"items":[{"ordinal":0}],"pipe":[{"arg":"1","op":"first"}]}`}},
		{name: "healthy diagnostic", outcome: "healthy", result: fixture10ProcessResult{stdout: row, stderr: "unexpected warning\n"}},
	} {
		t.Run(question.name, func(t *testing.T) {
			err := commandOutcomeCLIFirstVerdict(question.result, externalyang.OutcomeQuestion{
				Command: "show answer-outcome " + question.outcome + " long", Outcome: question.outcome, Count: 1,
			})
			if question.accept {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("fixture accepted a selected row without its completed original outcome")
				}
			}
		})
	}
}

// Goal: a status-only warning stays visible without becoming a failed command.
// Method: independent literal payloads reject lost labels, invented failure
// text, nonzero exits and corrupted rows for both full and selected answers.
func TestCommandOutcomeCLIWarningVerdictDiscriminatesInvisibleWarning(t *testing.T) {
	const rows = `{"items":[{"ordinal":0},{"ordinal":1}]}` + "\n"
	const selected = `{"items":[{"ordinal":0}],"pipe":[{"arg":"1","op":"first"}]}` + "\n"
	for _, first := range []bool{false, true} {
		for _, test := range []struct {
			name   string
			code   int
			stderr string
			accept bool
		}{
			{name: "visible warning", stderr: "Warning\n", accept: true},
			{name: "invisible warning"},
			{name: "failed warning", code: 1, stderr: "Warning\n"},
			{name: "invented original diagnostic", stderr: "Warning: cleanup incomplete\n"},
			{name: "invented failure", stderr: "error: unknown error\n"},
			{name: "duplicate label", stderr: "Warning\nWarning\n"},
		} {
			t.Run(fmt.Sprintf("%s/first=%t", test.name, first), func(t *testing.T) {
				question := externalyang.OutcomeQuestion{Command: "show answer-outcome warning short", Outcome: "warning", Count: 2}
				result := fixture10ProcessResult{code: test.code, stdout: rows, stderr: test.stderr}
				var err error
				if first {
					question.Command = "show answer-outcome warning long"
					question.Count = 1
					result.stdout = selected
					err = commandOutcomeCLIFirstVerdict(result, question)
				} else {
					err = commandOutcomeCLIVerdict(result, question)
				}
				if test.accept {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					if err == nil {
						t.Fatal("fixture accepted an invisible or altered warning outcome")
					}
				}
			})
		}
	}
}
