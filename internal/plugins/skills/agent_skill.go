// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: agent.go — the four command paths and their wire methods
// Related: agent_hook.go — the session-start hook item of each harness
// Related: main.go — the ze guide whose name and description the stub carries
//
// create, show and delete agent skill share one plan producer, agentPlan. The
// daemon command answers the plan's remote form and calls no file or
// environment function: the caller's home is on another machine. The shell
// route resolves the caller's own harness directories, then writes, inspects
// or removes each item of the plan's local form there.

package skills

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/skillstub"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// harness is one agent harness the plugin knows how to set up. The table
// below is the only list of them: the YANG enumeration of the agent leaf is
// compared with it by TestTheModelsAgentValuesAreTheHarnessTable.
type harness struct {
	// name is the agent leaf's value.
	name string
	// title is the name the operator knows the harness by.
	title string
	// configDefault is the configuration directory relative to the home
	// directory, slash-separated. The harness is FOUND when it exists.
	configDefault string
	// configVariable is the harness's own environment variable naming its
	// configuration directory, or "" when it has none.
	configVariable string
	// ownSkills says the stub goes under <configuration directory>/skills.
	// Without it the harness reads the shared ~/.agents/skills directory.
	ownSkills bool
	// hookFile is the file in the configuration directory that holds the
	// session-start hooks, or "" when Ze knows no hook format for the harness.
	hookFile string
	// hookTrustByPosition says the harness runs a new or changed hook only
	// once its user trusts it, and keys that trust by the hook's position in
	// the file. Ze never writes that trust, and a delete never moves a group.
	hookTrustByPosition bool
}

// harnesses is the harness table. Codex, OpenCode and Pi read one shared stub
// directory; Claude Code reads its own.
var harnesses = []harness{
	{name: "claude", title: "Claude Code", configDefault: ".claude", configVariable: "CLAUDE_CONFIG_DIR", ownSkills: true, hookFile: "settings.json"},
	{name: "codex", title: "Codex", configDefault: ".codex", configVariable: "CODEX_HOME", hookFile: "hooks.json", hookTrustByPosition: true},
	{name: "opencode", title: "OpenCode", configDefault: ".config/opencode"},
	{name: "pi", title: "Pi", configDefault: ".pi"},
}

// The places of the stub, relative to the home directory or to a skills
// directory, slash-separated.
const (
	sharedSkillsDefault = ".agents/skills"
	skillsUnderConfig   = "skills"
	stubName            = "ze"
	stubFile            = "SKILL.md"
	homePrefix          = "~/"
)

// The agent leaf ze-skills-cmd.yang declares, and the value delete adds for
// every harness found.
const (
	leafAgent = "agent"
	agentAll  = "all"
)

// keyAgentSkill is the envelope key of the three commands' answer, with a
// registered drawing.
const keyAgentSkill = "agent-skill"

// The two kinds of item a plan holds.
const (
	itemSkill = "skill"
	itemHook  = "hook"
)

// The states a row answers. Each command answers a closed subset: create
// created, updated, unchanged, foreign, refused, unsupported; show current,
// outdated, missing, foreign, refused, unsupported; delete removed, absent,
// kept, foreign, refused, unsupported; the daemon route not-inspected on every
// row. kept is a correct outcome, never a failure: the delete of one harness
// leaves a stub another found harness still reads.
const (
	stateKept         = "kept"
	stateCreated      = "created"
	stateUpdated      = "updated"
	stateUnchanged    = "unchanged"
	stateCurrent      = "current"
	stateOutdated     = "outdated"
	stateMissing      = "missing"
	stateRemoved      = "removed"
	stateAbsent       = "absent"
	stateForeign      = "foreign"
	stateRefused      = "refused"
	stateUnsupported  = "unsupported"
	stateNotInspected = "not-inspected"
)

// What a row says of its harness's configuration directory. The daemon route
// answers stateNotInspected, because it cannot see the caller's home.
const (
	harnessFound    = "found"
	harnessNotFound = "not-found"
)

// stubMarker is the first line after the stub's frontmatter. It is what makes
// a stub Ze's own: create refreshes and delete removes only a SKILL.md that
// carries it, byte for byte, and both routes write the same one.
const stubMarker = "<!-- Installed by ze create agent skill. Remove it with ze delete agent skill agent all. -->"

// setupVerb is which of the three commands runs.
type setupVerb int

