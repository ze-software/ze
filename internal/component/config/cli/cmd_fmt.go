// Design: docs/architecture/config/syntax.md — config fmt command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrOldConfig is returned when fmt is called on an old config.
var ErrOldConfig = errors.New("config needs migration")

// configFmtBytes formats config bytes and returns formatted output and whether
// changes were made. It is the whole of `ze config fmt` between reading the
// input and acting on -w, --check or --diff, so the unit tests and the command
// run one path. source names the input in a refusal: the path, or "-" for stdin.
func configFmtBytes(source string, input []byte) (string, bool, error) {
	formatted, hasChanges, _, err := formatConfigBytes(source, input)
	return formatted, hasChanges, err
}

// formatConfigBytes is configFmtBytes with the advisory weakness of each
// password it hashed, which the command reports beside its answer. It writes
// nothing: the caller decides where the warnings go.
func formatConfigBytes(source string, input []byte) (string, bool, []string, error) {
	// The set and set-meta forms are flat command lists, the form the editor
	// stores. The hierarchical parser would refuse their first word as an
	// unknown keyword, which names neither the form nor the conversion.
	if form := config.DetectFormat(string(input)); form != config.FormatHierarchical {
		return "", false, nil, setFormRefusal(source, form)
	}

	schema, err := config.YANGSchema()
	if err != nil {
		return "", false, nil, fmt.Errorf("YANG schema: %w", err)
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(string(input))
	if err != nil {
		return "", false, nil, fmt.Errorf("parse error: %w", err)
	}

	formatted, weaknesses, err := fmtTree(tree, schema)
	if err != nil {
		return "", false, nil, err
	}
	hasChanges := string(input) != formatted

	return formatted, hasChanges, weaknesses, nil
}

// fmtTree prints a parsed tree as fmt writes it, after the one transform
// every writer that persists a tree owes: a ze:bcrypt plaintext sibling
// (`plaintext-password`) is hashed into its canonical leaf. The plaintext leaf
// is ze:ephemeral, so Serialize never prints it: printing the tree unhashed
// would delete the operator's credential and lock the user out. The daemon's
// load path applies the same hash, so the credential reads back the same. It
// returns the advisory weakness of each password it hashed, which the caller
// shows, because this is the last point the plaintext is in hand.
func fmtTree(tree *config.Tree, schema *config.Schema) (string, []string, error) {
	weaknesses, err := hashPlaintextPasswords(tree, schema)
	if err != nil {
		return "", nil, err
	}
	// config.Serialize never prints a key in clear: it keeps the $9$ text a
	// key was read in, and encodes a key given in clear once.
	return config.Serialize(tree, schema), weaknesses, nil
}

// hashPlaintextPasswords is the transform every CLI writer that prints a whole
// tree to a file owes before it prints (fmt, migrate): each ze:bcrypt plaintext
// sibling (`plaintext-password`) is hashed into its canonical leaf. The
// plaintext leaf is ze:ephemeral, so both printers skip it, and a tree printed
// unhashed loses the credential. It returns the advisory weakness of each
// password it hashed, for the caller to show.
func hashPlaintextPasswords(tree *config.Tree, schema *config.Schema) ([]string, error) {
	hashed, err := config.ApplyPasswordHashing(tree, schema)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return config.PasswordWeaknessWarnings(hashed), nil
}

// printPasswordWeaknesses shows on stderr each weakness hashPlaintextPasswords
// returned, the last point the writer still holds the plaintext.
func printPasswordWeaknesses(weaknesses []string) {
	var b textbuf.Buffer
	writePasswordWeaknesses(&b, weaknesses)
	b.StdErr() //nolint:errcheck // terminal output, advisory
}

// writePasswordWeaknesses writes one warning line for each weakness.
func writePasswordWeaknesses(diag *textbuf.Buffer, weaknesses []string) {
	for _, weakness := range weaknesses {
		diag.Str("warning: ").Str(weakness).Byte('\n')
	}
}

// setFormRefusal names the flat form fmt met, why fmt cannot format it, and
// the command that converts it to the hierarchical form fmt formats.
func setFormRefusal(source string, form config.ConfigFormat) error {
	formName := "set"
	if form == config.FormatSetMeta {
		formName = "set-meta"
	}
	var b textbuf.Buffer
	return errors.New(b.Str(source).Str(" is written in the ").Str(formName).
		Str(" form, and ze config fmt formats only the hierarchical form.\n").
		Str("Convert it to the hierarchical form first:\n  ze config migrate ").
		Str(formKeyword).Str(" ").Str(formHierarchical).Str(" ").Str(source).String())
}

// cmdFmt is `ze config fmt`: the answer of `show config fmt`, printed.
func cmdFmt(args []string) int {
	return command.PrintLocalAnswer("show config fmt", fmtAnswer(args))
}

// fmtHelpPage is the help page of `ze config fmt`.
func fmtHelpPage() *helpfmt.Page {
	return &helpfmt.Page{
		Command:   "ze config fmt",
		ShortHelp: "Format and normalize configuration file",
		Usage:     []string{"ze config fmt [options] <config-file>"},
		Sections: []helpfmt.HelpSection{
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "-w", Desc: "Write result to source file"},
				{Name: "--check", Desc: "Check if formatting needed (exit 1 if changes)"},
				{Name: "--diff", Desc: "Show unified diff of changes"},
			}},
			{Title: helpSectionExitCodes, Entries: []helpfmt.HelpEntry{
				{Name: "0", Desc: "Success (or no changes needed with --check)"},
				{Name: "1", Desc: "Changes needed (with --check)"},
				{Name: "2", Desc: "Error (file not found, parse error, old config detected)"},
			}},
		},
		Examples: []string{
			"ze config fmt config.conf          Print formatted config to stdout",
			"ze config fmt -w config.conf       Write back to file",
			"ze config fmt --check config.conf  Check if formatting needed (for CI)",
			"ze config fmt --diff config.conf   Show what would change",
			"ze config fmt -                    Read from stdin, write to stdout",
		},
	}
}

