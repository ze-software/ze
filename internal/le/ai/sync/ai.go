// Design: ai/rules/repo-maintenance.md -- one canonical source, several tool mirrors
// Related: actions.go -- the verbs that reach this mirror
// Related: report.go -- what a run answers
//
// Package aisync generates the tool-specific files of Ze's agent instructions
// from their one canonical source.
//
// Three things are generated. A skill is ai/skills/<name>.md, and each harness
// directory gets the same STUB for it: the source's frontmatter byte for byte,
// a marker line naming the source, and an order to read that source in full.
// No line of the skill's body is copied, so an edit to the body owes no sync;
// only the frontmatter, which a harness lists the skill by, can go stale. A
// subagent definition is ai/agents/<name>.md, copied flat, and Claude Code is
// the only tool that reads one. AGENTS.md is a copy of ai/INSTRUCTIONS.md, and
// every agent reads it, Claude Code included.
//
// A sync removes a leftover stub, a <name>/SKILL.md in a skill tree whose
// first line after its frontmatter is the marker for <name> and that no source
// produces. Every other file a generation does not produce is left byte for
// byte and named in the answer, because le cannot show it wrote that file.
//
// A root CLAUDE.md MUST NOT exist. Claude Code reads AGENTS.md only when no
// CLAUDE.md is present, so a leftover CLAUDE.md silently replaces the rules
// with a stale copy. A sync removes it, and a check reports it as stale.
//
// EVERY TARGET IS GITIGNORED, so `git diff` cannot show drift in those targets.
// The check generates a fresh copy in a scratch tree and compares the content.
// Only that comparison makes the drift visible.
//
// A missing SOURCE is an error, never an empty run. The shell half answers
// "synced 0 skill(s) + 0 agent(s) + AGENTS.md" and exits 0 for a tree
// holding no skill and no instructions file, naming a file it did not write (plan/journal/zero-value-as-valid-answer.md, 2026-08-26).
package aisync

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the word this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "ai sync"

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
	agentsInstructions = "AGENTS.md"
)

// shadowInstructions is the file whose presence makes Claude Code ignore
// agentsInstructions. No generation writes it.
const shadowInstructions = "CLAUDE.md"

// skillFile is the name a skill takes inside its own directory, which is the
// layout each harness reads.
const skillFile = "SKILL.md"

// markdown is the extension every source carries.
const markdown = ".md"

// skillTrees are the harness directories that each receive one stub per skill.
var skillTrees = []string{claudeSkills, codexSkills, agentsSkills}

// fence is the line that opens and closes a frontmatter block.
const fence = "---"

// The frontmatter keys a stub needs. Every harness lists a skill by them.
const (
	nameKey        = "name"
	descriptionKey = "description"
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

// ErrBadSkill says a skill source cannot make a stub, so the run wrote nothing.
var ErrBadSkill = errors.New("aisync: a skill source cannot make a stub")

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
	slices.Sort(names)
	return names, nil
}