const (
	verbUnspecified setupVerb = iota
	verbCreate
	verbShow
	verbDelete
)

// planForm is which machine the plan is for.
type planForm int

const (
	formUnspecified planForm = iota
	// formLocal is for the machine the command runs on: the stub orders
	// `ze show skill ze`, and each harness has a hook item.
	formLocal
	// formRemote is for an agent connected to this Ze from a machine that may
	// hold no ze: the stub sends it back over its connection, and the plan
	// holds no hook, which would run a local ze.
	formRemote
)

// planItem is one file the plan sets up for one harness.
type planItem struct {
	harness *harness
	// item is itemSkill or itemHook.
	item string
	// path is the default place relative to the home directory,
	// slash-separated, and "" for the hook of a harness Ze has no hook for.
	path string
	// content is the stub text of a skill item, and "" for a hook.
	content string
}

// agentSkillAnswer is the answer of the three commands on both routes: one
// envelope, so the registered drawing draws it and every pipe renders it.
type agentSkillAnswer struct {
	plugin.DataMarker
	Setup agentSetup `json:"agent-skill"`
}

// agentSetup is what the envelope holds: what happened, then one row per
// harness and item.
type agentSetup struct {
	Note string     `json:"note"`
	Rows []setupRow `json:"rows"`
}

// setupRow is one item of one harness.
type setupRow struct {
	Agent   string `json:"agent"`
	Item    string `json:"item"`
	Harness string `json:"harness"`
	State   string `json:"state"`
	Path    string `json:"path"`
	Note    string `json:"note,omitempty"`
	// Content is the stub the caller places, on the daemon route's create and
	// show rows only.
	Content string `json:"content,omitempty"`
}

// outcome is what one file call left: the row's state, the path it acted on
// and, when there is something to say, the note.
type outcome struct {
	state string
	path  string
	note  string
}

// agentPlan is the one producer of what create, show and delete agent skill
// act on: for each harness acted on, its stub and, in the local form, its
// hook; for each harness in sharedOnly, its stub alone. Paths are the
// defaults relative to the home directory, and nothing is read.
func agentPlan(form planForm, acted, sharedOnly []*harness) []planItem {
	stub := stubText(form)
	plan := make([]planItem, 0, 2*len(acted)+len(sharedOnly))
	for _, h := range acted {
		plan = append(plan, planItem{harness: h, item: itemSkill, path: h.stubPath(), content: stub})
		if form == formLocal {
			plan = append(plan, planItem{harness: h, item: itemHook, path: h.hookPath()})
		}
	}
	for _, h := range sharedOnly {
		plan = append(plan, planItem{harness: h, item: itemSkill, path: h.stubPath(), content: stub})
	}
	return plan
}

// stubText answers the stub of the given form: the ze guide's name and
// description as frontmatter, the marker, the order and its fallback.
func stubText(form planForm) string {
	guide := zeGuide()
	var tb textbuf.Buffer
	tb.Str(skillstub.Fence).Byte('\n')
	tb.Str("name: ").Str(guide.Name).Byte('\n')
	tb.Str("description: ").Str(guide.Description).Byte('\n')
	tb.Str(skillstub.Fence).Byte('\n')
	tb.Str(stubMarker).Byte('\n')
	switch form {
	case formLocal:
		tb.Str("Run `ze show skill ze` and follow the guide it prints: it matches the ze installed here.\n")
		tb.Str("If that command fails, run `ze help`.\n")
	case formRemote:
		tb.Str("Ask the Ze system you are connected to for `show skill ze`, over the connection you ")
		tb.Str("already use (its MCP tool, its API or SSH), and follow the guide it answers.\n")
		tb.Str("If that fails, ask it for `help`.\n")
	case formUnspecified:
		panic("BUG: stubText called with no plan form")
	}
	return tb.String()
}

// zeGuide answers the inventory row of the ze guide, which the stub sends the
// agent to. The inventory is a literal of this package, so a miss is a Ze
// defect, never an operating error.
func zeGuide() *skill {
	for i := range inventory {
		if inventory[i].Name == stubName {
			return &inventory[i]
		}
	}
	panic("BUG: the skills inventory holds no ze guide")
}

// stubPath answers the default place of the harness's stub, relative to the
// home directory.
func (h *harness) stubPath() string {
	var tb textbuf.Buffer
	if h.ownSkills {
		tb.Str(h.configDefault).Byte('/').Str(skillsUnderConfig)
	} else {
		tb.Str(sharedSkillsDefault)
	}
	return tb.Byte('/').Str(stubName).Byte('/').Str(stubFile).String()
}