// writeFmtUsage writes the help page and the note that follows it. The page is
// colored when stderr, where the local routes print it, is a terminal.
func writeFmtUsage(diag *textbuf.Buffer) {
	fmtHelpPage().WriteTo(diag, slogutil.UseColor(os.Stderr))
	diag.Str("\nFormats current config files only. For old configs, run \"ze config migrate\" first.\n")
}

// fmtAnswer answers `show config fmt`: the formatted config as a Blob, and
// every line the command reports (the help page, a refusal, a password
// warning, the --check verdict, the --diff hunks) in Diagnostic. It writes
// nothing itself (command.LocalDataHandler); with -w it writes the config file
// it was given, which is what -w asks for.
func fmtAnswer(args []string) command.LocalAnswer {
	var diag textbuf.Buffer
	fs := flag.NewFlagSet("config fmt", flag.ContinueOnError)
	fs.SetOutput(&diag)
	write := fs.Bool("w", false, "write result to source file")
	check := fs.Bool("check", false, "check if formatting needed (exit 1 if changes)")
	diff := fs.Bool("diff", false, "show unified diff of changes")
	fs.Usage = func() { writeFmtUsage(&diag) }

	// The flag package wrote the help page, or the flag error and the page,
	// into diag. `-h` asked for the page, so it succeeds as it always did.
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return command.LocalRefusal(exitOK, diag.String())
		}
		return command.LocalRefusal(exitError, diag.String())
	}

	if fs.NArg() < 1 {
		diag.Str("error: missing config file (use - for stdin)\n")
		writeFmtUsage(&diag)
		return command.LocalRefusal(exitError, diag.String())
	}

	configPath := fs.Arg(0)

	input, err := cliio.ReadFile(configPath)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}

	formatted, hasChanges, weaknesses, err := formatConfigBytes(configPath, input)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	writePasswordWeaknesses(&diag, weaknesses)

	if *check {
		if !hasChanges {
			return command.LocalAnswer{Code: exitOK, Diagnostic: diag.String()}
		}
		if configPath == "-" {
			diag.Str("stdin needs formatting\n")
		} else {
			diag.Str(configPath).Str(" needs formatting\n")
		}
		return command.LocalRefusal(1, diag.String())
	}

	if *diff {
		if hasChanges {
			writeFmtDiff(&diag, configPath, string(input), formatted)
		}
		return command.LocalAnswer{Code: exitOK, Diagnostic: diag.String()}
	}

	if *write {
		return fmtWriteAnswer(&diag, configPath, formatted, hasChanges)
	}

	return command.LocalAnswer{Payload: command.Blob(formatted), Code: exitOK, Diagnostic: diag.String()}
}

// fmtWriteAnswer answers `fmt -w`. `fmt -w -` answers the formatted config on
// stdout (a pipeline stage), always, even when the input was already
// formatted. A named file is rewritten only when formatting changed it.
func fmtWriteAnswer(diag *textbuf.Buffer, configPath, formatted string, hasChanges bool) command.LocalAnswer {
	if cliio.IsStdin(configPath) {
		return command.LocalAnswer{Payload: command.Blob(formatted), Code: exitOK, Diagnostic: diag.String()}
	}
	if !hasChanges {
		return command.LocalAnswer{Code: exitOK, Diagnostic: diag.String()}
	}
	if err := cliio.WriteFile(configPath, []byte(formatted), 0o600); err != nil {
		diag.Str(command.LocalErrorLine(err))
		return command.LocalRefusal(exitError, diag.String())
	}
	diag.Str("Formatted: ").Str(configPath).Byte('\n')
	return command.LocalAnswer{Code: exitOK, Diagnostic: diag.String()}
}

// writeFmtDiff writes the hunks `fmt --diff` reports, which the local routes
// print on stderr as they always did.
func writeFmtDiff(diag *textbuf.Buffer, path, original, formatted string) {
	origLines := strings.Split(original, "\n")
	fmtLines := strings.Split(formatted, "\n")

	diag.Str("--- ").Str(path).Str(" (original)\n")
	diag.Str("+++ ").Str(path).Str(" (formatted)\n")

	maxLines := max(len(fmtLines), len(origLines))

	inHunk := false
	hunkStart := 0

	for i := range maxLines {
		origLine := ""
		fmtLine := ""
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(fmtLines) {
			fmtLine = fmtLines[i]
		}

		if origLine != fmtLine {
			if !inHunk {
				inHunk = true
				hunkStart = i
				start := max(i-3, 0)
				diag.Str("@@ -").Int(int64(start + 1)).Byte(',').Int(int64(len(origLines) - start)).
					Str(" +").Int(int64(start + 1)).Byte(',').Int(int64(len(fmtLines) - start)).Str(" @@\n")
				for j := start; j < i; j++ {
					if j < len(origLines) {
						diag.Byte(' ').Str(origLines[j]).Byte('\n')
					}
				}
			}
			if i < len(origLines) && origLine != "" {
				diag.Byte('-').Str(origLine).Byte('\n')
			}
			if i < len(fmtLines) && fmtLine != "" {
				diag.Byte('+').Str(fmtLine).Byte('\n')
			}
		} else if inHunk {
			diag.Byte(' ').Str(origLine).Byte('\n')
			if i-hunkStart > 6 {
				inHunk = false
			}
		}
	}
}