// generateInto writes every mirror under dest, keeping the repository-relative
// layout.
//
// dest is the checkout for a sync and a scratch tree for a check. Thus, both
// answers come from ONE generator. A check cannot judge the tree against a
// second implementation of the sync output.
//
// Every stub is composed before the first write. A source that cannot make
// one refuses the whole run, so a refusal never leaves some skills written and
// others not.
func (m Mirror) generateInto(dest string, skills, agents []string) error {
	stubs := make([][]byte, len(skills))
	for i, name := range skills {
		source, err := m.source(skillSources, name)
		if err != nil {
			return err
		}
		stub, err := stubOf(name, source)
		if err != nil {
			return err
		}
		stubs[i] = stub
	}
	if err := skillTreesAreDirectories(dest); err != nil {
		return err
	}

	for i, name := range skills {
		for _, tree := range skillTrees {
			if err := writeSkillUnder(dest, tree, name, stubs[i]); err != nil {
				return err
			}
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
	return writeUnder(dest, agentsInstructions, body)
}

// sourcePath answers the repository path of one skill's source. The name is
// one this run listed from ai/skills, never text read from a frontmatter.
func sourcePath(name string) string {
	var tb textbuf.Buffer
	return tb.Str(skillSources).Byte('/').Str(name).Str(markdown).String()
}

// markerFor answers the line that shows le generated a skill's stub. It is the
// first line after the stub's frontmatter, and a sync removes a file with no
// source only when that line is byte-equal to this one.
func markerFor(name string) string {
	var tb textbuf.Buffer
	return tb.Str("<!-- Generated by ./le ai sync update from ").Str(sourcePath(name)).
		Str(". Do not edit this file: edit that one. -->").String()
}

// instructionFor answers the body of a stub after its marker: the orders that
// send the agent to the source.
//
// It MUST hold no argument placeholder. A harness substitutes one wherever it
// finds it, and one that finds none appends the arguments to the stub, which
// is what the third order relies on.
func instructionFor(name string) string {
	path := sourcePath(name)
	var tb textbuf.Buffer
	tb.Str("\nThis skill's instructions are in `").Str(path).Str("`, relative to the repository root.\n\n")
	tb.Str("1. Read `").Str(path).Str("` in full now, in one read with no offset and no limit.\n")
	tb.Str("2. Follow that file as this skill's instructions.\n")
	tb.Str("3. The argument placeholder in that file stands for the arguments given to this skill.\n")
	tb.Str("4. Read that file again after a context compaction, before you go on.\n")
	tb.Str("5. If that file cannot be read, stop and say so. Do not work from this skill's description.\n")
	return tb.String()
}

// stubOf composes the stub every harness directory receives for one skill:
// the source's frontmatter block byte for byte, the marker, the instruction.
//
// It is the one composition, and generateInto is its only caller, so a sync
// and a check judge the tree against the same bytes.
func stubOf(name string, source []byte) ([]byte, error) {
	// A source named "...md", "..md" or ".md" lists as "..", "." or the empty
	// name, and its stub would be written outside its own skill directory.
	if name == "" {
		return nil, badSkill(name, "has a file name that is not a skill name",
			"its skill file would be written outside its own directory in each harness tree",
			"rename the file to the skill's name: mv "+sourcePath(name)+" "+skillSources+"/<skill name>"+markdown)
	}
	if name == "." || name == ".." {
		return nil, badSkill(name, "has a file name that is not a skill name",
			"its skill file would be written outside its own directory in each harness tree",
			"rename the file to the skill's name: mv "+sourcePath(name)+" "+skillSources+"/<skill name>"+markdown)
	}
	block, _, found := frontmatterOf(source)
	if !found {
		return nil, badSkill(name, "does not open with a frontmatter block",
			"a harness lists a skill by the name and description in that block",
			"add at its top the lines: "+fence+", "+nameKey+": "+name+", "+descriptionKey+": <when to use this skill>, "+fence)
	}
	declared, hasName := fieldOf(block, nameKey)
	if !hasName {
		return nil, badSkill(name, "has no `name:` line in its frontmatter",
			"a harness lists a skill by its name",
			"add this line to its frontmatter: "+nameKey+": "+name)
	}
	description, hasDescription := fieldOf(block, descriptionKey)
	if !hasDescription {
		return nil, badSkill(name, "has no `description:` line in its frontmatter",
			"a harness describes a skill by it, and Pi does not load a skill without one",
			"add this line to its frontmatter: "+descriptionKey+": <when to use this skill>")
	}
	if description == "" {
		return nil, badSkill(name, "has an empty `description:` line in its frontmatter",
			"a harness describes a skill by it, and Pi does not load a skill without one",
			"edit that line to: "+descriptionKey+": <when to use this skill>")
	}
	if declared != name {
		return nil, badSkill(name, "names itself \""+declared+"\" in its frontmatter, not its file name",
			"Claude Code names a skill by its directory and the other harnesses by `name`, so the skill would carry two names",
			"edit that line to: "+nameKey+": "+name)
	}

	marker := markerFor(name)
	instruction := instructionFor(name)
	stub := make([]byte, 0, len(block)+1+len(marker)+1+len(instruction))
	stub = append(stub, block...)
	if !bytes.HasSuffix(block, []byte{'\n'}) {
		stub = append(stub, '\n')
	}
	stub = append(stub, marker...)
	stub = append(stub, '\n')
	stub = append(stub, instruction...)
	return stub, nil
}

// badSkill builds the refusal for a source that cannot make a stub: the file
// and what is wrong first, why it matters next, and the edit that fixes it on
// the last line.
func badSkill(name, problem, why, fix string) error {
	var tb textbuf.Buffer
	tb.Str(sourcePath(name)).Byte(' ').Str(problem).Str(", so no skill file was written.\n")
	tb.Str(why).Str(".\n")
	tb.Str(fix)
	return fmt.Errorf("%w: %s", ErrBadSkill, tb.String())
}

// frontmatterOf answers the frontmatter block that opens source, from its
// opening fence line through its closing fence line and that line's newline,
// and the bytes after it. found is false when source opens with no block, or
// its block is never closed.
func frontmatterOf(source []byte) (block, rest []byte, found bool) {
	first, _, hasNewline := bytes.Cut(source, []byte{'\n'})
	if !hasNewline {
		return nil, nil, false
	}
	if !isFence(first) {
		return nil, nil, false
	}
	at := len(first) + 1
	// Bounded by len(source): each pass moves at past one line.
	for at < len(source) {
		line, _, _ := bytes.Cut(source[at:], []byte{'\n'})
		end := at + len(line)
		if end < len(source) {
			end++
		}
		if isFence(line) {
			return source[:end], source[end:], true
		}
		at = end
	}
	return nil, nil, false
}

// isFence reports whether one line is a frontmatter fence.
func isFence(line []byte) bool {
	return string(bytes.TrimSuffix(line, []byte{'\r'})) == fence
}

// fieldOf answers the value of one top-level key of a frontmatter block, with
// surrounding space and quotes removed. found is false when no line sets it.
func fieldOf(block []byte, key string) (value string, found bool) {
	prefix := key + ":"
	for line := range strings.SplitSeq(string(block), "\n") {
		rest, has := strings.CutPrefix(strings.TrimSuffix(line, "\r"), prefix)
		if !has {
			continue
		}
		value = strings.TrimSpace(rest)
		value = strings.Trim(value, `"'`)
		return strings.TrimSpace(value), true
	}
	return "", false
}

// source reads one canonical file by its directory and base name.
func (m Mirror) source(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(m.Root, dir, name+markdown)) //nolint:gosec // a name this run listed from the checkout
}

// skillTreesAreDirectories refuses the run when a skill tree root exists and
// is not a real directory. Every call below it (Lstat of a deeper path,
// Remove, MkdirAll, WriteFile) follows a symlinked tree root, so a sync would
// write into the link's target, outside the three trees. It runs before the
// first write, so a refusal writes nothing, the same as a bad source.
func skillTreesAreDirectories(dest string) error {
	for _, tree := range skillTrees {
		info, err := os.Lstat(filepath.Join(dest, tree))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && !info.IsDir() {
			var tb textbuf.Buffer
			return errors.New(tb.Str(tree).Str(" is a symbolic link or a file, not a directory, so no skill file was written.\n").
				Str("A sync would write through it into another directory.\n").
				Str("remove the link, and the next sync creates the directory: rm ").Str(tree).String())
		}
	}
	return nil
}

// writeSkillUnder writes one skill stub as a regular file at
// <tree>/<name>/SKILL.md, never through what already sits on that path.
//
// os.MkdirAll and os.WriteFile follow a symlink. A <name> directory linked
// elsewhere would have the SKILL.md of the link's target overwritten, so a
// <name> entry that is not a real directory is removed (only the link, never
// its target) and a real directory is created in its place. The tree root
// above it was judged by skillTreesAreDirectories.
//
// A SKILL.md linked to its source would have the source overwritten with the
// stub, so an entry there that is neither a regular file nor a directory is
// removed first and the stub written in its place. A directory at that path
// is an error, because removing it could drop files le did not write.
func writeSkillUnder(dest, tree, name string, body []byte) error {
	if err := removeNonDirectory(filepath.Join(dest, tree, name)); err != nil {
		return err
	}
	rel := filepath.Join(tree, name, skillFile)
	info, err := os.Lstat(filepath.Join(dest, rel))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		if info.IsDir() {
			var tb textbuf.Buffer
			return errors.New(tb.Str(rel).Str(" is a directory, so the sync cannot write the skill file there.\n").
				Str("Each harness reads a skill from that exact file path.\n").
				Str("remove it, keeping anything you need from it: rm -r ").Str(rel).String())
		}
		if !info.Mode().IsRegular() {
			if err := os.Remove(filepath.Join(dest, rel)); err != nil {
				return err
			}
		}
	}
	return writeUnder(dest, rel, body)
}