// hookPath answers the default place of the harness's hook file, relative to
// the home directory, or "" when Ze has no hook for the harness.
func (h *harness) hookPath() string {
	if h.hookFile == "" {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str(h.configDefault).Byte('/').Str(h.hookFile).String()
}

// harnessNamed answers the harness whose agent value is name.
func harnessNamed(name string) (*harness, bool) {
	for i := range harnesses {
		if harnesses[i].name == name {
			return &harnesses[i], true
		}
	}
	return nil, false
}

// everyHarness answers the whole table.
func everyHarness() []*harness {
	all := make([]*harness, len(harnesses))
	for i := range harnesses {
		all[i] = &harnesses[i]
	}
	return all
}

// sharedReaders answers the harnesses that read the shared stub directory.
func sharedReaders() []*harness {
	var readers []*harness
	for i := range harnesses {
		if !harnesses[i].ownSkills {
			readers = append(readers, &harnesses[i])
		}
	}
	return readers
}

// titlesOf answers the harnesses' titles as a sentence list: "A, B and C".
func titlesOf(list []*harness) string {
	var tb textbuf.Buffer
	for i, h := range list {
		if i > 0 {
			if i == len(list)-1 {
				tb.Str(" and ")
			} else {
				tb.Str(", ")
			}
		}
		tb.Str(h.title)
	}
	return tb.String()
}

// agentValues answers the agent values the table holds, for a message.
func agentValues() string {
	names := make([]string, len(harnesses))
	for i := range harnesses {
		names[i] = harnesses[i].name
	}
	return textbuf.Join(names, ", ")
}

// agentChoice is what the agent leaf selects: the harness it names, or nil for
// every harness (no agent on create and show, `agent all` on delete).
type agentChoice struct {
	named *harness
}

// selectedAgent reads the agent leaf. Only
// delete accepts `all` and only delete requires the leaf, because a delete
// that acts on every harness MUST be typed as such (ai/rules/cli.md). The
// validator judges both before the handler runs; this refuses again, so a
// route that skipped the model still cannot reach the every-harness form.
// prefix is how the route spells a command in its fix: "ze " at a shell, ""
// through the daemon.
func selectedAgent(verb setupVerb, validated command.ValidatedArgs, prefix string) (agentChoice, error) {
	value, given := validated.Value(leafAgent)
	if !given {
		if verb == verbDelete {
			var fix textbuf.Buffer
			return agentChoice{}, opererr.New("delete agent skill names no harness, so nothing was removed",
				"Removing Ze's setup from every harness found is typed as agent all, so an early Enter "+
					"cannot do it. Name one of "+agentValues()+", or all.",
				fix.Str(prefix).Str(pathDeleteAgentSkill).Str(" agent all").String())
		}
		return agentChoice{}, nil
	}
	if value == agentAll {
		if verb == verbDelete {
			return agentChoice{}, nil
		}
		return agentChoice{}, unknownAgent(value, prefix)
	}
	h, known := harnessNamed(value)
	if !known {
		return agentChoice{}, unknownAgent(value, prefix)
	}
	return agentChoice{named: h}, nil
}

// unknownAgent refuses an agent value the table does not hold.
func unknownAgent(value, prefix string) *opererr.Error {
	var problem, fix textbuf.Buffer
	return opererr.New(problem.Str("agent ").Str(value).Str(" is not a harness Ze knows").String(),
		"Ze knows "+agentValues()+". The report lists each with where Ze looks for it.",
		fix.Str(prefix).Str(pathShowAgentSkill).String())
}

// --- The daemon route -------------------------------------------------------

// remoteAnswer is the daemon route of the three commands: the plan's remote
// form, every row not-inspected, with the stub to place on create and show.
// It calls no file or environment function.
func remoteAnswer(verb setupVerb, validated command.ValidatedArgs) (agentSkillAnswer, error) {
	choice, err := selectedAgent(verb, validated, "")
	if err != nil {
		return agentSkillAnswer{}, err
	}
	acted := everyHarness()
	if choice.named != nil {
		acted = []*harness{choice.named}
	}
	plan := agentPlan(formRemote, acted, nil)
	rows := make([]setupRow, 0, len(plan))
	for _, item := range plan {
		row := setupRow{
			Agent:   item.harness.name,
			Item:    item.item,
			Harness: stateNotInspected,
			State:   stateNotInspected,
			Path:    homePrefix + item.path,
			Note:    remoteSharedNote(verb, item.harness),
		}
		if verb != verbDelete {
			row.Content = item.content
		}
		rows = append(rows, row)
	}
	return agentSkillAnswer{Setup: agentSetup{Note: remoteNote(verb), Rows: rows}}, nil
}

// remoteNote says what the daemon route's answer is: files for the caller to
// place or remove, on the machine its agent runs on.
func remoteNote(verb setupVerb) string {
	if verb == verbDelete {
		return "Nothing was read or removed on this Ze system: the agent's files are in the home " +
			"directory of the machine it runs on, which this system cannot see. There, remove each " +
			"file below whose first line after its frontmatter is: " + stubMarker
	}
	return "Nothing was read or written on this Ze system: the agent's files belong in the home " +
		"directory of the machine it runs on, which this system cannot see. There, write each " +
		"content below at its path, then load the ze skill."
}

// remoteSharedNote names the harnesses that read the same shared file.
func remoteSharedNote(verb setupVerb, h *harness) string {
	if h.ownSkills {
		return ""
	}
	readers := sharedReaders()
	if verb == verbDelete {
		return titlesOf(readers) + " all read this file: remove it only when no other of them is installed."
	}
	return titlesOf(readers) + " all read this one file."
}

// remoteResponse answers a daemon command.
func remoteResponse(verb setupVerb, validated command.ValidatedArgs) *plugin.Response {
	answer, err := remoteAnswer(verb, validated)
	if err != nil {
		if refusal, isRefusal := errors.AsType[*opererr.Error](err); isRefusal {
			return plugin.OperatorErrorResponse(refusal)
		}
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: answer}
}

// handleCreateAgentSkill is the daemon command of create agent skill.
func handleCreateAgentSkill(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return remoteResponse(verbCreate, validated), nil
}

// handleShowAgentSkill is the daemon command of show agent skill.
func handleShowAgentSkill(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return remoteResponse(verbShow, validated), nil
}

// handleDeleteAgentSkill is the daemon command of delete agent skill.
func handleDeleteAgentSkill(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	return remoteResponse(verbDelete, validated), nil
}

// --- The shell route --------------------------------------------------------

// harnessPlace is where one harness's files are on this machine: absolute
// paths, resolved from the home directory and the harness's own variable.
type harnessPlace struct {
	skills string
	// hook is the hook file, or "" when Ze has no hook for the harness.
	hook  string
	found bool
}

// localRun is one run of a command at a shell, with every harness's place
// resolved once.
type localRun struct {
	verb setupVerb
	// rerun is the command line that runs the command again, for a fix.
	rerun  string
	home   string
	places map[string]harnessPlace
	// named is the harness the operator named, or nil: every harness for
	// create and show, `agent all` for delete.
	named *harness
}

// newLocalRun resolves the home directory and every harness's place. HOME,
// CLAUDE_CONFIG_DIR and CODEX_HOME are the system's and the harnesses' own
// variables, not Ze's, so they are read from the environment as they stand
// (ai/rules/go-standards.md). Every path is made absolute, because a relative
// HOME would otherwise change meaning with the working directory.
func newLocalRun(verb setupVerb, path string, validated command.ValidatedArgs) (*localRun, error) {
	// A nil named harness on delete is `agent all`: selectedAgent refuses a
	// delete that names none.
	choice, err := selectedAgent(verb, validated, "ze ")
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, opererr.Wrap(err, "Ze cannot tell which home directory to set the agent up in",
			"The harnesses keep their skills under the home directory, which the HOME variable names.",
			"HOME=<your home directory> ze "+path)
	}
	home, err = filepath.Abs(home)
	if err != nil {
		return nil, opererr.Wrap(err, "Ze cannot make the home directory "+home+" an absolute path",
			"A relative home is read against the working directory, which Ze could not read.",
			"cd && ze "+path)
	}
	run := &localRun{verb: verb, home: home, places: make(map[string]harnessPlace, len(harnesses)), named: choice.named}
	var rerun textbuf.Buffer
	rerun.Str("ze ").Str(path)
	if value, given := validated.Value(leafAgent); given {
		rerun.Str(" agent ").Str(value)
	}
	run.rerun = rerun.String()
	for i := range harnesses {
		place, err := placeOf(&harnesses[i], home)
		if err != nil {
			return nil, err
		}
		run.places[harnesses[i].name] = place
	}
	return run, nil
}

