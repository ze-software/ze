// Design: docs/contributing/running-commands.md -- `le help ai`, the reference an agent reads
// Related: manifest.go -- the areas and action tables the commands section reads
// Related: group.go -- the purpose tags the groups section reads
// Related: exitcode.go -- the exit codes the exit-codes section reads
//
// `le help ai` answers le's whole surface in one shape an agent reads, through
// the pipe layer like every other answer: `le help ai | json`, `| yaml`. It
// takes no rendering flag (ai/rules/cli.md), so a dash-leading word is refused
// by name and the refusal names the pipe that does what it asked.
package leroot

import (
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// aiWord is the one reserved word after the help word: `le help ai` is the
// agent reference, and `le help ai <command> ...` pages a command of the `ai`
// namespace (asksAIReference decides which).
const aiWord = "ai"

// The section words, in the order the index lists them.
const (
	sectionCommands  = "commands"
	sectionGroups    = "groups"
	sectionVerbs     = "verbs"
	sectionExitCodes = "exit-codes"
	sectionLauncher  = "launcher"
	sectionAll       = "all"
)

// aiSections declares every section, its meaning, and how it fills a
// reference. `all` is not a row: it selects every row.
var aiSections = []struct {
	name    string
	meaning string
	fill    func(ref *reference.Reference, program string)
}{
	{sectionCommands, "every command, each action, each parameter with its kind, values, required and repeat", fillCommands},
	{sectionGroups, "the purpose tags a command declares, and what each means", fillGroups},
	{sectionVerbs, "the closed verb vocabulary: each verb, its kind, its meaning, and the names that declare it", fillVerbs},
	{sectionExitCodes, "what each exit code means, and the commands that answer another program's code", fillExitCodes},
	{sectionLauncher, "the words the ./le launcher reads before le starts, and what each does", fillLauncher},
}

// asksAIReference reports whether words, the line after the help word, ask
// for the agent reference rather than the page of an ai command: `ai` alone,
// or `ai` before a section, an option, a pipe, or any word that is the first
// word of no command under the ai namespace. The reference then refuses that
// word, naming its sections and the ai commands (answerHelpAI). Dispatch,
// retiredLine and JudgeCitation all decide through it, so a run, Tab and a
// citation read one line the same way.
func asksAIReference(words []string) bool {
	if len(words) == 0 {
		return false
	}
	if words[0] != aiWord {
		return false
	}
	if len(words) == 1 {
		return true
	}
	return !namesAICommand(words[1])
}

// namesAICommand reports whether word, typed after `help ai`, is the first
// word of a registered command under the ai namespace, hidden ones included,
// as `le help <hidden command>` pages one. A section word is answered first
// and never names a command: requireNoAICommandShadowsASection refuses an ai
// command spelled like a section, so the two sets cannot overlap.
func namesAICommand(word string) bool {
	if slices.Contains(sectionNames(), word) {
		return false
	}
	prefix := textbuf.New().Str(aiWord).Byte(' ').Str(word).String()
	for _, command := range Commands() {
		if command.Name == prefix || strings.HasPrefix(command.Name, prefix+" ") {
			return true
		}
	}
	return false
}

// aiCommandWords answers the distinct first words of the ai namespace's
// visible members, in the registry's order: what may follow `help ai` besides
// a section.
func aiCommandWords() []string {
	words := make([]string, 0, 8)
	for _, member := range members(aiWord) {
		first, _, _ := strings.Cut(member, " ")
		if !slices.Contains(words, first) {
			words = append(words, first)
		}
	}
	return words
}

// requireNoAICommandShadowsASection panics naming the first command under the
// ai namespace whose first word after `ai` is a section of the reference.
// `le help ai <section>` answers the section, so such a command's help page
// would have no spelling after `help ai`: a Ze defect only its author can fix,
// by naming the command another word.
func requireNoAICommandShadowsASection(names []string) {
	for _, name := range names {
		rest, under := strings.CutPrefix(name, aiWord+" ")
		if !under {
			continue
		}
		first, _, _ := strings.Cut(rest, " ")
		if slices.Contains(sectionNames(), first) {
			panic("BUG: le command \"" + name + "\" is spelled like the `le help ai " + first +
				"` section, so `le help ai " + rest + "` cannot page it: name the command another word")
		}
	}
}

// helpAI answers `le help ai [<section>...]`, rendered by the pipe the
// reader typed after it.
func helpAI(program string, args []string) int {
	// A dash-leading word is refused before the engine judges argv, so the
	// refusal names the pipe rather than an unknown flag.
	own, _ := splitChain(args)
	for _, word := range own {
		if strings.HasPrefix(word, "-") {
			return refuseHelpAI(program, textbuf.New().Str(word).
				Str(" is not an option of help ai: le takes no rendering flag; render with a pipe: ").
				Str(program).Str(" help ai '|' json").String())
		}
	}
	return Run("", toolHandler(func(words []string) (any, int) {
		return answerHelpAI(program, words)
	}), args, os.Stdout, os.Stderr)
}

// answerHelpAI builds the reference for the selected sections, or refuses a
// word that names none.
func answerHelpAI(program string, words []string) (any, int) {
	ref := reference.Reference{Program: program, Summary: rootSummary}
	for _, section := range aiSections {
		ref.Sections = append(ref.Sections, reference.Entry{Name: section.name, Meaning: section.meaning})
	}
	ref.Sections = append(ref.Sections, reference.Entry{Name: sectionAll, Meaning: "every section above"})

	selected := map[string]bool{}
	for _, word := range words {
		if word == sectionAll {
			for _, section := range aiSections {
				selected[section.name] = true
			}
			continue
		}
		if !slices.Contains(sectionNames(), word) {
			return nil, refuseHelpAI(program, textbuf.New().Str("help ai has no section ").Str(word).
				Str(" and the ai namespace holds no command ").Str(word).
				Str("; its sections: ").Join(sectionNames(), " | ").
				Str("; the ai commands it pages: ").Join(aiCommandWords(), " | ").String())
		}
		selected[word] = true
	}
	for _, section := range aiSections {
		if selected[section.name] {
			section.fill(&ref, program)
		}
	}
	return ref, 0
}

// sectionNames answers every word help ai accepts, `all` last.
func sectionNames() []string {
	names := make([]string, 0, len(aiSections)+1)
	for _, section := range aiSections {
		names = append(names, section.name)
	}
	return append(names, sectionAll)
}

// refuseHelpAI prints the refusal and the usage line, and answers the usage
// code.
func refuseHelpAI(program, why string) int {
	var tb textbuf.Buffer
	tb.Str("error: ").Str(why).Byte('\n').
		Str("usage: ").Str(program).Str(" help ai [").Join(sectionNames(), "|").Str("] ['|' json|yaml]\n").
		Str("       ").Str(program).Str(" help ai <command> [<action> [<keyword>]]\n").
		StdErr() //nolint:errcheck // CLI output
	return 2
}

// fillCommands copies every area of the manifest, so the count is the
// manifest's by construction, and every namespace those areas share with the
// line that describes it (namespaceLine), in the order the root page lists them.
func fillCommands(ref *reference.Reference, program string) {
	areas := manifestOf(program).Areas
	for _, section := range (Manifest{Areas: areas}).sections()[1:] {
		description, _ := NamespaceDescription(section.Title)
		ref.Namespaces = append(ref.Namespaces, reference.Namespace{
			Name: section.Title, ShortHelp: namespaceLine(section.Title, areas), Description: description,
		})
	}
	for _, area := range areas {
		entry := reference.Command{Name: area.Name, Group: string(area.Group), ShortHelp: area.ShortHelp, Description: area.Description, PassesTo: area.PassesTo, Hidden: area.Hidden}
		if area.Group == "" {
			entry.Group = ungroupedTag
		}
		for _, row := range area.Actions {
			entry.Actions = append(entry.Actions, actionOf(row))
		}
		ref.Commands = append(ref.Commands, entry)
	}
}

// actionOf projects one action row, the vocabulary verb it declares it
// performs, and its live parameters. A retired keyword is refused, never
// offered, so it is not part of the grammar a reader learns.
func actionOf(row leaction.Row) reference.Action {
	action := reference.Action{Verb: row.Verb, Does: row.Does, ShortHelp: row.Summary, Description: row.Why, Writes: row.Writes}
	for _, parameter := range row.Parameters {
		if parameter.Retired != "" {
			continue
		}
		action.Parameters = append(action.Parameters, reference.Parameter{
			Keyword:   parameter.Keyword,
			Value:     parameter.Value,
			Kind:      parameter.Kind.String(),
			Values:    parameter.Values,
			ValueHelp: parameter.ValueHelp,
			Pair:      parameter.Pair,
			Required:  parameter.Requirement == leaction.Required,
			Repeat:    parameter.Repeat,
			Help:      parameter.Help,
		})
	}
	return action
}

// fillGroups copies the purpose-tag table, in its order, each group with the
// line it is declared with. It reads the table itself, so no group can reach
// the page without its meaning.
func fillGroups(ref *reference.Reference, _ string) {
	for _, row := range groupTable {
		ref.Groups = append(ref.Groups, reference.Entry{Name: string(row.group), Meaning: row.meaning})
	}
}

// fillVerbs copies the closed vocabulary (leaction.Vocabulary), in its
// declared order, each verb with its kind, its meaning, and every name that
// declares it performs that verb (Action.Does). It reads the vocabulary and the
// manifest themselves, so no verb reaches the page without its meaning and no
// declaring name is left off.
func fillVerbs(ref *reference.Reference, program string) {
	declaredBy := map[string][]string{}
	for _, area := range manifestOf(program).Areas {
		for _, row := range area.Actions {
			if row.Does == "" {
				continue
			}
			// A verbless action is run by its area word alone (D-P4-areaword),
			// so the area word is the name that declares the verb.
			name := area.Name
			if row.Verb != "" {
				name = textbuf.New().Str(area.Name).Byte(' ').Str(row.Verb).String()
			}
			declaredBy[row.Does] = append(declaredBy[row.Does], name)
		}
	}
	for _, verb := range leaction.Vocabulary() {
		ref.Verbs = append(ref.Verbs, reference.Verb{
			Name: verb.Word, Kind: verb.Kind.String(), Meaning: verb.Meaning, Names: declaredBy[verb.Word],
		})
	}
}

// fillExitCodes copies the exit-code table.
func fillExitCodes(ref *reference.Reference, _ string) {
	ref.ExitCodes = append(ref.ExitCodes, exitCodes()...)
}
