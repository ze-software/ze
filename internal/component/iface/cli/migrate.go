// Design: docs/architecture/iface/management.md -- Interface migration CLI

package cli

import (
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/helpfmt"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Keywords an operator types before each value of `ze interface migrate`. They
// spell the same words as the migrateKeyword* constants of
// internal/component/iface/cmd, which are unexported and so cannot be shared
// with this package, and as the modifier groups container migrate declares in
// internal/component/iface/yang/ze-iface-cmd.yang.
//
// They are grammar, not flags: the daemon interprets them, and it refuses a
// flag-shaped token before any handler runs.
const (
	migrateKeywordFrom    = "from"
	migrateKeywordTo      = "to"
	migrateKeywordAddress = "address"
	migrateKeywordCreate  = "create"
	migrateKeywordTimeout = "timeout"
)

// migrateKeywords is the closed set a token is tested against before it is read
// as a keyword.
var migrateKeywords = []string{
	migrateKeywordFrom,
	migrateKeywordTo,
	migrateKeywordAddress,
	migrateKeywordCreate,
	migrateKeywordTimeout,
}

// migrateRequired lists the keywords the command cannot run without, in the
// order an operator types them.
var migrateRequired = []string{migrateKeywordFrom, migrateKeywordTo, migrateKeywordAddress}

// migrateTypes lists the interface types the destination can be created as.
var migrateTypes = []string{ifaceTypeDummy, ifaceTypeVeth, ifaceTypeBridge}

// migrateGrammar is the form every refusal quotes back, so an operator reads
// what was expected without opening the help page.
const migrateGrammar = "from <name>.<unit> to <name>.<unit> address <cidr> [create <dummy|veth|bridge>] [timeout <duration>]"

// migrateRequest holds the values `ze interface migrate` sends to the daemon.
//
// Every field is a token rather than a parsed value, because this command
// validates the operator's input and then forwards it: the daemon owns the
// meaning of each value, including the default wait when timeout is empty.
// createType and timeout are empty when the operator names neither.
type migrateRequest struct {
	from       string
	to         string
	address    string
	createType string
	timeout    string
}

// cmdMigrate handles: ze interface migrate [--user <name>] from <iface>.<unit>
// to <iface>.<unit> address <cidr> [create <type>] [timeout <duration>].
// Returns exit code.
func cmdMigrate(args []string) int {
	if slices.Contains(args, flagHelpShort) || slices.Contains(args, flagHelpLong) {
		migrateUsage()
		return 0
	}

	rest, user, code := migrateClientFlags(args)
	if code != 0 {
		return code
	}

	req, err := parseMigrateKeywords(rest)
	if err != nil {
		printMigrateError(err)
		migrateUsage()
		return 1
	}

	creds, err := sshclient.LoadCredentialsWithFlags(user)
	if err != nil {
		printMigrateError(err)
		fmt.Fprintln(os.Stderr, "hint: run 'ze init' first, or start the daemon with 'ze start'")
		return 1
	}

	result, err := sshclient.ExecCommand(creds, "request interface migrate "+req.arguments())
	if err != nil {
		printMigrateError(err)
		return 1
	}

	if result != "" {
		fmt.Println(result)
	}
	return 0
}

// printMigrateError writes one operator-facing error line to stderr.
func printMigrateError(err error) {
	var tb textbuf.Buffer
	tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
}

// migrateClientFlags parses the flags this CLIENT interprets and answers the
// tokens that belong to the daemon's grammar. A non-zero code means the caller
// must return it and go no further.
//
// Only the login name is a flag here. Every other token is grammar the daemon
// reads, so a flag left after the first keyword is refused rather than
// forwarded: Go's flag package stops at the first non-flag token, and the
// daemon rejects a flag-shaped token before any handler runs.
func migrateClientFlags(args []string) (rest []string, user string, code int) {
	fs := flag.NewFlagSet("ze interface migrate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(name, "u", "", "Short alias for --user")
	fs.Usage = migrateUsage

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, "", 0
		}
		return nil, "", 1
	}

	rest = fs.Args()
	for _, arg := range rest {
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("error: flag ").Quoted(arg).Str(" must come before the first keyword").Byte('\n').StdErr() //nolint:errcheck // CLI output
		migrateUsage()
		return nil, "", 1
	}
	return rest, *name, 0
}

// parseMigrateKeywords reads the keyword/value pairs an operator typed.
//
// It refuses an unknown keyword, a keyword with no value, and a keyword given
// twice, so a mistake costs an error here rather than a round trip to the
// daemon. The daemon reads the same grammar and refuses the same three, because
// `ze cli` reaches the command without passing through this parser.
//
// The loop is bounded by the argument count one command line carries, and it
// steps by two because a keyword and its value are one unit.
func parseMigrateKeywords(args []string) (migrateRequest, error) {
	var req migrateRequest
	seen := make(map[string]bool, len(migrateKeywords))

	for i := 0; i < len(args); i += 2 {
		keyword := args[i]
		if !slices.Contains(migrateKeywords, keyword) {
			return req, fmt.Errorf("unknown keyword %q, expected: %s", keyword, migrateGrammar)
		}
		if seen[keyword] {
			return req, fmt.Errorf("keyword %q given twice, expected: %s", keyword, migrateGrammar)
		}
		if i+1 >= len(args) {
			return req, fmt.Errorf("keyword %q has no value, expected: %s", keyword, migrateGrammar)
		}
		seen[keyword] = true

		if err := req.set(keyword, args[i+1]); err != nil {
			return req, err
		}
	}

	for _, keyword := range migrateRequired {
		if !seen[keyword] {
			return req, fmt.Errorf("keyword %q is missing, expected: %s", keyword, migrateGrammar)
		}
	}

	return req, nil
}

