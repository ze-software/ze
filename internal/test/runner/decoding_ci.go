// Design: docs/architecture/testing/ci-format.md — the decode suite's .ci reader
// Overview: decoding.go — DecodingTests, which discovers and runs these files

package runner

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// decodeProgram is the command a decode .ci names. The suite runs the exec=
// line as written, with "ze" replaced by the binary under test, so a line
// naming another program is refused: it belongs to another suite.
const decodeProgram = "ze bgp decode"

// flagPlugin is ze's --plugin flag, which the decode command a legacy decode
// file builds and the daemon argument scanner both name.
const flagPlugin = "--plugin"

// decodeCIState is what a decode .ci declares, collected line by line.
type decodeCIState struct {
	msgType, family, hexPayload, expectedJSON string
	cmdLine                                   string
	cmdLineNum                                int
	stdinBlocks                               map[string]string // name -> hex content
}

// decodeCIDirective is one line kind the decode reader knows: the prefix that
// selects it, and its parser.
type decodeCIDirective struct {
	prefix string
	parse  func(st *decodeCIState, rest string, lineNum int) error
}

// decodeCIDirectives is every line a decode .ci may carry. A line no entry
// matches is refused with the prefixes listed here, rather than skipped: the
// skip is how a misspelled directive used to vanish from the suite.
var decodeCIDirectives = []decodeCIDirective{
	{prefix: "cmd=", parse: parseDecodeCmdDirective},
	{prefix: "decode=", parse: parseDecodeLegacyDirective},
	{prefix: "expect=json:", parse: parseDecodeExpectDirective},
	{prefix: "stdin=", parse: parseDecodeStdinDirective},
}

// parseCIFile parses a .ci file with stdin=, cmd=, and expect= lines.
// Format:
//
//	stdin=payload:hex=<hex-payload>
//	cmd=foreground:seq=1:exec=ze bgp decode --family <family> -:stdin=payload
//	expect=json:json=<expected-json>
//
// Legacy format (still supported):
//
//	decode=<type>:family=<family>:hex=<hex-payload>
//	expect=json:json=<expected-json>
//
// Every refusal names the line; Discover adds the file.
func (dt *DecodingTests) parseCIFile(filePath string) (*decodingTest, error) {
	f, err := os.Open(filePath) //nolint:gosec // Test files from known directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	st := decodeCIState{stdinBlocks: make(map[string]string)}
	lineNum := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := parseDecodeCILine(&st, line, lineNum); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	name := strings.TrimSuffix(filepath.Base(filePath), ".ci")
	test := &decodingTest{Name: name, Nick: GenerateNick(name), File: filePath}
	switch {
	case st.cmdLine != "":
		if st.hexPayload != "" {
			return nil, fmt.Errorf("line %d: cmd= and decode= both name the payload; keep one", st.cmdLineNum)
		}
		execText, args, hex, err := parseDecodeCmdLine(st.cmdLine, st.stdinBlocks)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", st.cmdLineNum, err)
		}
		test.Label = execText
		test.Args = args
		test.HexPayload = hex
		// The block is piped as one line: `ze bgp decode -` reads one hex
		// message for each line of standard input.
		test.Stdin = hex + "\n"
	case st.hexPayload != "":
		if st.msgType == "" {
			st.msgType = msgTypeUpdate // decode= with an empty type has always meant update
		}
		args, err := legacyDecodeArgs(st.msgType, st.family, st.hexPayload)
		if err != nil {
			return nil, err
		}
		test.Label = st.msgType
		test.Args = args
		test.HexPayload = st.hexPayload
	default:
		return nil, errMissingHexPayloadUseStdinPayload
	}
	if st.expectedJSON == "" {
		return nil, errMissingExpectJsonLine
	}
	test.ExpectedJSON = st.expectedJSON
	return test, nil
}

// parseDecodeCILine dispatches one line to the directive its prefix selects.
func parseDecodeCILine(st *decodeCIState, line string, lineNum int) error {
	for _, d := range decodeCIDirectives {
		if rest, ok := strings.CutPrefix(line, d.prefix); ok {
			return d.parse(st, rest, lineNum)
		}
	}
	prefixes := make([]string, 0, len(decodeCIDirectives))
	for _, d := range decodeCIDirectives {
		prefixes = append(prefixes, d.prefix)
	}
	var b textbuf.Buffer
	b.Str("unknown directive ").Quoted(line).Str(" (the decode suite reads ").Join(prefixes, ", ").Byte(')')
	return errors.New(b.String())
}

// decodeKV splits a directive's key=value tail and refuses a key outside known.
func decodeKV(directive string, parts []string, known ...string) (map[string]string, error) {
	kv, err := parseDirectiveKV(directive, parts, known)
	if err != nil {
		return nil, err
	}
	if err := checkKeys(directive, kv, known...); err != nil {
		return nil, err
	}
	return kv, nil
}

