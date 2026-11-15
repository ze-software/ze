// Design: docs/contributing/rule-authoring.md -- the agent rule set and which agent starts with it
// Related: ruleset.go -- the parts an agent that omits the instruction files is sent or told to read
//
// agentstart.go is the one predicate that decides, from a mirrored agent
// definition, whether the agent starts without the project instruction files.
// The subagent start hook (internal/le/hookruntime) sends the agent rule set
// on its answer, and `./le ai sync check` (internal/le/ai/sync) reports, per
// definition, the payload that answer gives the agent, so the hook and the
// report never disagree about which agent starts with what. It lives here,
// under the hook source seal, because the hook acts on it and the sync check
// cannot import the hook runtime, which imports the sync check.

package ruleset

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// OmitInstructionsKey is the frontmatter key the harness reads to start a
// subagent without the project instruction files. The definitions it is read
// from are the mirror `./le ai sync update` writes (AgentDefinitionsDir).
const OmitInstructionsKey = "omitClaudeMd"

// omitInstructionsLoads is the one omitClaudeMd value read as an agent that
// loads the instruction files. Every other value, quoted, capitalised or
// unknown, is read as omitting: the rule set sent to an agent that did load the
// files costs it a few thousand characters, while nothing sent to one that did
// not leaves it with no ban.
const omitInstructionsLoads = "false"

// agentTypeLengthMax bounds an agent type joined to a path. Every definition
// name in ai/agents is far shorter.
const agentTypeLengthMax = 64

// AgentStart is what the mirror tells about how an agent of one type starts.
// Its zero value is no answer, and no reader returns it.
type AgentStart uint8

const (
	// AgentStartUnspecified is the zero value: no answer.
	AgentStartUnspecified AgentStart = iota
	// AgentStartUndefined is a type that cannot name a file in the mirror (a
	// plugin agent's scoped name, a path), or one with no definition there (a
	// built-in agent, an agent defined outside the checkout, a source not yet
	// mirrored). No start text adds a rule set for it.
	AgentStartUndefined
	// AgentStartLoads is a definition that names the agent and leaves
	// omitClaudeMd absent or a bare false: the agent loads the instruction
	// files.
	AgentStartLoads
	// AgentStartOmits is a definition that names the agent and sets
	// omitClaudeMd to any other value: the agent omits the instruction files.
	AgentStartOmits
	// AgentStartUnread is a definition at the type's path whose frontmatter
	// names no agent of that type as this reader reads it (another name,
	// frontmatter indented as a whole, no closed block). It cannot be read for
	// sure, so it takes the side that sends the rule set: read as omitting.
	AgentStartUnread
)

// Omits reports whether an agent with this start is sent the agent rule set:
// one that omits the instruction files, or one whose definition cannot be read
// for sure.
func (s AgentStart) Omits() bool {
	switch s {
	case AgentStartOmits, AgentStartUnread:
		return true
	case AgentStartLoads, AgentStartUndefined:
		return false
	case AgentStartUnspecified:
	}
	panic("BUG: ruleset: Omits asked of an AgentStart no reader answers")
}

// ReadAgentStart answers how the agent of agentType starts, read from its
// definition in the mirror under tree: the definition must name that agent in
// its frontmatter, because the harness reports a custom agent's type as its
// frontmatter name, not its file name.
//
// It sees only the mirror this checkout writes, so a type that cannot name a
// file there, and a type with no definition in it, answer AgentStartUndefined
// whether or not their own definition elsewhere sets omitClaudeMd. A
// definition that exists and cannot be read is an error: the caller cannot
// tell which start the agent had.
func ReadAgentStart(tree, agentType string) (AgentStart, error) {
	if !validAgentType(agentType) {
		return AgentStartUndefined, nil
	}
	path := filepath.Join(tree, AgentDefinitionsDir, agentType+".md")
	body, err := os.ReadFile(path) //nolint:gosec // agentType is validated as a file name above
	// refuse-by-default: guards Omits -- an undefined type gets no rule set; only definitions this checkout writes can be read
	if errors.Is(err, fs.ErrNotExist) {
		return AgentStartUndefined, nil
	}
	if err != nil {
		return AgentStartUnspecified, err
	}
	fields := FrontmatterFields(string(body))
	// A definition at the type's path whose name the reader cannot match
	// (another name, frontmatter indented as a whole, no closed block) cannot
	// be read for sure, so it takes the side that sends the rule set.
	if !slices.ContainsFunc(fields["name"], func(name string) bool { return FrontmatterUnquoted(name) == agentType }) {
		return AgentStartUnread, nil
	}
	if FrontmatterOmits(fields[OmitInstructionsKey]) {
		return AgentStartOmits, nil
	}
	return AgentStartLoads, nil
}

// FrontmatterOmits reports whether the omitClaudeMd values a definition sets
// read as an agent that omits the instruction files: any occurrence other
// than the bare loading value. A repeated key cannot be read for sure, so it
// takes the side that sends the rule set. No occurrence reads as loading.
func FrontmatterOmits(values []string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return value != omitInstructionsLoads })
}

// validAgentType reports whether an agent type can name a file in the mirror:
// letters, digits, '-' and '_' only, so no separator, dot segment or plugin
// scope (a plugin agent's type holds a colon) reaches the path.
func validAgentType(agentType string) bool {
	if agentType == "" {
		return false
	}
	if len(agentType) > agentTypeLengthMax {
		return false
	}
	for _, char := range agentType {
		switch {
		case 'a' <= char && char <= 'z':
		case 'A' <= char && char <= 'Z':
		case '0' <= char && char <= '9':
		case char == '-', char == '_':
		default:
			return false
		}
	}
	return true
}

// FrontmatterFields reads the top-level key: value lines between a
// definition's opening and closing "---" lines, each value with its trailing
// " #" comment removed and its quotes kept, and every occurrence of a key kept
// in file order. A key is read without one pair of quotes around it. An
// indented line belongs to the value above it (a multi-line description), so
// it is never read as a key and never closes the block. A definition with no
// closed frontmatter has no fields, so an unterminated block is never read as
// configuration.
func FrontmatterFields(body string) map[string][]string {
	fields := map[string][]string{}
	lines := strings.Split(body, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return fields
	}
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if strings.TrimSpace(line) == "---" {
			return fields
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = FrontmatterUnquoted(strings.TrimSpace(key))
		fields[key] = append(fields[key], frontmatterValue(value))
	}
	return map[string][]string{}
}

// frontmatterValue answers a frontmatter value without its surrounding blanks
// and its trailing comment: a '#' at the value's start or after a blank opens
// a YAML comment.
func frontmatterValue(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "#") {
		return ""
	}
	for at := range len(value) {
		if value[at] == '#' && (value[at-1] == ' ' || value[at-1] == '\t') {
			return strings.TrimSpace(value[:at])
		}
	}
	return value
}

// FrontmatterUnquoted answers a value without one pair of matching quotes
// around it, as YAML reads "ze-read" and 'ze-read' as ze-read.
func FrontmatterUnquoted(value string) string {
	if len(value) < 2 {
		return value
	}
	first, last := value[0], value[len(value)-1]
	if first == last && (first == '"' || first == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}
