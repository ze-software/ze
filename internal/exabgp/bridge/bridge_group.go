// Design: docs/architecture/exabgp-bridge.md -- a group is applied at its end
// Related: bridge_lines.go -- ScriptLines.Batch, which reads every line through Group.read
// Related: bridge_attribute.go -- bridgeRouteAttrs, takeAttrValue, which give an attribute its extent

package bridge

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// groupCommandsMax and groupOctetsMax are ExaBGP's MAX_GROUP_COMMANDS and
// MAX_GROUP_BYTES (src/exabgp/reactor/api/command/group.py at the pin). A
// group is closed only by the script writing `group end`, so a script that
// never does would otherwise grow the buffer until the daemon runs out of
// memory. Past either bound the group is discarded and the line refused.
const (
	groupCommandsMax = 100000
	groupOctetsMax   = 100 * 1024 * 1024
)

// groupWord opens every group line: `group start`, `group end`, and the
// inline `group <command> ; <command>`.
const groupWord = "group"

// peerWord opens an API 6 line that names its peers: `peer <selector> ...`.
const peerWord = "peer"

// attributeWord and attributesWord are the two spellings of the group
// command whose attributes the group's routes share.
const (
	attributeWord  = "attribute"
	attributesWord = "attributes"
)

// groupedRoutes are the kinds of announce and withdraw a group applies:
// ExaBGP's GROUPED_ROUTES (src/exabgp/reactor/api/__init__.py). An End-of-RIB
// or a route-refresh in API 6 is sent as it comes, and in a group's own
// commands it cannot be parsed as a route.
var groupedRoutes = map[string]struct{}{
	"route": {}, "ipv4": {}, "ipv6": {}, "flow": {}, "vpls": {}, attributeWord: {}, attributesWord: {},
}

// Group is one script's `group start` buffer, which ExaBGP keeps per process
// (_GROUP_BUFFERS, keyed by the service name).
//
// The zero value is a script outside any group. A runner holds one per script
// and MUST reset it when the script's child exits, as ExaBGP's clear_group
// does: a new child never sends the `group end` its predecessor owed. Not safe
// for concurrent use; a script's lines are read by one goroutine.
type Group struct {
	open   bool
	held   []groupCommand
	octets int
}

// groupCommand is one command a group applies at its end. prefix is the `peer
// <selector> ` an API 6 line named its peers with, empty for a line that
// names none; body starts with the action word.
type groupCommand struct {
	prefix string
	body   string
}

// groupStep is what one line is to the group buffer.
type groupStep uint8

const (
	groupStepUnspecified groupStep = iota
	// groupStepPass is a line that is no group's: the translator reads it.
	groupStepPass
	// groupStepAnswer is a line the group answers at once with reply, then
	// done, or error when failed is set.
	groupStepAnswer
	// groupStepApply is a group that closed: its commands are applied now.
	groupStepApply
)

// groupRead is the group's answer to one line.
type groupRead struct {
	step     groupStep
	reply    []byte
	failed   bool
	commands []groupCommand
}

// read takes one line through the group buffer, as ExaBGP's API.process
// does before it dispatches: `group start` and `group end` are group
// commands; while a group is open a line starting with `announce` or
// `withdraw` is held, and so is `peer <selector> announce|withdraw <kind>`
// when the line's API version admits it (_held_by_group); `group a ; b`, with
// or without `peer <selector>` in front, is a group of its own (group_inline).
//
// asJSON is ExaBGP's use_json for the line: API 6, or a line ending in `json`.
func (g *Group) read(line string, asJSON, admitted bool) groupRead {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return groupRead{step: groupStepPass}
	}
	if fields[0] == groupWord && len(fields) > 1 {
		switch fields[1] {
		case "start":
			return g.start(asJSON)
		case "end":
			return g.end(asJSON)
		}
		// group_inline: the commands are everything after "group ".
		return groupInline("", strings.TrimSpace(trimmed[len(groupWord):]), asJSON)
	}
	if fields[0] == peerWord {
		// The selector is every word between `peer` and the verb.
		for k := 2; k < len(fields); k++ {
			if fields[k] == groupWord {
				return groupInline(peerPrefix(trimmed, k), wordsFrom(trimmed, k+1), asJSON)
			}
			if fields[k] != announceVerb && fields[k] != withdrawVerb {
				continue
			}
			if !g.open || !admitted || k+1 >= len(fields) {
				return groupRead{step: groupStepPass}
			}
			if _, grouped := groupedRoutes[fields[k+1]]; !grouped {
				return groupRead{step: groupStepPass}
			}
			return g.hold(groupCommand{prefix: peerPrefix(trimmed, k), body: wordsFrom(trimmed, k)}, asJSON)
		}
		return groupRead{step: groupStepPass}
	}
	if !g.open {
		return groupRead{step: groupStepPass}
	}
	if strings.HasPrefix(lower, announceVerb) || strings.HasPrefix(lower, withdrawVerb) {
		return g.hold(groupCommand{body: trimmed}, asJSON)
	}
	return groupRead{step: groupStepPass}
}

