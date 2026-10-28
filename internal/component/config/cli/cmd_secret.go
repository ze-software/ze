// Design: docs/guide/command-reference.md — ze secret decode and encode
// Related: register.go — registers the secret root command
//
// The ze secret command reads the plaintext behind a $9$ value of a config, and
// writes the $9$ form of a plaintext. Both are one call to the secret codec, so
// the command holds only the operand handling and the operator's error text.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// secretActions maps each `ze secret` action to the codec call it makes. The
// registered Subs and the unknown-action refusal both derive from it.
var secretActions = map[string]func(string) (string, error){
	"decode": decodeSecret,
	"encode": encodeSecret,
}

// secretHashFormat matches the modular crypt prefix `$<id>$` that one-way
// password hashes carry: bcrypt ($2a$, $2b$, $2y$), SHA-crypt ($5$, $6$),
// MD5-crypt ($1$) and yescrypt ($y$). A $9$ value carries the same shape, so
// the decoder tests for $9$ before it tests for a hash.
var secretHashFormat = regexp.MustCompile(`^\$([0-9a-z]+)\$`)

// secretSubs returns the sorted, comma-separated action list for the root
// registration, derived from secretActions.
func secretSubs() string {
	actions := make([]string, 0, len(secretActions))
	for action := range secretActions {
		actions = append(actions, action)
	}
	slices.Sort(actions)
	return textbuf.Join(actions, ", ")
}

// cmdSecret runs `ze secret <action> <value|->` on the process streams.
func cmdSecret(args []string) int {
	return runSecret(args, os.Stdout, os.Stderr)
}

// runSecret runs one `ze secret` action. The answer goes to stdout on a line of
// its own, so `$(ze secret decode -)` captures exactly the value.
func runSecret(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		secretUsage(stderr)
		return exitError
	}
	action := args[0]
	if isHelpToken(action) {
		secretUsage(stdout)
		return exitOK
	}
	transform, known := secretActions[action]
	if !known {
		writeSecretLines(stderr,
			"error: unknown ze secret action "+quoteSecretToken(action)+".",
			"The actions are "+strings.ReplaceAll(secretSubs(), ", ", " and ")+":",
			"  ze secret decode '$9$...'",
			"  ze secret encode '<plaintext>'")
		return exitError
	}
	if len(args) > 1 && isHelpToken(args[1]) {
		secretUsage(stdout)
		return exitOK
	}

	value, code := readSecretOperand(action, args[1:], stderr)
	if code != exitOK {
		return code
	}
	answer, err := transform(value)
	if err != nil {
		writeSecretLines(stderr, err.Error())
		return exitInvalid
	}
	writeSecretLines(stdout, answer)
	return exitOK
}

// isHelpToken reports whether token asks for the help page.
func isHelpToken(token string) bool {
	return token == "help" || token == "-h" || token == "--help"
}

// readSecretOperand returns the one value an action works on: the argument, or
// stdin when the argument is "-". A value read from stdin loses one trailing
// line ending, the one `echo` and a here-string add.
func readSecretOperand(action string, operands []string, stderr io.Writer) (string, int) {
	if len(operands) == 0 {
		writeSecretLines(stderr,
			"error: ze secret "+action+" needs the value to "+action+".",
			"Pass it as an argument in single quotes, or pass - and write it on stdin:",
			"  ze secret "+action+" '<value>'",
			"  ze secret "+action+" -")
		return "", exitError
	}
	if len(operands) > 1 {
		writeSecretLines(stderr,
			"error: ze secret "+action+" takes one value, and it got "+textbuf.StringInt(int64(len(operands)))+".",
			"A value with spaces must be one argument. Put it in single quotes:",
			"  ze secret "+action+" '<value>'")
		return "", exitError
	}
	operand := operands[0]
	if cliio.IsStdin(operand) {
		data, err := cliio.ReadFile(operand)
		if err != nil {
			writeSecretLines(stderr,
				"error: ze secret "+action+" could not read the value from stdin: "+err.Error(),
				"Pass the value as an argument instead:",
				"  ze secret "+action+" '<value>'")
			return "", exitError
		}
		value := strings.TrimSuffix(string(data), "\n")
		return strings.TrimSuffix(value, "\r"), exitOK
	}
	if strings.HasPrefix(operand, "-") {
		writeSecretLines(stderr,
			"error: ze secret "+action+" has no option "+quoteSecretToken(operand)+".",
			"A value that starts with - reads as an option. Write it on stdin instead:",
			"  printf '%s\\n' '<value>' | ze secret "+action+" -")
		return "", exitError
	}
	return operand, exitOK
}

