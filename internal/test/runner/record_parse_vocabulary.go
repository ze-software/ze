// Design: docs/architecture/testing/ci-format.md -- the .ci directive vocabulary
// Overview: record_parse.go -- the switches these lists gate
// Related: record_parse_keys.go -- the same refusal, one level down, for a directive's keys

package runner

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/ci"
)

// The directive type tokens a switch case and a vocabulary list both name.
// Spelling one of them twice is what goconst reports, and the constant is the
// better answer here for the same reason the lists exist: the case label and
// the list entry are then the SAME declaration. directiveTypeBGP
// (record_parse.go) is the first of them.
const (
	directiveTypeFile   = "file"
	directiveTypeKey    = "key"
	directiveTypeOpen   = "open"
	directiveTypeStderr = "stderr"
	directiveTypeStdout = "stdout"
	directiveTypeSyslog = "syslog"
)

// The directive words left of the '=' that a switch case, a vocabulary list and
// a family table all name.
const (
	directiveAction = "action"
	directiveAwait  = "await"
	directiveCmd    = "cmd"
	directiveExpect = "expect"
	directiveOption = "option"
	directiveReject = "reject"
)

// The keys more than one family entry names. The entry's key list and the
// parser's map read are then the same declaration.
const (
	keyConn  = "conn"
	keyGroup = "group"
	keyPeer  = "peer"
	keySeq   = "seq"
	keyValue = "value"
	keyVar   = "var"
)

// The vocabulary of the .ci directives record_parse.go reads, declared once per
// switch. Each list GATES its parser: a word the list does not carry is refused
// before the switch runs, and the refusal prints the same slice the gate read.
// So the list is what the runner accepts rather than a description of it, and
// neither drift direction is silent (ai/rules/principles.md). Add a case
// without adding its word here and the directive stops parsing, loudly, in
// every .ci that writes it. Add the word without a case and the parser answers
// errDirectiveUnlisted.
var (
	// recordActions is the action= word, the part before the first ':'.
	// expect=output:, expect=stream: and expect=command-error: never reach the
	// switch: parseLine intercepts them ahead of the ':' split because their
	// contains= needle may itself hold a ':'. They are expect types, so they are
	// named in recordExpectTypes rather than here.
	recordActions = []string{
		directiveAction, directiveAwait, directiveCmd, engineActionCommand, directiveExpect, "http",
		directiveOption, directiveReject, engineActionStream,
	}

	// recordOptionTypes is derived from recordOptionArms, so the refusal lists
	// exactly the types an arm parses.
	recordOptionTypes = armNames(recordOptionArms)

	// recordExpectTypes carries the three intercepted spellings as well, because
	// an author reading this refusal is asking what they may write, and those
	// three are writable. parseExpect gates on recordExpectTypesParsed, the arms
	// it holds itself.
	recordExpectTypes = slices.Concat(recordExpectTypesParsed, recordExpectTypesRaw)

	// recordExpectTypesParsed are the expect types parseExpect reads from a
	// key=value map.
	recordExpectTypesParsed = []string{
		directiveTypeBGP, "event", "exit", directiveTypeFile, directiveTypeKey, "json",
		directiveTypeStderr, directiveTypeStdout, directiveTypeSyslog,
	}

	// recordExpectTypesRaw are the expect types parseLine reads whole, before
	// the ':' split, so their value keeps every colon the author wrote.
	recordExpectTypesRaw = []string{"command-error", "output", engineActionStream}

	recordRejectTypes = []string{directiveTypeBGP, directiveTypeStderr, directiveTypeStdout, directiveTypeSyslog}

	recordActionTypes = armNames(recordActionArms)

	// recordCmdTypes joins the two cmd= grammars: cmd=api reads a key=value
	// map, and the program arms read the raw line by marker (parseCmdExec).
	recordCmdTypes = append(armNames(recordCmdArms), modeBackground, modeForeground, modeStop)

	recordAwaitTypes = armNames(recordAwaitArms)

	// recordHTTPMethods are the http= words parseHTTP reads. wait polls with
	// GET until the condition holds.
	recordHTTPMethods = []string{"get", "post", "wait"}
)

