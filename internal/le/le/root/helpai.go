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
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// aiWord is the one reserved word after the help word: `le help ai`.
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
				Str(program).Str(" help ai | json").String())
		}
	}
	return Run("", func(validated command.ValidatedArgs) (any, int) {
		return answerHelpAI(program, validated.Tokens())
	}, args, os.Stdout, os.Stderr)
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
				Str("; it holds: ").Join(sectionNames(), " | ").String())
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
		Str("usage: ").Str(program).Str(" help ai [").Join(sectionNames(), "|").Str("] [| json | yaml]\n").
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

// actionOf projects one action row and its live parameters. A retired keyword
// is refused, never offered, so it is not part of the grammar a reader learns.
func actionOf(row leaction.Row) reference.Action {
	action := reference.Action{Verb: row.Verb, ShortHelp: row.Summary, Description: row.Why, Writes: row.Writes}
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