// placeOf resolves where the harness's files are under home.
func placeOf(h *harness, home string) (harnessPlace, error) {
	config := filepath.Join(home, filepath.FromSlash(h.configDefault))
	if h.configVariable != "" {
		// An empty value names no directory, so the default stands.
		if value := os.Getenv(h.configVariable); value != "" {
			absolute, err := filepath.Abs(value)
			if err != nil {
				return harnessPlace{}, opererr.Wrap(err,
					h.configVariable+" names "+value+", which Ze cannot make an absolute path",
					"A relative directory is read against the working directory, which Ze could not read.",
					"export "+h.configVariable+"=<the absolute directory>")
			}
			config = absolute
		}
	}
	var place harnessPlace
	if h.ownSkills {
		place.skills = filepath.Join(config, skillsUnderConfig)
	} else {
		place.skills = filepath.Join(home, filepath.FromSlash(sharedSkillsDefault))
	}
	if h.hookFile != "" {
		place.hook = filepath.Join(config, h.hookFile)
	}
	info, err := os.Stat(config)
	if err == nil {
		place.found = info.IsDir()
		return place, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return place, nil
	}
	var problem textbuf.Buffer
	return harnessPlace{}, opererr.Wrap(err,
		problem.Str("Ze cannot tell whether ").Str(h.title).Str(" is installed: ").Str(config).Str(" cannot be read").String(),
		"Ze sets a harness up only where it can see the harness's configuration directory.",
		"ls -ld "+config)
}

