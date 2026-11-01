// Design: docs/features/ai-first.md — show diagnostic command
// Overview: register.go — command registration
//
// `show diagnostic` is a local-data command: Run answers the explanation as a
// value and writes nothing, so every route that calls it decides where the
// answer and the refusal go (docs/architecture/api/commands.md, "Local-data
// commands"). The text an operator reads is the drawing registered for the
// answer's envelope key, identical to what the command printed before.

package explain

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyDiagnostic is the envelope key of the answer, the key its drawing is
// registered under.
const keyDiagnostic = "diagnostic"

// answer is the explanation of one diagnostic code.
//
// asJSON records the local `--json` rendering flag. It is not part of the
// data, so JSON never carries it: the daemon route has no rendering flag, and
// its caller's `| json` renders the same fields.
type answer struct {
	Diagnostic diagnostic.ExplainResult `json:"diagnostic"`
	asJSON     bool
}

// Run answers the show diagnostic command.
func Run(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	if len(args) == 0 {
		return command.LocalRefusal(1, usage())
	}

	jsonOutput := false
	var codes []string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "help", "-h", "--help":
			return command.LocalRefusal(0, usage())
		default:
			codes = append(codes, arg)
		}
	}

	if len(codes) == 0 {
		return command.LocalRefusal(1, "error: missing diagnostic code\n"+usage())
	}
	if len(codes) > 1 {
		var tb textbuf.Buffer
		return command.LocalRefusal(1, tb.Str("error: expected one diagnostic code, got ").Int(int64(len(codes))).Byte('\n').String())
	}
	code := codes[0]

	meta := diagnostic.Lookup(code)
	if meta == nil {
		var tb textbuf.Buffer
		tb.Str("error: unknown diagnostic code: ").Str(code).Byte('\n')
		if all := diagnostic.AllCodes(); len(all) > 0 {
			tb.Str("valid codes: ").Join(all, ", ").Byte('\n')
		}
		return command.LocalRefusal(1, tb.String())
	}

	return command.LocalAnswer{Payload: answer{
		Diagnostic: diagnostic.ExplainResult{
			Code:         meta.Code,
			Title:        meta.Title,
			Description:  meta.Description,
			Examples:     meta.Examples,
			RelatedCodes: meta.RelatedCodes,
		},
		asJSON: jsonOutput,
	}}
}

// render prints Run's answer on the local route. `--json` prints the
// explanation as indented JSON, as it always did; every other answer goes
// through the configured default format, whose text form is drawText.
func render(path string, payload any) int {
	explained, ok := payload.(answer)
	if !ok || !explained.asJSON {
		return command.RenderLocalAnswer(path, payload)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(explained.Diagnostic); err != nil {
		var tb textbuf.Buffer
		os.Stderr.WriteString(tb.Str("error: ").Err(err).Byte('\n').String()) //nolint:errcheck // CLI diagnostic
		return 1
	}
	os.Stdout.Write(out.Bytes()) //nolint:errcheck // CLI output
	return 0
}

// drawText draws an explanation as the text the command printed: the code and
// title, the description, then the examples and related codes when present.
func drawText(raw json.RawMessage) (string, error) {
	var meta diagnostic.ExplainResult
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "", err
	}
	var b textbuf.Buffer
	b.Str(meta.Code).Str(": ").Str(meta.Title).Byte('\n').Byte('\n')
	b.Str(meta.Description).Byte('\n')

	if len(meta.Examples) > 0 {
		b.Byte('\n').Str("Examples:\n")
		for _, ex := range meta.Examples {
			b.Str("  ").Str(ex).Byte('\n')
		}
	}
	if len(meta.RelatedCodes) > 0 {
		b.Byte('\n').Str("Related: ").Join(meta.RelatedCodes, ", ").Byte('\n')
	}
	return b.String(), nil
}

// usage answers the help page, colored when stderr, where the local route
// prints it, is a terminal.
func usage() string {
	p := helpfmt.Page{
		Command:   "ze show diagnostic",
		ShortHelp: "Explain a diagnostic code emitted by ze config validate",
		Usage:     []string{"ze show diagnostic [--json] <diagnostic-code>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--json", Desc: "Output structured JSON explanation"},
			}},
		},
		Examples: []string{
			"ze show diagnostic config-parse",
			"ze show diagnostic --json config-yang-type",
		},
	}
	var help bytes.Buffer
	p.WriteTo(&help, slogutil.UseColor(os.Stderr))
	return help.String()
}
