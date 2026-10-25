// Design: docs/architecture/core-design.md -- le's whole command surface, as data
// Overview: dispatch.go -- the dispatcher that answers this payload and renders it
// Related: leroot.go -- the registration adapter an area and its actions join through
//
// The root used to write its help straight to stderr, so the one thing a reader
// or an agent could not ask le was what le holds: `le | json` answered `unknown
// command: |`. Here that page is a payload, and the text is a rendering OF the
// payload rather than a second copy of it.

package leroot

import (
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// rootSummary is what le is, on the header line of the root page.
const rootSummary = "the Ze repository and development entry point"

// Manifest is le's whole command surface: every registered area, what it is
// for, the group it renders under, and the actions it declared.
type Manifest struct {
	// Program is the name the reader types, which is "le" for the binary and
	// "ze le" in a ze_le build. Every usage line the page prints is that name.
	Program string `json:"program"`
	// Summary is the one line under the program name.
	Summary string `json:"summary"`
	// Areas is every registered command, in the order the registry lists them,
	// which is by full path.
	Areas []ManifestArea `json:"areas"`
}

// ManifestArea is one registered le command and what it declared about itself.
type ManifestArea struct {
	// Name is the word, or the two words, typed after the program name.
	Name string `json:"name"`
	// Group is what the command is FOR, and the page renders it as the
	// line's purpose tag. An area registered by a route other than Register
	// declares none, and its tag reads ungrouped.
	Group Group `json:"group"`
	// Description is the one-line summary the registration carries.
	Description string `json:"description"`
	// Actions is the area's whole action table, each row with its keyword
	// grammar. An area that hand-rolls its dispatch registers no table, and the
	// key is then absent rather than empty: absent says the area declared
	// nothing, which is a different fact from an area that declared no action.
	Actions []leaction.Row `json:"actions,omitempty"`
	// PassesWords says every word after the name is another program's command
	// line (RegisterForwarding), so the program, not le, reads and refuses it,
	// and its exit code is the area's. Actions is then absent: the area's
	// table is the pass-through declaration (leaction.PassesWords).
	PassesWords bool `json:"passes-words,omitempty"`
	// PassesTo names that program, as help prints it.
	PassesTo string `json:"passes-to,omitempty"`
}

// manifestOf answers the surface as it stands in the shared registry. It is
// unexported because the surface leaves this package as a rendering: Text for a
// person, and the payload through Run for `| json` and its siblings.
func manifestOf(program string) Manifest {
	return manifestFrom(program, Commands())
}

// manifestFrom builds the manifest over one command set. The set is a parameter
// so a test can pin a page byte for byte, which the live registry cannot: every
// registration in the process is in it.
func manifestFrom(program string, roots []registry.RootCommand) Manifest {
	manifest := Manifest{
		Program: program,
		Summary: rootSummary,
		Areas:   make([]ManifestArea, 0, len(roots)),
	}
	for _, root := range roots {
		area := ManifestArea{Name: root.Name, Description: root.Meta.ShortHelp}
		if group, declared := GroupOf(root.Name); declared {
			area.Group = group
		}
		if list, declared := ActionsOf(root.Name); declared {
			area.Actions = list.Actions
			area.PassesWords = list.PassesWords
			area.PassesTo = list.PassesTo
		}
		manifest.Areas = append(manifest.Areas, area)
	}
	return manifest
}

// Text renders the manifest as the page a person reads, which is the root help.
// It is the Prose rendering leroot.Run prints when the operator typed no pipe
// operator, so the color decision is stdout's, as WriteErr's is stderr's.
func (m Manifest) Text() string {
	page := m.page()
	page.Width = leaction.PageWidth(os.Stdout)

	var out strings.Builder
	page.WriteTo(&out, slogutil.UseColor(os.Stdout))
	return out.String()
}

// page builds the help page for the manifest. It is the one declaration of what
// the root page says: Text renders it for stdout and Usage renders it for
// stderr, so a refusal and the payload cannot describe two command sets.
func (m Manifest) page() helpfmt.Page {
	var tb textbuf.Buffer
	return helpfmt.Page{
		Command:   m.Program,
		ShortHelp: m.Summary,
		Usage:     []string{tb.Str(m.Program).Str(" <command> [options] [| json | yaml | table]").String()},
		Sections:  m.sections(),
	}
}

// sections lays the areas out by tree: one section for the single-word
// commands, then one per namespace (the first word two-word commands share), in
// the registry's order, which is by full path. A single-word command that is
// also a namespace token lists in its namespace, so every area appears once.
// Each line is the name, the purpose tag, and the first sentence of the
// summary (entryText). A namespace's heading carries the first sentence of
// what the namespace is for (namespaceLine).
func (m Manifest) sections() []helpfmt.HelpSection {
	namespaces := make(map[string]bool, len(m.Areas))
	for _, area := range m.Areas {
		if word, _, nested := strings.Cut(area.Name, " "); nested {
			namespaces[word] = true
		}
	}

	single := helpfmt.HelpSection{Title: "Commands"}
	byNamespace := make(map[string]int, len(namespaces))
	sections := make([]helpfmt.HelpSection, 1, len(namespaces)+1)
	for _, area := range m.Areas {
		entry := helpfmt.HelpEntry{Name: area.Name, Desc: areaText(area)}
		word, _, _ := strings.Cut(area.Name, " ")
		if !namespaces[word] {
			single.Entries = append(single.Entries, entry)
			continue
		}
		index, seen := byNamespace[word]
		if !seen {
			index = len(sections)
			byNamespace[word] = index
			sections = append(sections, helpfmt.HelpSection{
				Title: word, Summary: leaction.FirstSentence(namespaceLine(word, m.Areas)),
			})
		}
		sections[index].Entries = append(sections[index].Entries, entry)
	}
	sections[0] = single
	return sections
}

// areaText is one area's line on the root page: the purpose tag the area
// declared, then the first sentence of its summary.
func areaText(area ManifestArea) string {
	tag := ungroupedTag
	if KnownGroup(area.Group) {
		tag = string(area.Group)
	}
	var tb textbuf.Buffer
	return tb.Byte('[').Str(tag).Str("] ").Str(leaction.FirstSentence(area.Description)).String()
}
