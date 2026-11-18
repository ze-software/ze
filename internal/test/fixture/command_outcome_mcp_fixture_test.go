// Design: docs/architecture/testing/ci-format.md -- compiled fixture verdicts.
// Related: command_outcome_surfaces_fixture.go -- actual daemon-to-MCP assertions.

package fixture

import "testing"

// Goal: a status-only warning retains its original rows and visible label
// without becoming an error. Method: literal results pair valid warning and
// healthy controls with lost labels, false errors and corrupted payloads.
func TestCommandOutcomeMCPVerdictKeepsStatusOnlyWarning(t *testing.T) {
	const prefix = "probe status=200 code=ok result="
	const rows = `{"type":"text","text":"{\"items\":[{\"ordinal\":0},{\"ordinal\":1}]}"}`
	const partial = `{"type":"text","text":"{\"items\":[{\"ordinal\":0},{\"ordinal\":1}],\"errors\":[{\"cause\":\"fixture acknowledgement refused\"}]}"}`
	const head = `{"resultType":"complete","content":[`
	const warning = `,{"type":"text","text":"Warning"}`
	for _, question := range []struct {
		name    string
		outcome string
		result  string
		accept  bool
	}{
		{name: "healthy control", outcome: "healthy", result: head + rows + `]}`, accept: true},
		{name: "status only warning", outcome: "warning", result: head + rows + warning + `]}`, accept: true},
		{name: "lost warning label", outcome: "warning", result: head + rows + `]}`},
		{name: "invented warning diagnostic", outcome: "warning", result: head + rows + `,{"type":"text","text":"Warning: Warning"}]}`},
		{name: "warning became error", outcome: "warning", result: head + rows + warning + `],"isError":true}`},
		{name: "warning lost original data", outcome: "warning", result: head + `{"type":"text","text":"{\"items\":[]}"}` + warning + `]}`},
		{name: "warning changed ordinal", outcome: "warning", result: head + `{"type":"text","text":"{\"items\":[{\"ordinal\":0},{\"ordinal\":7}]}"}` + warning + `]}`},
		{name: "healthy acquired warning", outcome: "healthy", result: head + rows + warning + `]}`},
		{name: "partial remains error", outcome: "partial", result: head + partial + `,{"type":"text","text":"Error: partial"}],"isError":true}`, accept: true},
		{name: "failed remains error", outcome: "failed", result: head + rows + `,{"type":"text","text":"Error: unknown error"}],"isError":true}`, accept: true},
	} {
		t.Run(question.name, func(t *testing.T) {
			err := commandOutcomeMCPResult(prefix+question.result, commandOutcomeQuestion{outcome: question.outcome, count: 2})
			if question.accept {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("fixture accepted an answer that lost the original command outcome")
				}
			}
		})
	}
}