// removeNonDirectory removes the entry at path when it exists and is not a
// real directory: a symlink (to a directory or not) or a file. Lstat never
// follows the link, and Remove removes the link, never its target. An absent
// path or a real directory is left for the write to use.
func removeNonDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && !info.IsDir() {
		return os.Remove(path)
	}
	return nil
}

// writeUnder writes one generated file, creating the directories it needs.
func writeUnder(dest, rel string, body []byte) error {
	full := filepath.Join(dest, rel)
	if err := os.MkdirAll(filepath.Dir(full), dirMode); err != nil {
		return err
	}
	return os.WriteFile(full, body, fileMode)
}

// Sync writes every generated file into the checkout, then removes each
// leftover stub and names every other file it found and left in place.
func (m Mirror) Sync() (Report, error) {
	skills, agents, err := m.sources()
	if err != nil {
		return Report{}, err
	}
	if err := m.generateInto(m.Root, skills, agents); err != nil {
		return Report{}, err
	}
	shadow := filepath.Join(m.Root, shadowInstructions)
	if err := os.Remove(shadow); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Report{}, err
	}
	removed, left, err := m.clearLeftovers(skills, agents)
	if err != nil {
		return Report{}, err
	}
	return Report{Mode: modeSync, Skills: skills, Agents: agents, Removed: removed, Left: left}, nil
}