// The key=value directive families, one table each. A type's entry is the
// whole of what the runner knows about it: the keys its parser reads and the
// parser. The dispatcher refuses an unknown type with the table's names and an
// unknown key with the entry's keys, both before the parser runs, so a parser
// cannot read a key its entry does not list without every .ci that writes the
// key going red (ai/rules/principles.md, one declaration).
var (
	recordOptionArms = map[string]optionArm{
		"asn":             {keys: []string{keyValue}, parse: parseOptionAsn},
		"bind":            {keys: []string{keyValue}, parse: parseOptionBind},
		"env":             {keys: []string{keyVar, keyValue}, parse: parseOptionEnv},
		"exclusive":       {keys: []string{keyGroup}, parse: parseOptionExclusive},
		directiveTypeFile: {keys: []string{"path"}, parse: parseOptionFile},
		"needs-arch":      {keys: []string{keyValue}, parse: parseOptionNeedsArch},
		"needs-linux":     {keys: []string{"caps"}, parse: parseOptionNeedsLinux},
		"needs-path":      {keys: []string{keyValue, "hint"}, parse: parseOptionNeedsPath},
		"netns-link":      {keys: []string{"name", keyPeer, "address", "vlan"}, parse: parseOptionNetnsLink},
		directiveTypeOpen: {keys: []string{keyValue}, parse: parseOptionOpen},
		"skip-env":        {keys: []string{keyVar, keyValue}, parse: parseOptionSkipEnv},
		"skip-os":         {keys: []string{keyValue}, parse: parseOptionSkipOs},
		"tcp_connections": {keys: []string{keyValue}, parse: parseOptionTcpConnections},
		"timeout":         {keys: []string{keyValue}, parse: parseOptionTimeout},
		"update":          {keys: []string{keyValue}, parse: parseOptionUpdate},
	}

	recordActionArms = map[string]kvArm{
		"notification": {keys: []string{keyConn, keySeq, "text"}, parse: parseActionNotification},
		"rewrite":      {keys: []string{keyConn, keySeq, "source", "dest"}, parse: parseActionRewrite},
		"send":         {keys: []string{keyConn, keySeq, "hex"}, parse: parseActionSend},
		"sighup":       {keys: []string{keyConn, keySeq}, parse: parseActionSighup},
		"sigterm":      {keys: []string{keyConn, keySeq}, parse: parseActionSigterm},
	}

	recordCmdArms = map[string]kvArm{
		"api": {keys: []string{keyConn, keySeq, "text"}, parse: parseCmdAPI},
	}

	recordAwaitArms = map[string]kvArm{
		directiveTypeStderr: {keys: []string{"contains", "timeout", "then"}, parse: parseAwaitStderr},
	}
)

// optionArm is one option= type: the keys its parser reads, and the parser.
// The parser takes the .ci path because option=file and option=needs-path
// resolve a path against it.
type optionArm struct {
	keys  []string
	parse func(r *Record, ciFile string, kv map[string]string) error
}

// kvArm is one type of a key=value directive family that needs no path.
type kvArm struct {
	keys  []string
	parse func(r *Record, kv map[string]string) error
}

// armNames answers a family's type names, sorted, for its gate and its refusal.
func armNames[A any](arms map[string]A) []string {
	return slices.Sorted(maps.Keys(arms))
}

