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
	"github.com/ze-software/ze/internal/le/leaction"
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
	// Group is what the command is FOR, and it decides the section the page
	// renders it under. An area registered by a route other than Register
	// declares none, and it renders under Ungrouped.
	Group Group `json:"group"`
	// Description is the one-line summary the registration carries.
	Description string `json:"description"`
	// Actions is the area's whole action table, each row with its keyword
	// grammar. An area that hand-rolls its dispatch registers no table, and the
	// key is then absent rather than empty: absent says the area declared
	// nothing, which is a different fact from an area that declared no action.
	Actions []leaction.Row `json:"actions,omitempty"`
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
		area := ManifestArea{Name: root.Name, Description: root.Meta.Description}
		if group, declared := GroupOf(root.Name); declared {
			area.Group = group
		}
		if list, declared := ActionsOf(root.Name); declared {
			area.Actions = list.Actions
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
		Command:  m.Program,
		Summary:  m.Summary,
		Usage:    []string{tb.Str(m.Program).Str(" <command> [options] [| json | yaml | table]").String()},
		Sections: m.sections(),
	}
}

// sections splits the areas into one help section for each group, in render
// order.
//
// An area whose group is unknown still prints. It goes in a final section of
// its own. A help page that hides a command is worse than one that files a
// command badly. Only a registration that bypassed Register can produce one.
func (m Manifest) sections() []helpfmt.HelpSection {
	byGroup := make(map[Group][]helpfmt.HelpEntry, len(groupOrder))
	ungrouped := make([]helpfmt.HelpEntry, 0)
	for _, area := range m.Areas {
		entry := helpfmt.HelpEntry{Name: area.Name, Desc: area.Description}
		if !KnownGroup(area.Group) {
			ungrouped = append(ungrouped, entry)
			continue
		}
		byGroup[area.Group] = append(byGroup[area.Group], entry)
	}

	sections := make([]helpfmt.HelpSection, 0, len(groupOrder)+1)
	for _, group := range groupOrder {
		entries := byGroup[group]
		if len(entries) == 0 {
			continue
		}
		sections = append(sections, helpfmt.HelpSection{Title: GroupTitle(group), Entries: entries})
	}
	if len(ungrouped) != 0 {
		sections = append(sections, helpfmt.HelpSection{Title: "Ungrouped", Entries: ungrouped})
	}
	return sections
}
