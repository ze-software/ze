// Design: ai/rules/repo-maintenance.md -- one canonical source, several tool mirrors
// Related: actions.go -- the verbs that reach this mirror
// Related: report.go -- what a run answers
//
// Package aisync generates the tool-specific copies of Ze's agent instructions
// from their one canonical source.
//
// Three things are mirrored. A skill is ai/skills/<name>.md, copied verbatim
// for Claude and Codex and with .claude/ repointed at .agents/ for the Codex
// CLI. A subagent definition is ai/agents/<name>.md, copied flat, and Claude
// Code is the only tool that reads one. CLAUDE.md and AGENTS.md come from
// ai/INSTRUCTIONS.md with {{TOOL}} substituted.
//
// EVERY TARGET IS GITIGNORED, so `git diff` cannot show drift in those targets.
// The check generates a fresh copy in a scratch tree and compares the content.
// Only that comparison makes the drift visible.
//
// A missing SOURCE is an error, never an empty run. The shell half answers
// "synced 0 skill(s) + 0 agent(s) + CLAUDE.md + AGENTS.md" and exits 0 for a
// tree holding no skill and no instructions file, naming two files it did not
// write (plan/journal/zero-value-as-valid-answer.md, 2026-08-26).
package aisync

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the word this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "ai"

// The canonical sources, as repository paths. Each is REQUIRED: a checkout
// missing one is a caller pointed at the wrong tree.
const (
	skillSources = "ai/skills"
	agentSources = "ai/agents"
	instructions = "ai/INSTRUCTIONS.md"
)

// The generated targets, as repository paths.
const (
	claudeSkills       = ".claude/skills"
	codexSkills        = ".codex/skills"
	agentsSkills       = ".agents/skills"
	claudeAgents       = ".claude/agents"
	claudeInstructions = "CLAUDE.md"
	codexInstructions  = "AGENTS.md"
)

// skillFile is the name a skill takes inside its own directory, which is the
// layout each harness reads.
const skillFile = "SKILL.md"

// markdown is the extension every source carries.
const markdown = ".md"

// toolToken is what an instructions file spells where the tool's name goes.
const toolToken = "{{TOOL}}"

// The two tool names substituted for toolToken.
const (
	claudeTool = "Claude"
	codexTool  = "Codex"
)

// The path prefix a skill names, and what the Codex CLI's own mirror needs it
// to say instead.
const (
	claudePrefix = ".claude/"
	agentsPrefix = ".agents/"
)

// Directory and file modes for what this writes. The mirrors are generated
// artifacts of a build host, and nothing outside this account reads them.
const (
	dirMode  os.FileMode = 0o750
	fileMode os.FileMode = 0o600
)

// ErrNoSource says a canonical source this mirror is generated from is not in
// the checkout at all.
var ErrNoSource = errors.New("aisync: a canonical source is missing, so this is not a Ze checkout")

// Mirror generates one checkout's agent files.
type Mirror struct {
	// Root is the checkout the sources are read from and the mirrors are
	// written into.
	Root string
}

// sources answers the skill and agent source names of the checkout, and reports
// a missing source as an error.
//
// The skills are REQUIRED, but the agents are not. Three agent definitions
// exist today, and a checkout with none is still a checkout. In contrast, no
// skills means that this tool received the wrong tree.
func (m Mirror) sources() (skills, agents []string, err error) {
	skills, err = m.markdownIn(skillSources)
	if err != nil {
		return nil, nil, err
	}
	if len(skills) == 0 {
		var tb textbuf.Buffer
		return nil, nil, errors.New(tb.Err(ErrNoSource).Str(": ").Str(skillSources).
			Str(" holds no ").Str(markdown).String())
	}

	agents, err = m.markdownIn(agentSources)
	if err != nil {
		return nil, nil, err
	}

	if _, statErr := os.Stat(filepath.Join(m.Root, instructions)); statErr != nil {
		var tb textbuf.Buffer
		return nil, nil, errors.New(tb.Err(ErrNoSource).Str(": ").Str(instructions).String())
	}
	return skills, agents, nil
}

// markdownIn answers the base names of the markdown files in one source
// directory, sorted. A directory that is not there answers an empty list, which
// the caller judges.
func (m Mirror) markdownIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(m.Root, dir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), markdown) {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), markdown))
	}
	sort.Strings(names)
	return names, nil
}

// generateInto writes every mirror under dest, keeping the repository-relative
// layout.
//
// dest is the checkout for a sync and a scratch tree for a check. Thus, both
// answers come from ONE generator. A check cannot judge the tree against a
// second implementation of the sync output.
func (m Mirror) generateInto(dest string, skills, agents []string) error {
	for _, name := range skills {
		body, err := m.source(skillSources, name)
		if err != nil {
			return err
		}
		var tb textbuf.Buffer
		leaf := tb.Str(name).Byte('/').Str(skillFile).String()
		if err := writeUnder(dest, filepath.Join(claudeSkills, leaf), body); err != nil {
			return err
		}
		if err := writeUnder(dest, filepath.Join(codexSkills, leaf), body); err != nil {
			return err
		}
		repointed := bytes.ReplaceAll(body, []byte(claudePrefix), []byte(agentsPrefix))
		if err := writeUnder(dest, filepath.Join(agentsSkills, leaf), repointed); err != nil {
			return err
		}
	}

	for _, name := range agents {
		body, err := m.source(agentSources, name)
		if err != nil {
			return err
		}
		if err := writeUnder(dest, filepath.Join(claudeAgents, name+markdown), body); err != nil {
			return err
		}
	}

	body, err := os.ReadFile(filepath.Join(m.Root, instructions)) //nolint:gosec // a fixed path of the checkout
	if err != nil {
		return err
	}
	if err := writeUnder(dest, claudeInstructions,
		bytes.ReplaceAll(body, []byte(toolToken), []byte(claudeTool))); err != nil {
		return err
	}
	return writeUnder(dest, codexInstructions,
		bytes.ReplaceAll(body, []byte(toolToken), []byte(codexTool)))
}