// start opens the group (group_start).
func (g *Group) start(asJSON bool) groupRead {
	if g.open {
		return groupRefusal("already in group block (nested groups not allowed)", asJSON)
	}
	g.open = true
	return groupRead{step: groupStepAnswer, reply: groupStatus("group started", "group started", asJSON)}
}

// end closes the group and answers its commands for applying (group_end).
func (g *Group) end(asJSON bool) groupRead {
	if !g.open {
		return groupRefusal("not in group block", asJSON)
	}
	held := g.held
	*g = Group{}
	if len(held) == 0 {
		var reply []byte
		if asJSON {
			reply = []byte(`{"status": "group ended", "commands": 0}` + "\n")
		} else {
			reply = []byte("group ended (0 commands)\n")
		}
		return groupRead{step: groupStepAnswer, reply: reply}
	}
	return groupRead{step: groupStepApply, commands: held}
}

// hold buffers one command of an open group (group_add_command). It is
// answered done alone; past ExaBGP's bounds the group is discarded.
func (g *Group) hold(command groupCommand, asJSON bool) groupRead {
	octets := len(command.prefix) + len(command.body)
	if len(g.held) >= groupCommandsMax || g.octets+octets > groupOctetsMax {
		*g = Group{}
		return groupRefusal("group buffer limit reached, group discarded", asJSON)
	}
	g.held = append(g.held, command)
	g.octets += octets
	return groupRead{step: groupStepAnswer}
}

// groupInline splits `group a ; b` into its commands, each for the peers
// prefix names (group_inline).
func groupInline(prefix, text string, asJSON bool) groupRead {
	parts := SplitCommands(text)
	if len(parts) == 0 {
		return groupRefusal("empty group", asJSON)
	}
	commands := make([]groupCommand, 0, len(parts))
	for _, part := range parts {
		commands = append(commands, groupCommand{prefix: prefix, body: part})
	}
	return groupRead{step: groupStepApply, commands: commands}
}

// peerPrefix answers the first k words of line, `peer <selector>`, with a
// trailing space, so a group command written after it names the same peers.
func peerPrefix(line string, k int) string {
	return textbuf.Join(strings.Fields(line)[:k], " ") + " "
}

// wordsFrom answers line from its k-th word on, as written.
func wordsFrom(line string, k int) string {
	return textbuf.Join(strings.Fields(line)[k:], " ")
}

// groupRefusal is a group line answered with its reason, then error.
func groupRefusal(reason string, asJSON bool) groupRead {
	var tb textbuf.Buffer
	if asJSON {
		tb.Str(`{"error": `).Str(jsonString(reason)).Str("}\n")
	} else {
		tb.Str("error: ").Str(reason).Byte('\n')
	}
	return groupRead{step: groupStepAnswer, failed: true, reply: tb.Bytes()}
}

// groupStatus is a group line's `status` answer in its two forms.
func groupStatus(status, text string, asJSON bool) []byte {
	var tb textbuf.Buffer
	if asJSON {
		tb.Str(`{"status": `).Str(jsonString(status)).Str("}\n")
	} else {
		tb.Str(text).Byte('\n')
	}
	return tb.Bytes()
}

