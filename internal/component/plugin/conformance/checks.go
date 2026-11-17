// Design: docs/architecture/plugin/plugin-system.md -- the plugin conformance checker
// Overview: conformance.go -- Run, the report and the check registry
// Related: register.go -- the init() that registers these checks
//
// checks.go holds the checks of this version. The declare-mode checks judge
// what one run of the program produced. The Stage 1 rows come from the
// daemon's own check table. Every documented behavior nobody judges yet is a
// check that answers not-judged with its reason.

package conformance

import (
	"bytes"
	"encoding/json"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The stage a row belongs to.
const (
	stageDeclare   = "declare"
	stageOne       = "1"
	stageFive      = "5"
	stageTransport = "transport"
	stageRuntime   = "runtime"
	stageShutdown  = "shutdown"
	stageSchema    = "schema"
)

// protocolPage is the page a plugin author reads the contract on.
const protocolPage = "docs/plugin-development/protocol.md"

// declarationID is the request id of the declaration line. rpc.WriteDeclaration
// writes it, and TestStrictLineAcceptsTheLineTheProtocolWrites holds the two
// in step.
const declarationID = 1

// fixUseRunOrDeclare is the change that makes a Go plugin answer a query: the
// SDK entry point writes the line and exits 0 without calling the start
// function.
const fixUseRunOrDeclare = "when ZE_PLUGIN_MODE=declare is set, write the declaration line and exit 0 " +
	"without starting the plugin: in Go, return sdk.RunOrDeclare(declaration, start) from main"

// notJudged builds the check of a documented behavior this version does not
// judge. Its row says so on every report, which is what stops a report with
// no failure from reading as full conformance.
func notJudged(order int, name, stage, reason string) Check {
	return Check{Name: name, Order: order, Rows: func(*Evidence) []Row {
		return []Row{{
			Check:   name,
			Stage:   stage,
			Verdict: VerdictNotJudged,
			Reason:  reason,
			Fix:     "test this behavior under a daemon: configure the plugin with plugin { external <name> { run ...; } } and start ze",
		}}
	}}
}

// started reports whether the run string produced a plugin process. Every
// check after program-starts answers not-judged when it did not.
func started(evidence *Evidence) bool {
	return evidence.Query.Exit != plugin.QueryExitNotStarted
}

func notStartedRow(check, stage string) []Row {
	return []Row{{
		Check: check, Stage: stage, Verdict: VerdictNotJudged,
		Reason: "the program did not start, so there is nothing to judge",
		Fix:    "fix the program-starts row first",
	}}
}

// programStarts judges that the run string starts a program in declare mode.
func programStarts(evidence *Evidence) []Row {
	const check = "program-starts"
	if started(evidence) {
		return []Row{{Check: check, Stage: stageDeclare, Verdict: VerdictPass}}
	}

	var reason, fix textbuf.Buffer
	reason.Str("the run string ").Quoted(evidence.Target.Run).Str(" started no program")
	if evidence.Query.ExitCode != 0 {
		reason.Str(": the shell exited ").Int(int64(evidence.Query.ExitCode)).
			Str(", which means no such command, or a file that is not executable")
	}
	if evidence.Query.Cause != "" {
		reason.Str(" (").Str(evidence.Query.Cause).Str(")")
	}
	fix.Str("set the run string to a program that exists and is executable")
	if evidence.Target.WorkDir != "" {
		fix.Str("; a relative path is read from ").Str(evidence.Target.WorkDir)
	}
	return []Row{{
		Check: check, Stage: stageDeclare, Verdict: VerdictFail,
		Reason: reason.String(), Fix: fix.String(),
	}}
}

// exitsClean judges that the program exits 0 within the budget and leaves
// nothing running.
func exitsClean(evidence *Evidence) []Row {
	const check = "exits-clean"
	row := Row{Check: check, Stage: stageDeclare, Verdict: VerdictFail}

	var tb textbuf.Buffer
	switch evidence.Query.Exit {
	case plugin.QueryExitClean:
		row.Verdict = VerdictPass
	case plugin.QueryExitNotStarted:
		return notStartedRow(check, stageDeclare)
	case plugin.QueryExitFailed:
		row.Reason = tb.Str("the program exited ").Int(int64(evidence.Query.ExitCode)).
			Str(" in declare mode, and a query must exit 0").String()
		row.Fix = "exit 0 after writing the declaration line: " + fixUseRunOrDeclare
	case plugin.QueryExitTimeout:
		row.Reason = tb.Str("the program was still running after ").Str(evidence.Budget.String()).
			Str(" and was stopped, so it started for real or waits on something").String()
		row.Fix = fixUseRunOrDeclare
	case plugin.QueryExitCanceled:
		row.Verdict = VerdictNotJudged
		row.Reason = "the caller stopped the declaration query, so conformance was not judged"
		row.Fix = "retry the conformance check with a caller that can wait for completion"
	case plugin.QueryExitOutlived:
		row.Reason = "the program exited 0 and a process it started still held its standard output"
		row.Fix = "start no process in declare mode: move the work that starts it into the " +
			"start function, which a query never calls"
	case plugin.QueryExitUnspecified:
		panic("BUG: conformance: plugin.QueryDeclaration answered a run with no exit")
	}
	return []Row{row}
}

// declarationLine judges that standard output is exactly the declaration
// line, in strict form, and names the rule the output broke.
func declarationLine(evidence *Evidence) []Row {
	const check = "declaration-line"
	if !started(evidence) {
		return notStartedRow(check, stageDeclare)
	}
	_, broken := strictLine(evidence.Query.Stdout)
	if broken == "" {
		return []Row{{Check: check, Stage: stageDeclare, Verdict: VerdictPass}}
	}
	var fix textbuf.Buffer
	return []Row{{
		Check: check, Stage: stageDeclare, Verdict: VerdictFail,
		Reason: broken,
		Fix: fix.Str("write exactly one line to standard output, #").Uint(declarationID).Str(" ").
			Str(rpc.MethodDeclareRegistration).Str(" and the declaration as compact JSON, ended by one line feed; ").
			Str("send every other text to standard error").String(),
	}}
}

// strictLine judges stdout against the strict form of the declaration line
// and answers its JSON payload. broken names the first rule the output broke,
// and is empty when the output is exactly the line.
//
// The strict form is narrower than what `show plugin declarations` reads:
// that reader skips a banner or a log line around the declaration. A
// conforming plugin writes the line alone, so a reader never has to.
func strictLine(stdout []byte) (payload []byte, broken string) {
	var tb textbuf.Buffer
	if len(stdout) == 0 {
		return nil, "the program wrote nothing to standard output"
	}
	if bytes.IndexByte(stdout, '\r') >= 0 {
		return nil, "standard output holds a carriage return (CR): the line ends with a line feed (LF) alone"
	}
	if stdout[len(stdout)-1] != '\n' {
		return nil, "the last line on standard output does not end with a line feed (LF)"
	}
	line := stdout[:len(stdout)-1]
	if extra := bytes.Count(line, []byte{'\n'}); extra > 0 {
		return nil, tb.Str("standard output holds ").Int(int64(extra + 1)).
			Str(" lines, and the declaration is the only line a plugin writes there").String()
	}
	id, method, payload, err := rpc.ParseLine(line)
	if err != nil {
		return nil, tb.Str("the line is not in the form #<id> <method> <json>: ").Err(err).String()
	}
	if id != declarationID {
		return nil, tb.Str("the request id is #").Uint(id).
			Str(", and the declaration is request #").Uint(declarationID).String()
	}
	if method != rpc.MethodDeclareRegistration {
		return nil, tb.Str("the method is ").Quoted(method).Str(", and the declaration is ").
			Str(rpc.MethodDeclareRegistration).String()
	}
	if !json.Valid(payload) {
		return nil, "the text after the method is not valid JSON"
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, payload); err != nil {
		return nil, tb.Str("the text after the method is not valid JSON: ").Err(err).String()
	}
	if !bytes.Equal(compact.Bytes(), payload) {
		return nil, "the JSON is not compact: it holds white space between its tokens"
	}
	return payload, ""
}

// declarationDecodes judges that the JSON of the line decodes as a Stage 1
// declaration.
func declarationDecodes(evidence *Evidence) []Row {
	const check = "declaration-decodes"
	if !started(evidence) {
		return notStartedRow(check, stageDeclare)
	}
	row := Row{Check: check, Stage: stageDeclare, Verdict: VerdictFail}
	row.Fix = "make the JSON one object with the fields of the Stage 1 declaration (" +
		protocolPage + ", \"Stage 1: Registration (Plugin to Engine)\")"

	// The strict line names a decode error exactly. Output that is not the
	// strict line may still hold a declaration the lenient reader found.
	if payload, broken := strictLine(evidence.Query.Stdout); broken == "" {
		var declaration rpc.DeclareRegistrationInput
		if err := json.Unmarshal(payload, &declaration); err != nil {
			var tb textbuf.Buffer
			row.Reason = tb.Str("the JSON does not decode as a declaration: ").Err(err).String()
			return []Row{row}
		}
	}
	if !evidence.Query.Declared {
		row.Reason = "no " + rpc.MethodDeclareRegistration + " line that decodes was read from standard output"
		return []Row{row}
	}
	return []Row{{Check: check, Stage: stageDeclare, Verdict: VerdictPass}}
}

// stage1Rows answers one row for each check of the daemon's Stage 1 table
// that needs no daemon state. A declaration nobody read leaves each row
// not-judged, under the same names.
func stage1Rows(evidence *Evidence) []Row {
	verdicts := evidence.Judgement.Verdicts
	rows := make([]Row, 0, len(verdicts))
	for i := range verdicts {
		verdict := &verdicts[i]
		row := Row{Check: verdict.Check, Stage: stageOne, Verdict: VerdictPass}
		if verdict.KeepsPlugin {
			row.Stage = stageFive
		}
		rows = append(rows, judgedRow(evidence, verdict, row))
	}
	return rows
}

func judgedRow(evidence *Evidence, verdict *plugin.DeclarationVerdict, row Row) Row {
	if !evidence.Query.Declared {
		row.Verdict = VerdictNotJudged
		row.Reason = "no declaration was read, so there is nothing to judge"
		row.Fix = "fix the declaration-line and declaration-decodes rows first"
		return row
	}
	if verdict.Err == nil {
		return row
	}
	var fix textbuf.Buffer
	row.Verdict = VerdictFail
	row.Reason = verdict.Err.Error()
	row.Fix = fix.Str("change the declaration the plugin writes so the daemon accepts it: the rule is in ").
		Str(protocolPage).Str(", ").Quoted(verdict.Section).String()
	return row
}

// daemonStateRows answers one not-judged row for each Stage 1 check that
// needs a running daemon.
func daemonStateRows(evidence *Evidence) []Row {
	unjudged := evidence.Judgement.Unjudged
	rows := make([]Row, 0, len(unjudged))
	for i := range unjudged {
		var reason textbuf.Buffer
		rows = append(rows, Row{
			Check: unjudged[i].Check, Stage: stageOne, Verdict: VerdictNotJudged,
			Reason: reason.Str("only a running daemon judges this check: ").Str(unjudged[i].Reason).String(),
			Fix:    "configure the plugin with plugin { external <name> { run ...; } } and start ze: the daemon judges this check at Stage 1",
		})
	}
	return rows
}