// source reads one canonical file by its directory and base name.
func (m Mirror) source(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(m.Root, dir, name+markdown)) //nolint:gosec // a name this run listed from the checkout
}

// writeUnder writes one generated file, creating the directories it needs.
func writeUnder(dest, rel string, body []byte) error {
	full := filepath.Join(dest, rel)
	if err := os.MkdirAll(filepath.Dir(full), dirMode); err != nil {
		return err
	}
	return os.WriteFile(full, body, fileMode)
}

// Sync writes every mirror into the checkout.
func (m Mirror) Sync() (Report, error) {
	skills, agents, err := m.sources()
	if err != nil {
		return Report{}, err
	}
	if err := m.generateInto(m.Root, skills, agents); err != nil {
		return Report{}, err
	}
	return Report{Mode: modeSync, Skills: skills, Agents: agents}, nil
}

// Preview names what a sync would write, and writes nothing.
func (m Mirror) Preview() (Report, error) {
	skills, agents, err := m.sources()
	if err != nil {
		return Report{}, err
	}
	return Report{Mode: modePreview, Skills: skills, Agents: agents}, nil
}

// Check compares the checkout's mirrors against a fresh generation and answers
// every path the two disagree about.
//
// It WRITES NOTHING into the checkout. The fresh copy goes to a scratch tree
// that every exit path removes. The native hook runtime runs this check at
// session start. A check that touches the judged tree is not read-only.
func (m Mirror) Check() (Report, error) {
	skills, agents, err := m.sources()
	if err != nil {
		return Report{}, err
	}

	scratch, err := os.MkdirTemp("", "ze-aisync-")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(scratch) //nolint:errcheck // a scratch tree this run owns

	if err := m.generateInto(scratch, skills, agents); err != nil {
		return Report{}, err
	}

	report := Report{Mode: modeCheck, Skills: skills, Agents: agents}
	for _, tree := range []string{claudeSkills, codexSkills, agentsSkills, claudeAgents} {
		stale, err := driftIn(scratch, m.Root, tree)
		if err != nil {
			return Report{}, err
		}
		report.Stale = append(report.Stale, stale...)
	}
	for _, name := range []string{claudeInstructions, codexInstructions} {
		same, err := sameFile(filepath.Join(scratch, name), filepath.Join(m.Root, name))
		if err != nil {
			return Report{}, err
		}
		if !same {
			report.Stale = append(report.Stale, name)
		}
	}
	sort.Strings(report.Stale)
	return report, nil
}

// driftIn answers every repository-relative path of one mirror tree that the
// fresh generation and the checkout disagree about.
//
// The comparison reads the UNION of both sides. A path present only in the
// checkout is an ORPHAN, which is a mirror whose source was deleted. A path
// present only in the generation is a mirror that was never written. A walk of
// one side finds only one case and incorrectly calls the tree fresh.
func driftIn(fresh, live, tree string) ([]string, error) {
	freshPaths, err := filesUnder(filepath.Join(fresh, tree))
	if err != nil {
		return nil, err
	}
	livePaths, err := filesUnder(filepath.Join(live, tree))
	if err != nil {
		return nil, err
	}

	union := make(map[string]bool, len(freshPaths)+len(livePaths))
	for path := range freshPaths {
		union[path] = true
	}
	for path := range livePaths {
		union[path] = true
	}

	var stale []string
	for path := range union {
		same, err := sameFile(filepath.Join(fresh, tree, path), filepath.Join(live, tree, path))
		if err != nil {
			return nil, err
		}
		if !same {
			stale = append(stale, filepath.Join(tree, path))
		}
	}
	sort.Strings(stale)
	return stale, nil
}

// filesUnder answers every file below dir, keyed by its path relative to dir. A
// directory that is not there answers an empty set, which the union then reads
// as "every generated file is missing".
func filesUnder(dir string) (map[string]bool, error) {
	found := make(map[string]bool)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		found[rel] = true
		return nil
	})
	return found, err
}

// sameFile reports whether two paths hold the same bytes.
//
// A missing path differs from an existing path. This rule makes both a missing
// mirror and an orphan mirror visible. An unreadable path is an ERROR rather
// than a difference. A check that cannot read the tree has not judged it.
func sameFile(left, right string) (bool, error) {
	leftBody, leftErr := os.ReadFile(left) //nolint:gosec // a path this run composed from its own tables
	if leftErr != nil && !errors.Is(leftErr, os.ErrNotExist) {
		return false, leftErr
	}
	rightBody, rightErr := os.ReadFile(right) //nolint:gosec // a path this run composed from its own tables
	if rightErr != nil && !errors.Is(rightErr, os.ErrNotExist) {
		return false, rightErr
	}
	if leftErr != nil || rightErr != nil {
		// One side is absent and the other is not. That is a DIFFERENCE, not
		// a read failure. This branch reports both the missing mirror and the
		// orphan mirror.
		return false, nil //nolint:nilerr // absence is the answer here, not an error
	}
	return bytes.Equal(leftBody, rightBody), nil
}