// parseDecodeStdinDirective reads stdin=NAME:hex=HEX.
func parseDecodeStdinDirective(st *decodeCIState, rest string, _ int) error {
	parts := strings.Split(rest, ":")
	name := parts[0]
	kv, err := decodeKV("stdin="+name, parts[1:], "hex")
	if err != nil {
		return err
	}
	if kv["hex"] == "" {
		return fmt.Errorf("stdin=%s: needs hex=<payload>", name)
	}
	if _, held := st.stdinBlocks[name]; held {
		return fmt.Errorf("stdin=%s is declared twice; keep one", name)
	}
	st.stdinBlocks[name] = kv["hex"]
	return nil
}

// parseDecodeCmdDirective records the cmd= line. It is read after the whole
// file, because the stdin= block it names may follow it.
func parseDecodeCmdDirective(st *decodeCIState, rest string, lineNum int) error {
	if st.cmdLine != "" {
		return fmt.Errorf("second cmd= line (the first is line %d); the decode suite runs one command", st.cmdLineNum)
	}
	st.cmdLine = rest
	st.cmdLineNum = lineNum
	return nil
}

// parseDecodeLegacyDirective reads decode=TYPE:family=F:hex=HEX.
func parseDecodeLegacyDirective(st *decodeCIState, rest string, _ int) error {
	parts := strings.Split(rest, ":")
	kv, err := decodeKV("decode="+parts[0], parts[1:], "family", "hex")
	if err != nil {
		return err
	}
	st.msgType = strings.ToLower(parts[0])
	st.family = kv["family"]
	st.hexPayload = kv["hex"]
	return nil
}

// parseDecodeExpectDirective reads expect=json:json=JSON.
func parseDecodeExpectDirective(st *decodeCIState, rest string, _ int) error {
	kv, err := decodeKV("expect=json", strings.Split(rest, ":"), "json")
	if err != nil {
		return err
	}
	if st.expectedJSON != "" {
		return errors.New("second expect=json: line; the decode suite compares one document")
	}
	st.expectedJSON = kv["json"]
	return nil
}

// parseDecodeCmdLine reads the tail of a cmd= line (the text after "cmd=")
// into the exec= text, the argv to run after the program, and the hex payload
// of the stdin= block it names.
// Format: foreground:seq=1:exec=ze bgp decode <flags> -:stdin=payload.
//
// The words of exec= are not interpreted here: the suite runs them, so a flag
// ze does not know fails the test the way it fails an operator. The reader used
// to lift the flags off this line and run an argv of its own, so the command
// text was a comment the suite never executed.
func parseDecodeCmdLine(cmdLine string, stdinBlocks map[string]string) (execText string, args []string, hex string, err error) {
	parts := strings.Split(cmdLine, ":")
	if parts[0] != modeForeground {
		return "", nil, "", fmt.Errorf("cmd=%s: the decode suite runs cmd=foreground only", parts[0])
	}
	kv, err := decodeKV("cmd=foreground", parts[1:], "seq", "exec", "stdin")
	if err != nil {
		return "", nil, "", err
	}

	execText = kv["exec"]
	if !strings.HasPrefix(execText, decodeProgram+" ") {
		return "", nil, "", fmt.Errorf("cmd=foreground: exec=%q does not run %s, the only program the decode suite runs", execText, decodeProgram)
	}
	argv, err := splitCommand(execText)
	if err != nil {
		return "", nil, "", fmt.Errorf("cmd=foreground: exec=%q: %w", execText, err)
	}

	stdinRef := kv["stdin"]
	if stdinRef == "" {
		return "", nil, "", errors.New("cmd=foreground: needs stdin=<name> naming the stdin= block to decode")
	}
	hex, held := stdinBlocks[stdinRef]
	if !held {
		return "", nil, "", fmt.Errorf("cmd=foreground: stdin=%q names no stdin= block in this file", stdinRef)
	}
	return execText, argv[1:], hex, nil
}

// legacyDecodeArgs builds the argv a .test file or a decode= line asks for.
// Those formats name a message type and a family rather than a command, so
// the suite writes the command for them; a type it has no flag for is refused.
func legacyDecodeArgs(msgType, family, hex string) ([]string, error) {
	args := []string{"bgp", "decode"}
	switch msgType {
	case msgTypeOpen:
		args = append(args, "--open")
	case msgTypeNLRI:
		if family == "" {
			family = "unknown/unknown" // --nlri takes the family as its value
		}
		args = append(args, "--nlri", family)
	case msgTypeUpdate:
		args = append(args, "--update")
		if family != "" {
			args = append(args, "-f", family)
		}
	case msgTypeNotification:
		args = append(args, "--notification")
	case msgTypeKeepalive:
		args = append(args, "--keepalive")
	default:
		return nil, fmt.Errorf("unknown message type %q (accepts %s, %s, %s, %s, %s)",
			msgType, msgTypeKeepalive, msgTypeNLRI, msgTypeNotification, msgTypeOpen, msgTypeUpdate)
	}
	return append(args, hex), nil
}