// selection answers the harnesses this run acts on, and those whose shared
// stub alone it acts on.
func (r *localRun) selection() (acted, sharedOnly []*harness) {
	if r.named != nil {
		return []*harness{r.named}, nil
	}
	if r.verb == verbShow {
		return everyHarness(), nil
	}
	found := r.foundAmong(everyHarness(), nil)
	if r.verb == verbCreate {
		if len(found) == 0 {
			// No harness found: the shared stub is what any of its readers
			// installed later would list, and no settings file is created.
			return nil, sharedReaders()
		}
		return found, nil
	}
	// delete agent all: every harness found, and the shared stub even when
	// none of its readers is, because a create that found none wrote it.
	if len(r.foundAmong(sharedReaders(), nil)) == 0 {
		return found, sharedReaders()
	}
	return found, nil
}

// foundAmong answers the harnesses of list whose configuration directory
// exists, leaving out except.
func (r *localRun) foundAmong(list []*harness, except *harness) []*harness {
	var found []*harness
	for _, h := range list {
		if h == except {
			continue
		}
		if r.places[h.name].found {
			found = append(found, h)
		}
	}
	return found
}

// answer runs the plan's local form and answers one row per item.
func (r *localRun) answer() agentSkillAnswer {
	acted, sharedOnly := r.selection()
	plan := agentPlan(formLocal, acted, sharedOnly)
	// One stub directory may serve several harnesses: it is acted on once,
	// and every row that reads it reports the same outcome.
	stubs := make(map[string]outcome)
	rows := make([]setupRow, 0, len(plan))
	for _, item := range plan {
		place := r.places[item.harness.name]
		row := setupRow{Agent: item.harness.name, Item: item.item, Harness: harnessNotFound}
		if place.found {
			row.Harness = harnessFound
		}
		var result outcome
		switch item.item {
		case itemSkill:
			done, seen := stubs[place.skills]
			if !seen {
				done = r.stub(item, place)
				stubs[place.skills] = done
			}
			result = done
		case itemHook:
			result = r.hook(item, place)
		default:
			panic("BUG: agentPlan answered an item that is neither a skill nor a hook")
		}
		row.State, row.Path, row.Note = result.state, result.path, result.note
		if item.item == itemSkill && !item.harness.ownSkills && result.state != stateRefused {
			row.Note = joinNotes(row.Note, titlesOf(sharedReaders())+" all read this one file.")
		}
		rows = append(rows, row)
	}
	return agentSkillAnswer{Setup: agentSetup{Note: r.note(acted), Rows: rows}}
}