// SplitCommands answers the commands of a `group` line: it ends a command at
// each `;` ExaBGP's lexer would end a statement with, which is one outside
// quotes and braces, read with the backslash escapes resolved
// (configuration/grammar/lexer.py split_commands at the pin). A `;` inside a
// quoted word is part of the word, so a name a peer chose cannot carry a
// command of its own. Each command is trimmed and an empty one dropped.
func SplitCommands(text string) []string {
	var commands []string
	start := 0
	var quote byte
	depth := 0
	for position := 0; position < len(text); {
		begin := position
		char, next := lexerCharacter(text, position)
		position = next
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '"' || char == '\'':
			quote = char
		case char == '{':
			depth++
		case char == '}':
			depth = max(depth-1, 0)
		case char == ';' && depth == 0:
			commands = appendCommand(commands, text[start:begin])
			start = position
		}
	}
	return appendCommand(commands, text[start:])
}

// lexerCharacter answers the character at position as the lexer reads it,
// and where the next one starts (lexer._characters). An escape is never a
// quote, a brace nor a terminator, so it answers zero for every escape but
// the ones that resolve to a character this split reads.
func lexerCharacter(text string, position int) (byte, int) {
	if text[position] != '\\' || position+1 >= len(text) {
		return text[position], position + 1
	}
	escape := text[position+1]
	if escape == 'u' {
		// `\uXXXX` resolves to a character no split reads, valid or not.
		digits := min(len(text)-position-2, 4)
		return 0, position + 2 + digits
	}
	// `\b`, `\f`, `\n`, `\r`, `\t` resolve to control characters; any other
	// escaped character is itself, so `\;` and `\"` still split and quote, as
	// ExaBGP's _ESCAPES.get(escape, escape) reads them.
	switch escape {
	case 'b', 'f', 'n', 'r', 't':
		return 0, position + 2
	}
	return escape, position + 2
}

// appendCommand appends one trimmed command, dropping an empty one.
func appendCommand(commands []string, command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return commands
	}
	return append(commands, command)
}

// groupApply is a closed group's commands, translated for the runner's
// dispatch: one BatchLine per route command, the action that counts it, and
// the reason for each command that could not be applied, in command order.
type groupApply struct {
	lines   []BatchLine
	actions []string
	index   []int // the command each line came from
	errors  []string
}

// group translates the commands of a closed group (_process_group). The first
// `attributes` command sets what every following route shares: an attribute a
// route does not write itself, and a next-hop for a route that gives none.
func (t Translator) group(commands []groupCommand, version APIVersion) groupApply {
	out := groupApply{errors: make([]string, len(commands))}
	var shared []sharedAttribute
	for i, command := range commands {
		words := strings.Fields(command.body)
		if len(words) == 0 {
			continue
		}
		action := strings.ToLower(words[0])
		switch action {
		case attributeWord, attributesWord:
			parsed, err := sharedAttributes(words[1:])
			if err != nil {
				out.errors[i] = "could not parse attributes: " + command.body
				continue
			}
			shared = parsed
		case announceVerb, withdrawVerb:
			if len(words) < 2 {
				out.errors[i] = "could not parse: " + command.body
				continue
			}
			if _, grouped := groupedRoutes[strings.ToLower(words[1])]; !grouped {
				out.errors[i] = "could not parse: " + command.body
				continue
			}
			line := command.prefix + mergeShared(words, shared)
			translation, err := t.Line(line)
			if err != nil {
				out.errors[i] = "could not parse: " + command.body
				continue
			}
			out.lines = append(out.lines, BatchLine{Text: line, Version: version, Translation: translation})
			out.actions = append(out.actions, action)
			out.index = append(out.index, i)
		default:
			out.errors[i] = "unknown action in group: " + action
		}
	}
	return out
}

// sharedAttribute is one attribute of a group's `attributes` command: the
// name it is known by and the words that write it.
type sharedAttribute struct {
	name  string
	words []string
}