// set validates one keyword's value and stores the token the daemon receives.
//
// A source and a destination are normalized to <name>.<unit>, so the daemon is
// given the unit number this side already read. An address and a wait are
// stored as the operator typed them: parsing here proves the value is well
// formed, and the spelling stays the operator's.
func (r *migrateRequest) set(keyword, value string) error {
	switch keyword {
	case migrateKeywordFrom:
		name, unit, ok := parseIfaceUnit(value)
		if !ok {
			return fmt.Errorf("invalid %s value %q, expected <name>.<unit>", keyword, value)
		}
		r.from = resolveIfaceUnit(name, unit)
	case migrateKeywordTo:
		name, unit, ok := parseIfaceUnit(value)
		if !ok {
			return fmt.Errorf("invalid %s value %q, expected <name>.<unit>", keyword, value)
		}
		r.to = resolveIfaceUnit(name, unit)
	case migrateKeywordAddress:
		if _, err := netip.ParsePrefix(value); err != nil {
			return fmt.Errorf("invalid %s value %q, expected an address in CIDR form such as 10.0.0.1/24: %w", keyword, value, err)
		}
		r.address = value
	case migrateKeywordCreate:
		if !slices.Contains(migrateTypes, value) {
			return fmt.Errorf("invalid %s value %q, expected one of %s", keyword, value, textbuf.Join(migrateTypes, ", "))
		}
		r.createType = value
	case migrateKeywordTimeout:
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("invalid %s value %q, expected a number and a unit such as 30s: %w", keyword, value, err)
		}
		r.timeout = value
	}
	return nil
}

// arguments builds the keyword tail this request sends, without the command
// path. The caller writes that path as a literal so `./le ci-dispatch check`
// can hold it against the command tree.
//
// A keyword the operator did not name is left out, so the daemon applies its
// own default rather than one this side would have to keep in step with it.
func (r *migrateRequest) arguments() string {
	var tb textbuf.Buffer
	tb.Str(migrateKeywordFrom).Byte(' ').Str(r.from)
	tb.Byte(' ').Str(migrateKeywordTo).Byte(' ').Str(r.to)
	tb.Byte(' ').Str(migrateKeywordAddress).Byte(' ').Str(r.address)
	if r.createType != "" {
		tb.Byte(' ').Str(migrateKeywordCreate).Byte(' ').Str(r.createType)
	}
	if r.timeout != "" {
		tb.Byte(' ').Str(migrateKeywordTimeout).Byte(' ').Str(r.timeout)
	}
	return tb.String()
}

// parseIfaceUnit splits "<name>.<unit>" into name and unit number.
// Returns false if the format is invalid.
func parseIfaceUnit(s string) (string, int, bool) {
	idx := strings.LastIndex(s, ".")
	if idx <= 0 || idx == len(s)-1 {
		return "", 0, false
	}

	name := s[:idx]
	unitStr := s[idx+1:]

	unit, err := strconv.Atoi(unitStr)
	if err != nil || unit < 0 {
		return "", 0, false
	}

	return name, unit, true
}

// resolveIfaceUnit joins a name and a unit number back into "<name>.<unit>".
func resolveIfaceUnit(name string, unit int) string {
	var tb textbuf.Buffer
	return tb.Str(name).Byte('.').Int(int64(unit)).String()
}

func migrateUsage() {
	var tb textbuf.Buffer
	p := helpfmt.Page{
		Command: "ze interface migrate",
		Summary: "Perform a make-before-break IP migration between interfaces",
		Usage:   []string{tb.Str("ze interface migrate [--user <name>] ").Str(migrateGrammar).String()},
		Sections: []helpfmt.HelpSection{
			{Title: "Five phases", Entries: []helpfmt.HelpEntry{
				{Name: "1.", Desc: "Create the destination interface (when create is given)"},
				{Name: "2.", Desc: "Add the address to the destination unit"},
				{Name: "3.", Desc: "Wait for BGP readiness on the new address"},
				{Name: "4.", Desc: "Remove the address from the source unit"},
				{Name: "5.", Desc: "Clean up the source interface (when Ze manages it)"},
			}},
			{Title: "Required keywords", Entries: []helpfmt.HelpEntry{
				{Name: "from <iface>.<unit>", Desc: "Source interface and unit (for example, eth0.0)"},
				{Name: "to <iface>.<unit>", Desc: "Destination interface and unit (for example, lo1.0)"},
				{Name: "address <cidr>", Desc: "Address to move (for example, 10.0.0.1/24)"},
			}},
			{Title: "Optional keywords", Entries: []helpfmt.HelpEntry{
				{Name: "create <type>", Desc: "Create the destination interface: dummy, veth, or bridge"},
				{Name: "timeout <duration>", Desc: "BGP readiness wait (default: 30s)"},
			}},
			{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
				{Name: "--user <name>", Desc: "SSH login username; it comes before the first keyword"},
			}},
		},
		Examples: []string{
			"ze interface migrate from eth0.0 to lo1.0 address 10.0.0.1/24",
			"ze interface migrate from eth0.0 to lo1.0 address 10.0.0.1/24 create dummy",
			"ze interface migrate from eth0.100 to lo2.0 address fd00::1/64 timeout 60s",
		},
	}
	p.WriteErr()
}
