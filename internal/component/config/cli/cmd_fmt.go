// Design: docs/architecture/config/syntax.md — config fmt command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrOldConfig is returned when fmt is called on an old config.
var ErrOldConfig = errors.New("config needs migration")

// configFmtBytes formats config bytes and returns formatted output and whether
// changes were made. It is the whole of `ze config fmt` between reading the
// input and acting on -w, --check or --diff, so the unit tests and the command
// run one path. source names the input in a refusal: the path, or "-" for stdin.
func configFmtBytes(source string, input []byte) (string, bool, error) {
	// The set and set-meta forms are flat command lists, the form the editor
	// stores. The hierarchical parser would refuse their first word as an
	// unknown keyword, which names neither the form nor the conversion.
	if form := config.DetectFormat(string(input)); form != config.FormatHierarchical {
		return "", false, setFormRefusal(source, form)
	}

	schema, err := config.YANGSchema()
	if err != nil {
		return "", false, fmt.Errorf("YANG schema: %w", err)
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(string(input))
	if err != nil {
		return "", false, fmt.Errorf("parse error: %w", err)
	}

	formatted, weaknesses, err := fmtTree(tree, schema)
	if err != nil {
		return "", false, err
	}
	printPasswordWeaknesses(weaknesses)
	hasChanges := string(input) != formatted

	return formatted, hasChanges, nil
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
	for _, weakness := range weaknesses {
		var b textbuf.Buffer
		b.Str("warning: ").Str(weakness).Byte('\n').StdErr() //nolint:errcheck // terminal output, advisory
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

func cmdFmt(args []string) int {
	fs := flag.NewFlagSet("config fmt", flag.ExitOnError)
	write := fs.Bool("w", false, "write result to source file")
	check := fs.Bool("check", false, "check if formatting needed (exit 1 if changes)")
	diff := fs.Bool("diff", false, "show unified diff of changes")

	fs.Usage = func() {
		p := helpfmt.Page{
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
		p.WriteErr()
		fmt.Fprintf(os.Stderr, "\nFormats current config files only. For old configs, run \"ze config migrate\" first.\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file (use - for stdin)\n")
		fs.Usage()
		return exitError
	}

	configPath := fs.Arg(0)

	input, err := cliio.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	formatted, hasChanges, err := configFmtBytes(configPath, input)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // terminal output, the command is failing
		return exitError
	}

	if *check {
		if hasChanges {
			if configPath == "-" {
				fmt.Fprintf(os.Stderr, "stdin needs formatting\n")
			} else {
				fmt.Fprintf(os.Stderr, "%s needs formatting\n", configPath)
			}
			return 1
		}
		return exitOK
	}

	if *diff {
		if hasChanges {
			printDiff(configPath, string(input), formatted)
		}
		return exitOK
	}

	if *write {
		if cliio.IsStdin(configPath) {
			// `fmt -w -` emits the formatted config to stdout (a pipeline stage),
			// always -- even when the input was already formatted. Routed through
			// cliio (not fmt.Print) so it shares the one stdout sink.
			if err := cliio.WriteFile(configPath, []byte(formatted), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return exitError
			}
			return exitOK
		}
		if hasChanges {
			if err := cliio.WriteFile(configPath, []byte(formatted), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return exitError
			}
			fmt.Fprintf(os.Stderr, "Formatted: %s\n", configPath)
		}
		return exitOK
	}

	fmt.Print(formatted)
	return exitOK
}

func printDiff(path, original, formatted string) {
	origLines := strings.Split(original, "\n")
	fmtLines := strings.Split(formatted, "\n")

	fmt.Fprintf(os.Stderr, "--- %s (original)\n", path)
	fmt.Fprintf(os.Stderr, "+++ %s (formatted)\n", path)

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
				fmt.Fprintf(os.Stderr, "@@ -%d,%d +%d,%d @@\n", start+1, len(origLines)-start, start+1, len(fmtLines)-start)
				for j := start; j < i; j++ {
					if j < len(origLines) {
						fmt.Fprintf(os.Stderr, " %s\n", origLines[j])
					}
				}
			}
			if i < len(origLines) && origLine != "" {
				fmt.Fprintf(os.Stderr, "-%s\n", origLine)
			}
			if i < len(fmtLines) && fmtLine != "" {
				fmt.Fprintf(os.Stderr, "+%s\n", fmtLine)
			}
		} else if inHunk {
			fmt.Fprintf(os.Stderr, " %s\n", origLine)
			if i-hunkStart > 6 {
				inHunk = false
			}
		}
	}
}
