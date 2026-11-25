// Design: docs/contributing/claude-code-cheatsheet.md -- what `./le ai sync check` reports
// Related: ai.go -- skillTrees and claudeAgents that this table describes
// Related: report.go -- how the harness rows render
//
// harnesses.go declares, for every coding-agent harness tree Ze syncs into,
// which enforcement and instruction capabilities that harness actually holds:
// skill stubs, subagent definitions, PreToolUse/PostToolUse hooks, and `@path`
// include expansion. Tests check every declaration against skillTrees and
// `.claude/settings.json`, so adding a harness tree or dropping hooks cannot
// leave a silent enforcement gap.
package aisync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const claudeSettingsFile = ".claude/settings.json"

// Support states whether a harness provides one capability.
type Support uint8

const (
	// SupportUnspecified is the invalid zero value.
	SupportUnspecified Support = iota
	// SupportYes means the harness provides the capability.
	SupportYes
	// SupportNo means the harness does not provide it; Reason explains the gap.
	SupportNo
)

// MarshalJSON encodes Support as "yes" or "no".
func (s Support) MarshalJSON() ([]byte, error) {
	switch s {
	case SupportYes:
		return []byte(`"yes"`), nil
	case SupportNo:
		return []byte(`"no"`), nil
	case SupportUnspecified:
		panic("BUG: aisync: SupportUnspecified marshaled to JSON")
	default:
		panic("BUG: aisync: unknown Support value")
	}
}

// label answers the plain-text word rendered in `./le ai sync check`.
func (s Support) label() string {
	switch s {
	case SupportYes:
		return "yes"
	case SupportNo:
		return "no"
	case SupportUnspecified:
		panic("BUG: aisync: SupportUnspecified rendered")
	default:
		panic("BUG: aisync: unknown Support value")
	}
}

// Capability pairs one support verdict with the reason when it is absent.
type Capability struct {
	State  Support `json:"state"`
	Reason string  `json:"reason,omitempty"`
}

// HarnessGrants declares what one synced harness actually receives from Ze.
type HarnessGrants struct {
	Name             string     `json:"name"`
	SkillsDir        string     `json:"skills-dir"`
	SkillStubs       Capability `json:"skill-stubs"`
	AgentDefinitions Capability `json:"agent-definitions"`
	HookEnforcement  Capability `json:"hook-enforcement"`
	IncludeExpansion Capability `json:"include-expansion"`
}

// declaredHarnesses lists one row per synced harness directory in skillTrees.
var declaredHarnesses = [...]HarnessGrants{
	{
		Name:             "claude",
		SkillsDir:        claudeSkills,
		SkillStubs:       Capability{State: SupportYes},
		AgentDefinitions: Capability{State: SupportYes},
		HookEnforcement:  Capability{State: SupportYes},
		IncludeExpansion: Capability{State: SupportYes},
	},
	{
		Name:             "codex",
		SkillsDir:        codexSkills,
		SkillStubs:       Capability{State: SupportYes},
		AgentDefinitions: Capability{State: SupportNo, Reason: "only .claude/agents/ is mirrored and read"},
		HookEnforcement:  Capability{State: SupportNo, Reason: "no PreToolUse/PostToolUse hook runtime; rules are self-enforced"},
		IncludeExpansion: Capability{State: SupportNo, Reason: "reads AGENTS.md flat without expanding @path includes"},
	},
	{
		Name:             "agents",
		SkillsDir:        agentsSkills,
		SkillStubs:       Capability{State: SupportYes},
		AgentDefinitions: Capability{State: SupportNo, Reason: "only .claude/agents/ is mirrored and read"},
		HookEnforcement:  Capability{State: SupportNo, Reason: "no PreToolUse/PostToolUse hook runtime; rules are self-enforced"},
		IncludeExpansion: Capability{State: SupportNo, Reason: "reads AGENTS.md flat without expanding @path includes"},
	},
}

// DeclaredHarnesses returns a copy of the declared harness capability table.
func DeclaredHarnesses() []HarnessGrants {
	out := make([]HarnessGrants, len(declaredHarnesses))
	copy(out, declaredHarnesses[:])
	return out
}

// InspectHarnesses returns the declared harness table verified against root's
// live hook wiring (`.claude/settings.json`).
func InspectHarnesses(root string) []HarnessGrants {
	rows := DeclaredHarnesses()
	hooksWired, reason := claudeHooksWired(root)
	for idx := range rows {
		if rows[idx].Name != "claude" {
			continue
		}
		if !hooksWired {
			rows[idx].HookEnforcement = Capability{State: SupportNo, Reason: reason}
		}
	}
	return rows
}

func claudeHooksWired(root string) (bool, string) {
	path := filepath.Join(root, filepath.FromSlash(claudeSettingsFile))
	raw, err := os.ReadFile(path) //nolint:gosec // reading canonical .claude/settings.json under checkout root
	if errors.Is(err, os.ErrNotExist) {
		return false, claudeSettingsFile + " is absent"
	}
	if err != nil {
		return false, "cannot read " + claudeSettingsFile + ": " + err.Error()
	}
	var parsed struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, "cannot parse " + claudeSettingsFile + ": " + err.Error()
	}
	if len(parsed.Hooks) == 0 {
		return false, claudeSettingsFile + " declares no hooks"
	}
	return true, ""
}

func (h HarnessGrants) text(tb *textbuf.Buffer) {
	tb.Str("  ").Str(h.Name).Str(" (").Str(h.SkillsDir).Str("): ")
	tb.Str("skills=").Str(h.SkillStubs.State.label())
	tb.Str(", agents=").Str(h.AgentDefinitions.State.label())
	tb.Str(", hooks=").Str(h.HookEnforcement.State.label())
	tb.Str(", includes=").Str(h.IncludeExpansion.State.label())
	tb.Byte('\n')
}