// clearLeftovers walks the four generated trees for every path this run did
// not generate. In a skill tree, a leftover stub is removed, with its
// directory when that leaves it empty. Every other such path, in any of the
// four trees, is left byte for byte. Both answers are repository-relative and
// sorted.
func (m Mirror) clearLeftovers(skills, agents []string) (removed, left []string, err error) {
	generated := make(map[string]bool, len(skills))
	for _, name := range skills {
		generated[filepath.Join(name, skillFile)] = true
	}
	for _, tree := range skillTrees {
		extras, err := extrasIn(m.Root, tree, generated)
		if err != nil {
			return nil, nil, err
		}
		for _, rel := range extras {
			isStub, err := isLeftoverStub(m.Root, tree, rel)
			if err != nil {
				return nil, nil, err
			}
			if !isStub {
				left = append(left, filepath.Join(tree, rel))
				continue
			}
			if err := removeStub(m.Root, tree, rel); err != nil {
				return nil, nil, err
			}
			removed = append(removed, filepath.Join(tree, rel))
		}
	}

	// A subagent definition carries no marker, so le can never show it wrote
	// one: an extra file in this tree is always left and named.
	definitions := make(map[string]bool, len(agents))
	for _, name := range agents {
		definitions[name+markdown] = true
	}
	extras, err := extrasIn(m.Root, claudeAgents, definitions)
	if err != nil {
		return nil, nil, err
	}
	for _, rel := range extras {
		left = append(left, filepath.Join(claudeAgents, rel))
	}

	slices.Sort(removed)
	slices.Sort(left)
	return removed, left, nil
}

// extrasIn answers every path below one generated tree, relative to that tree,
// that is not in generated. It reuses the check's walk, so the sync and the
// check agree on what is extra.
func extrasIn(root, tree string, generated map[string]bool) ([]string, error) {
	paths, err := filesUnder(filepath.Join(root, tree))
	if err != nil {
		return nil, err
	}
	var extras []string
	for rel := range paths {
		if !generated[rel] {
			extras = append(extras, rel)
		}
	}
	slices.Sort(extras)
	return extras, nil
}

// stubName answers the skill name a tree-relative path would be the stub of:
// a path of the exact shape <name>/SKILL.md. found is false for any other
// shape.
func stubName(rel string) (name string, found bool) {
	dir, file := filepath.Split(rel)
	if file != skillFile {
		return "", false
	}
	name = strings.TrimSuffix(dir, string(filepath.Separator))
	if name == "" {
		return "", false
	}
	if strings.ContainsRune(name, filepath.Separator) {
		return "", false
	}
	return name, true
}

// isLeftoverStub reports whether one extra path of a skill tree is a stub le
// wrote: a <name>/SKILL.md that is a regular file, never a symlink, whose
// first line after its frontmatter block is byte-equal to the marker for
// <name>. The marker anywhere else in the file does not count, so a
// hand-written file quoting it is never removed.
func isLeftoverStub(root, tree, rel string) (bool, error) {
	name, shaped := stubName(rel)
	if !shaped {
		return false, nil
	}
	full := filepath.Join(root, tree, rel)
	info, err := os.Lstat(full)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	body, err := os.ReadFile(full) //nolint:gosec // a regular file this run found under its own generated tree
	if err != nil {
		return false, err
	}
	_, rest, found := frontmatterOf(body)
	if !found {
		return false, nil
	}
	line, _, _ := bytes.Cut(rest, []byte{'\n'})
	return string(line) == markerFor(name), nil
}

// removeStub removes one leftover stub, then its directory when that leaves
// the directory empty. A directory holding another file is kept, and the walk
// names that file as left.
func removeStub(root, tree, rel string) error {
	full := filepath.Join(root, tree, rel)
	if err := os.Remove(full); err != nil {
		return err
	}
	dir := filepath.Dir(full)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(dir)
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
	// shadowInstructions is never generated, so a live copy reads as an
	// orphan: the fresh side is absent and the live side is not.
	for _, name := range []string{agentsInstructions, shadowInstructions} {
		same, err := sameFile(filepath.Join(scratch, name), filepath.Join(m.Root, name))
		if err != nil {
			return Report{}, err
		}
		if !same {
			report.Stale = append(report.Stale, name)
		}
	}
	slices.Sort(report.Stale)
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
	slices.Sort(stale)
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
// A missing path differs from an existing path, and two missing paths agree. This rule makes both a missing
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
	if leftErr != nil && rightErr != nil {
		// Absent on both sides is the answer for shadowInstructions, which
		// no generation writes.
		return true, nil //nolint:nilerr // absence on both sides is the answer here, not an error
	}
	if leftErr != nil || rightErr != nil {
		// One side is absent and the other is not. That is a DIFFERENCE, not
		// a read failure. This branch reports both the missing mirror and the
		// orphan mirror.
		return false, nil //nolint:nilerr // absence is the answer here, not an error
	}
	return bytes.Equal(leftBody, rightBody), nil
}