// stub acts on one stub directory as the run's verb says.
func (r *localRun) stub(item planItem, place harnessPlace) outcome {
	switch r.verb {
	case verbCreate:
		return createStub(place.skills, []byte(item.content), r.rerun)
	case verbShow:
		return inspectStub(place.skills, []byte(item.content), r.rerun)
	case verbDelete:
		if r.named != nil && !r.named.ownSkills {
			if readers := r.foundAmong(sharedReaders(), r.named); len(readers) > 0 {
				return keptSharedStub(place.skills, readers)
			}
		}
		return removeStub(place.skills, r.rerun)
	case verbUnspecified:
		panic("BUG: a local run with no verb")
	}
	panic("BUG: a local run with an unknown verb")
}

// hook acts on one harness's hook as the run's verb says. Every settings file
// call is behind agent_hook.go.
func (r *localRun) hook(item planItem, place harnessPlace) outcome {
	if place.hook == "" {
		return outcome{
			state: stateUnsupported,
			note:  item.harness.title + " has no session-start hook format Ze knows, so the stub alone sets it up.",
		}
	}
	switch r.verb {
	case verbCreate:
		return createHook(item.harness, place.hook, r.rerun)
	case verbShow:
		return inspectHook(item.harness, place.hook, r.rerun)
	case verbDelete:
		return deleteHook(item.harness, place.hook, r.rerun)
	case verbUnspecified:
		panic("BUG: a local run with no verb")
	}
	panic("BUG: a local run with an unknown verb")
}

// note says what the run did, above its rows.
func (r *localRun) note(acted []*harness) string {
	switch r.verb {
	case verbCreate:
		if len(acted) == 0 {
			return "No agent harness was found under " + r.home + ", so Ze wrote only the shared stub, " +
				"the one " + titlesOf(sharedReaders()) + " read. Name a harness with agent to set it up in full."
		}
		return "Ze set the ze skill up for each harness below. A refused row names its fix."
	case verbShow:
		return "What Ze finds of its setup for each agent harness it knows. Nothing was changed."
	case verbDelete:
		return "Ze removed what it installed for each harness below, and left every file it did not write."
	case verbUnspecified:
		panic("BUG: a local run with no verb")
	}
	panic("BUG: a local run with an unknown verb")
}

// exitCode answers 1 when any row was refused, so a script sees that the
// setup is not what was asked for, and 0 otherwise.
func exitCode(answer agentSkillAnswer) int {
	for _, row := range answer.Setup.Rows {
		if row.State == stateRefused {
			return 1
		}
	}
	return 0
}

// localAnswer is the shell route of the three commands, as a value.
func localAnswer(verb setupVerb, path string, validated command.ValidatedArgs) command.LocalAnswer {
	run, err := newLocalRun(verb, path, validated)
	if err != nil {
		return command.LocalRefusal(1, command.LocalErrorLine(err))
	}
	answer := run.answer()
	return command.LocalAnswer{Payload: answer, Code: exitCode(answer)}
}

// createAgentSkillLocal is the shell route of create agent skill.
func createAgentSkillLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswer(pathCreateAgentSkill, localAnswer(verbCreate, pathCreateAgentSkill, validated))
}

// showAgentSkillLocal is the shell route of show agent skill.
func showAgentSkillLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswer(pathShowAgentSkill, localAnswer(verbShow, pathShowAgentSkill, validated))
}

// deleteAgentSkillLocal is the shell route of delete agent skill.
func deleteAgentSkillLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswer(pathDeleteAgentSkill, localAnswer(verbDelete, pathDeleteAgentSkill, validated))
}

// --- The drawing ------------------------------------------------------------

// drawAgentSkill draws the answer of the three commands: the note, then per
// row its agent, item, state and harness, its path, its note indented, and,
// on the daemon route, the content to place, as it is.
func drawAgentSkill(raw json.RawMessage) (string, error) {
	var setup agentSetup
	if err := json.Unmarshal(raw, &setup); err != nil {
		return "", err
	}
	var b textbuf.Buffer
	b.Str(setup.Note).Byte('\n')
	for _, row := range setup.Rows {
		b.Byte('\n').Str(row.Agent).Byte(' ').Str(row.Item).Str(": ").Str(row.State)
		b.Str(" (harness ").Str(row.Harness).Str(")\n")
		if row.Path != "" {
			b.Str("  ").Str(row.Path).Byte('\n')
		}
		if row.Note != "" {
			for line := range bytes.SplitSeq([]byte(row.Note), []byte{'\n'}) {
				b.Str("  ").Str(string(line)).Byte('\n')
			}
		}
		if row.Content != "" {
			b.Str("  content to place:\n").Str(row.Content)
		}
	}
	return b.String(), nil
}