// sharedAttributes reads the attributes of a group's `attributes` command, up
// to its `nlri` if it has one. Each attribute keeps its own words, with the
// extent the route parser gives it (takeAttrValue).
func sharedAttributes(words []string) ([]sharedAttribute, error) {
	var out []sharedAttribute
	for i := 0; i < len(words); {
		key := strings.ToLower(words[i])
		if key == bridgeUpdateNLRI {
			break
		}
		attr, known := bridgeRouteAttrs[key]
		if !known {
			return nil, ErrLineNotTranslated
		}
		_, next, err := takeAttrValue(attr.Arity, words, i+1)
		if err != nil {
			return nil, err
		}
		out = append(out, sharedAttribute{name: attributeIdentity(key, attr), words: words[i:next]})
		i = next
	}
	return out, nil
}

// attributeIdentity names an attribute by its ze spelling, so two ExaBGP
// spellings of one attribute (`rd`, `route-distinguisher`) are one name.
func attributeIdentity(key string, attr bridgeRouteAttr) string {
	if attr.Ze != "" {
		return attr.Ze
	}
	return key
}

// mergeShared answers a group's route command with the shared attributes it
// does not write itself, as ExaBGP's Route.with_merged_attributes keeps the
// route's own (and gives the shared next-hop to a route with none). They go
// before the `nlri` of the attributes form, and after the route otherwise.
func mergeShared(words []string, shared []sharedAttribute) string {
	if len(shared) == 0 {
		return textbuf.Join(words, " ")
	}
	written := make(map[string]bool, len(words))
	cut := len(words)
	for i, word := range words[2:] {
		key := strings.ToLower(word)
		if key == bridgeUpdateNLRI && cut == len(words) {
			cut = i + 2
		}
		if attr, known := bridgeRouteAttrs[key]; known {
			written[attributeIdentity(key, attr)] = true
		}
	}
	merged := make([]string, 0, len(words)+2*len(shared))
	merged = append(merged, words[:cut]...)
	for _, attribute := range shared {
		if !written[attribute.name] {
			merged = append(merged, attribute.words...)
		}
	}
	merged = append(merged, words[cut:]...)
	return textbuf.Join(merged, " ")
}

// answer writes a group's outcome as ExaBGP's _answer_group does: how many
// routes each action applied and, when a command could not be applied, how
// many errors (the reasons themselves in JSON). failed means the line ends
// with error.
func (a groupApply) answer(answers []BatchAnswer, asJSON bool) (reply []byte, failed bool) {
	announced, withdrawn := 0, 0
	for i, line := range a.lines {
		if answers[i].Failed {
			a.errors[a.index[i]] = "invalid route: " + answers[i].Error
			continue
		}
		routes := 0
		for _, command := range line.Translation.Commands {
			if command.Key.Route() {
				routes++
			}
		}
		if a.actions[i] == announceVerb {
			announced += routes
		} else {
			withdrawn += routes
		}
	}
	var errors []string
	for _, reason := range a.errors {
		if reason != "" {
			errors = append(errors, reason)
		}
	}

	var tb textbuf.Buffer
	if asJSON {
		tb.Str(`{"status": "group processed", "announced": `).Str(strconv.Itoa(announced)).
			Str(`, "withdrawn": `).Str(strconv.Itoa(withdrawn))
		if len(errors) > 0 {
			tb.Str(`, "errors": [`)
			for i, reason := range errors {
				if i > 0 {
					tb.Str(", ")
				}
				tb.Str(jsonString(reason))
			}
			tb.Byte(']')
		}
		tb.Str("}\n")
	} else {
		tb.Str("group processed: ").Str(strconv.Itoa(announced)).Str(" announced, ").
			Str(strconv.Itoa(withdrawn)).Str(" withdrawn")
		if len(errors) > 0 {
			tb.Str(", ").Str(strconv.Itoa(len(errors))).Str(" errors")
		}
		tb.Byte('\n')
	}
	return tb.Bytes(), len(errors) > 0
}

// answersInJSON is ExaBGP's API._answers_in_json: API 6 answers in JSON, and
// API 4 or a script still undecided only when the line ends in `json`.
func answersInJSON(version APIVersion, line string) bool {
	if version == APIV6 {
		return true
	}
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[len(fields)-1] == encoderNameJSON
}
