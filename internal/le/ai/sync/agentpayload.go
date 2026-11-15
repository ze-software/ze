// Design: docs/contributing/claude-code-cheatsheet.md -- what `./le ai sync check` reports
// Related: ai.go -- the check these rows ride on
// Related: report.go -- how the rows render
//
// agentpayload.go answers, for each agent definition under ai/agents, the rule
// payload that agent starts with. A definition that omits the project
// instruction files (omitClaudeMd) starts with the agent rule set the start
// hook hands it; one that does not loads the whole always-loaded payload. A new
// definition that forgets the line silently costs the old size, so the check
// prints the figure for every definition.
//
// Nothing here decides or measures on its own. Whether a definition omits the
// files is ruleset.ReadAgentStart, the predicate the start hook acts on, read
// over the same mirror. Each figure is the total airules.Payload measures, the
// one measure `./le ai rules report payload` prints.

package aisync

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	airules "github.com/ze-software/ze/internal/le/ai/rules"
	"github.com/ze-software/ze/internal/le/ai/rules/ruleset"
)

// The start a row names, as its `start` key spells it.
const (
	// startOmits is a definition that omits the instruction files.
	startOmits = "omits-instructions"
	// startLoads is a definition that loads them.
	startLoads = "loads-instructions"
	// startUnread is a definition whose frontmatter names no agent of its own
	// name as the predicate reads it: read as omitting, the predicate's safe
	// side.
	startUnread = "unread-frontmatter"
	// startNotMirrored is a source with no copy in the mirror, which no agent
	// can start from until a sync writes it.
	startNotMirrored = "not-mirrored"
)

// The payload a row names, as its `payload` key spells it.
const (
	payloadAgentRuleSet = "agent-rule-set"
	payloadAlwaysLoaded = "always-loaded"
)

// ErrPayloadUnmeasured says the payload some definition starts with cannot be
// measured, because a part of it is missing, so the check prints no figure.
var ErrPayloadUnmeasured = errors.New("aisync: an agent's starting payload cannot be measured")

// AgentPayload is one agent definition's row: its name, how it starts, and the
// payload that start gives it in tokens.
type AgentPayload struct {
	Name  string `json:"name"`
	Start string `json:"start"`
	// Payload and Tokens are absent exactly when Start is startNotMirrored:
	// no agent of that type starts at all.
	Payload string `json:"payload,omitempty"`
	Tokens  *int   `json:"tokens,omitempty"`
}

// checkWithPayloads is Check with one row per agent source: the answer of
// `./le ai sync check`. The rows are not part of Check, because the start hook
// asks Check whether the mirror is fresh, and a rule part missing from the
// checkout is no stale mirror.
func (m Mirror) checkWithPayloads() (Report, error) {
	report, err := m.Check()
	if err != nil {
		return Report{}, err
	}
	report.AgentPayloads, err = m.agentPayloads(report.Agents)
	if err != nil {
		return Report{}, err
	}
	return report, nil
}

// agentPayloads answers one row per name, in the order given. With no name it
// measures nothing, so a checkout with no agent definition never fails on a
// rule part no agent starts with.
func (m Mirror) agentPayloads(agents []string) ([]AgentPayload, error) {
	if len(agents) == 0 {
		return nil, nil
	}
	measured, err := airules.Payload(m.Root)
	if err != nil {
		return nil, err
	}
	rows := make([]AgentPayload, 0, len(agents))
	for _, name := range agents {
		start, err := ruleset.ReadAgentStart(m.Root, name)
		if err != nil {
			return nil, err
		}
		row, err := payloadRow(name, start, measured)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// payloadRow answers the row of one definition. A row whose payload has a
// missing part is an error naming the part, never a figure: an absent part is
// not a smaller payload.
func payloadRow(name string, start ruleset.AgentStart, measured airules.PayloadReport) (AgentPayload, error) {
	row := AgentPayload{Name: name}
	switch start {
	case ruleset.AgentStartUndefined:
		row.Start = startNotMirrored
		return row, nil
	case ruleset.AgentStartOmits:
		row.Start = startOmits
	case ruleset.AgentStartUnread:
		row.Start = startUnread
	case ruleset.AgentStartLoads:
		row.Start = startLoads
	case ruleset.AgentStartUnspecified:
		panic("BUG: aisync: ReadAgentStart answered no start and no error")
	}
	set, payload, title := measured.PayloadSet, payloadAlwaysLoaded, "always-loaded payload"
	if start.Omits() {
		set, payload, title = measured.Agent, payloadAgentRuleSet, "agent rule set"
	}
	if len(set.Missing) > 0 {
		var tb textbuf.Buffer
		tb.Str(name).Str(" starts with the ").Str(title).Str(", which is missing ").Join(set.Missing, ", ").
			Str(". Restore the part, then run: ./le ai rules report payload")
		return AgentPayload{}, fmt.Errorf("%w: %s", ErrPayloadUnmeasured, tb.String())
	}
	tokens := set.Tokens
	row.Payload, row.Tokens = payload, &tokens
	return row, nil
}

// text renders one row on one line, in the words a person reads it by.
func (p AgentPayload) text(tb *textbuf.Buffer) {
	tb.Str("  ").Str(p.Name).Str(": ")
	switch p.Start {
	case startOmits:
		tb.Str("omits the instruction files")
	case startLoads:
		tb.Str("loads the instruction files")
	case startUnread:
		tb.Str("frontmatter names no agent ").Quoted(p.Name).Str(", so it is read as omitting the instruction files")
	case startNotMirrored:
		tb.Str("not in ").Str(claudeAgents).Str(", so no agent of this type starts until: ./le ai sync update\n")
		return
	default:
		panic("BUG: aisync: a row with a start payloadRow never writes")
	}
	tb.Str(", starts with the ")
	if p.Payload == payloadAgentRuleSet {
		tb.Str("agent rule set: ")
	} else {
		tb.Str("always-loaded payload: ")
	}
	tb.Int(int64(*p.Tokens)).Str(" tokens\n")
}