// decodeSecret returns the plaintext behind a $9$ value, or an error that says
// why the value cannot be decoded and what to do instead.
func decodeSecret(value string) (string, error) {
	if value == "" {
		return "", errors.New("error: the value to decode is empty.\n" +
			"Copy the whole $9$ value from the config, in single quotes:\n" +
			"  ze secret decode '$9$...'")
	}
	if !secret.IsEncoded(value) {
		return "", notEncodedError(value)
	}
	plain, err := secret.Decode(value)
	if err != nil {
		return "", errors.New("error: the value is not a valid $9$ secret: " + decodeFailureReason(err) + ".\n" +
			"It was probably cut short, or the shell changed it: an unquoted $ starts a shell variable.\n" +
			"Copy the whole value from the config again, in single quotes:\n" +
			"  ze secret decode '$9$...'")
	}
	return plain, nil
}

// notEncodedError explains a value without the $9$ prefix. A one-way password
// hash gets its own message, because no command can recover its password.
func notEncodedError(value string) error {
	if match := secretHashFormat.FindStringSubmatch(value); match != nil {
		return errors.New("error: the value is a one-way password hash ($" + match[1] + "$), not a $9$ secret.\n" +
			"A hash keeps no copy of the password, so it cannot be decoded.\n" +
			"To replace the password, set a new one:\n" +
			"  ze config set <file> <path> '<new-password>'")
	}
	return errors.New("error: the value does not start with $9$, so it is not a Ze secret.\n" +
		"Copy the whole value from the config, with its $9$ prefix, in single quotes:\n" +
		"  ze secret decode '$9$...'")
}

// decodeFailureReason names, in the operator's terms, what the codec found
// wrong with a value that carries the $9$ prefix.
func decodeFailureReason(err error) string {
	if errors.Is(err, secret.ErrInvalidChar) {
		return "it holds a character that a $9$ value never uses"
	}
	if errors.Is(err, secret.ErrTooShort) {
		return "it ends too early"
	}
	return err.Error()
}

// encodeSecret returns the $9$ form of a plaintext. A value that already
// carries the $9$ prefix is refused, because the config reads such a value as
// encoded and would never see the plaintext it was meant to hold.
func encodeSecret(value string) (string, error) {
	if value == "" {
		return "", errors.New("error: the value to encode is empty.\n" +
			"Pass the plaintext in single quotes:\n" +
			"  ze secret encode '<plaintext>'")
	}
	if secret.IsEncoded(value) {
		return "", errors.New("error: the value already starts with $9$, so it is already encoded.\n" +
			"To read its plaintext, decode it:\n" +
			"  ze secret decode '" + value + "'")
	}
	encoded, err := secret.Encode(value)
	if err != nil {
		return "", fmt.Errorf("error: ze could not encode the value, because the system random source failed.\n"+
			"Run the command again. If it keeps failing, check the host:\n"+
			"  ze doctor\n"+
			"cause: %w", err)
	}
	return encoded, nil
}

// quoteSecretToken wraps a token the operator typed in quotes for a message.
func quoteSecretToken(token string) string {
	return "\"" + token + "\""
}

// writeSecretLines writes each line followed by a newline. A failed write to a
// closed terminal has no one left to tell.
func writeSecretLines(w io.Writer, lines ...string) {
	for _, line := range lines {
		io.WriteString(w, line+"\n") //nolint:errcheck // best-effort operator output
	}
}

// secretUsage writes the `ze secret` help page.
func secretUsage(w io.Writer) {
	p := helpfmt.Page{
		Command:   "ze secret",
		ShortHelp: "Decode a $9$ secret to its plaintext, or encode a plaintext",
		Description: "A ze:sensitive leaf (a BGP MD5 password, an API token, a pre-shared key) is stored " +
			"in the reversible $9$ form. decode prints the plaintext behind a $9$ value copied " +
			"from a config. encode prints a $9$ value for a plaintext. A ze:bcrypt leaf holds a " +
			"one-way hash, which cannot be decoded. The command works offline and reads no config.",
		Usage: []string{
			"ze secret decode <value|->",
			"ze secret encode <value|->",
		},
		Sections: []helpfmt.HelpSection{
			{Title: "Actions", Entries: []helpfmt.HelpEntry{
				{Name: "decode <value|->", Desc: "Print the plaintext behind a $9$ value"},
				{Name: "encode <value|->", Desc: "Print the $9$ form of a plaintext"},
			}},
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "-", Desc: "Read the value from stdin, so it stays out of the shell history"},
			}},
			{Title: helpSectionExitCodes, Entries: []helpfmt.HelpEntry{
				{Name: "0", Desc: helpDescSuccess},
				{Name: "1", Desc: "The value cannot be decoded or encoded"},
				{Name: "2", Desc: "Usage error, or stdin could not be read"},
			}},
		},
		Examples: []string{
			"ze secret decode '$9$...'",
			"ze secret decode - < key.txt",
			"ze secret encode 'my-md5-key'",
		},
	}
	p.WriteTo(w, false)
}