// parseDirectiveKV splits a key=value directive's segments and names the
// directive and the keys it accepts on a refusal, so the author meets
// `option=asn: segment "65000" has no "=" ... (accepts value)` rather than a
// bare splitter error. accepted is nil where the caller's arms hold their keys
// themselves (expect=, reject=), and the refusal then lists none.
func parseDirectiveKV(directive string, parts, accepted []string) (map[string]string, error) {
	kv, err := ci.ParseKVPairs(parts)
	if err == nil {
		return kv, nil
	}
	if len(accepted) == 0 {
		return nil, fmt.Errorf("%s: %w", directive, err)
	}
	var b textbuf.Buffer
	b.Str(" (accepts ").Join(slices.Sorted(slices.Values(accepted)), ", ").Byte(')')
	return nil, fmt.Errorf("%s: %w%s", directive, err, b.String())
}

// familyKeys answers the keys the arm for action=lineType reads, from the
// family tables, or nil for a family whose arms hold their keys themselves.
func familyKeys(action, lineType string) []string {
	switch action {
	case directiveOption:
		return recordOptionArms[lineType].keys
	case directiveAction:
		return recordActionArms[lineType].keys
	case directiveAwait:
		return recordAwaitArms[lineType].keys
	case directiveCmd:
		return recordCmdArms[lineType].keys
	}
	return nil
}

// directiveTypeGate is how one directive family refuses a type it does not
// read: the kind its refusal names, the types its parser reads, and the types
// the refusal lists, which for expect= also carries the spellings parseLine
// reads whole.
type directiveTypeGate struct {
	kind     string
	parsed   []string
	accepted []string
}

// recordTypeGates holds one gate per directive family whose word selects a
// type. parseLine runs the gate before it splits the tail, so an unknown type
// is reported as unknown with the accepted list rather than as a malformed
// key=value segment, and each parser runs the same gate for its direct callers.
var recordTypeGates = map[string]directiveTypeGate{
	directiveAction: {kind: "action type", parsed: recordActionTypes, accepted: recordActionTypes},
	directiveAwait:  {kind: "await type", parsed: recordAwaitTypes, accepted: recordAwaitTypes},
	directiveCmd:    {kind: "cmd type", parsed: recordCmdTypes, accepted: recordCmdTypes},
	directiveExpect: {kind: "expect type", parsed: recordExpectTypesParsed, accepted: recordExpectTypes},
	directiveOption: {kind: "option type", parsed: recordOptionTypes, accepted: recordOptionTypes},
	directiveReject: {kind: "reject type", parsed: recordRejectTypes, accepted: recordRejectTypes},
}

// checkDirectiveType refuses a type the family of action does not read. An
// action with no gate (http=, the engine steps) selects no typed parser here.
func checkDirectiveType(action, lineType string) error {
	gate, ok := recordTypeGates[action]
	if !ok {
		return nil
	}
	if slices.Contains(gate.parsed, lineType) {
		return nil
	}
	return unknownDirective(gate.kind, lineType, gate.accepted)
}

// unknownDirective refuses a directive word no arm of its switch reads, and
// names what the runner accepts in its place.
//
// The three facts an author needs are the word they wrote, the accepted set,
// and the line, and the line is added by parseAndAdd, which is the only caller
// that knows it. A refusal that names only the word sends its reader into the
// parser to find the vocabulary, which is where the .ci format was learned by
// reading source for as long as this message existed.
func unknownDirective(kind, name string, accepted []string) error {
	sorted := slices.Clone(accepted)
	slices.Sort(sorted)

	var b textbuf.Buffer
	b.Str("unknown ").Str(kind).Byte(' ').Quoted(name).Str(" (accepts ").Join(sorted, ", ").Byte(')')
	return errors.New(b.String())
}

// errDirectiveUnlisted is the arm no author can reach: the vocabulary above
// carries the word and no case reads it, which is an edit that added half a
// directive. It fails the file rather than falling through, because a directive
// nobody parses is a directive that asserts nothing.
func errDirectiveUnlisted(kind, name string) error {
	var b textbuf.Buffer
	b.Str(kind).Byte(' ').Quoted(name).Str(" is listed as accepted and no parser reads it (BUG in the runner's vocabulary)")
	return errors.New(b.String())
}
