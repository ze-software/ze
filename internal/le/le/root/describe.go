// Design: docs/architecture/core-design.md -- le's command surface as one tree
// Related: leroot.go -- checkRegistrations, which runs this check once
// Related: namespacesummary.go -- the same refusal for a namespace
//
// Every word le offers says what it is for: an area, an action, a keyword and
// each value an enum or a closed list declares. Help, completion, the menu and
// `le help ai` print that text beside the word, so a blank one is a word an
// operator has to guess at. The check walks what the areas registered and
// refuses to start, naming the item and the file the text goes in.
package leroot

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// shortMax is the most characters a short text holds: the menu's right column
// beside an 18-column name fits it on an 80-column terminal, and so do the
// root page and completion.
const shortMax = 60

// requireEveryItemHasText panics naming the first item of roots whose text
// breaks the rule every level shares: a short of one line, at most shortMax
// characters, on every item (an area's Meta ShortHelp, an action's Summary, a
// keyword's Help, a value's ValueHelp line), and a long text on every item a
// reader can enter or open a page for (an area's Meta Description, an action's
// Why; a pass-through area's long text is the program its words go to). A
// ValueHelp line for a value the parameter does not declare is refused too.
// Each is a Ze defect only the area's author can fix, so it is a BUG panic at
// startup rather than a blank or overlong line in help and the menu.
func requireEveryItemHasText(roots []registry.RootCommand) {
	for _, root := range roots {
		requireShort(root.Name, "area \""+root.Name+"\"", "summary", root.Meta.ShortHelp,
			"give its leroot.Register call a Meta ShortHelp saying what the area is for")
		list, declared := ActionsOf(root.Name)
		if !declared || !list.PassesWords {
			if blank(root.Meta.Description) {
				refuse(root.Name, "area \""+root.Name+"\" has no description",
					"give its leroot.Register call a Meta Description: what the area holds, in one to three sentences")
			}
		}
		if !declared {
			continue
		}
		// A pass-through area's text is the line naming where its words go.
		if list.PassesWords {
			if blank(list.PassesTo) {
				refuse(root.Name, "pass-through area \""+root.Name+"\" names no program its words pass to",
					"name the program in its leroot.RegisterForwarding call")
			}
			continue
		}
		for _, row := range list.Actions {
			requireRowHasText(root.Name, row)
		}
	}
}

// requireRowHasText refuses the first blank text of one action row: its Why,
// a live parameter's Help, then each declared value's line.
func requireRowHasText(area string, row leaction.Row) {
	action := area + " " + row.Verb
	requireShort(area, "action \""+action+"\"", "Summary", row.Summary,
		"fill Summary in its leaction.Action, saying in one line what the action does")
	if blank(row.Why) {
		refuse(area, "action \""+action+"\" has no Why",
			"fill Why in its leaction.Action, saying what the action does")
	}
	for _, parameter := range row.Parameters {
		if parameter.Retired != "" {
			continue
		}
		slot := action + " " + slotName(parameter)
		requireShort(area, "parameter \""+slot+"\"", "Help", parameter.Help,
			"fill Help in its leaction.Parameter, saying in one line what the keyword does")
		for _, value := range parameter.Values {
			if blank(parameter.ValueHelp[value]) {
				refuse(area, "value \""+value+"\" of \""+slot+"\" has no description",
					"add \""+value+"\": \"<what choosing it does>\" to that parameter's ValueHelp")
			}
			requireShort(area, "value \""+value+"\" of \""+slot+"\"", "ValueHelp line", parameter.ValueHelp[value],
				"shorten its line in that parameter's ValueHelp")
		}
		for value := range parameter.ValueHelp {
			if slices.Contains(parameter.Values, value) {
				continue
			}
			refuse(area, "parameter \""+slot+"\" describes \""+value+"\", which is not one of its Values",
				"delete that ValueHelp entry, or add the value to Values")
		}
	}
}

// requireShort refuses a short text that is blank, spans more than one line,
// or holds more than shortMax characters, naming the item and the field.
func requireShort(area, item, field, text, fix string) {
	if blank(text) {
		refuse(area, item+" has no "+field, fix)
	}
	if strings.ContainsRune(text, '\n') {
		refuse(area, item+" has a "+field+" of more than one line", fix)
	}
	if utf8.RuneCountInString(text) > shortMax {
		var tb textbuf.Buffer
		refuse(area, tb.Str(item).Str(" has a ").Str(field).Str(" of ").Int(int64(utf8.RuneCountInString(text))).
			Str(" characters, over the ").Int(shortMax).Str(" a menu line holds").String(), fix)
	}
}

// requireShortAndLong refuses a namespace or other enterable item whose short
// text breaks requireShort or whose long text is blank; where names the call
// that declares both.
func requireShortAndLong(item, short, long, area, where string) {
	requireShort(area, item, "summary", short, "fix the summary in "+where)
	if blank(long) {
		refuse(area, item+" has no description", "add one to three sentences to "+where)
	}
}

// refuse panics with the defect, why it stops le, and where the fix goes.
func refuse(area, what, fix string) {
	var tb textbuf.Buffer
	message := tb.Str(what).
		Str(", so help, completion and the menu would print it blank: ").Str(fix).
		Str(" (internal/le/").Str(strings.ReplaceAll(area, " ", "/")).Str("/)").String()
	panic("BUG: le " + message)
}

// slotName answers the word a parameter is known by: its keyword, or its
// value name for the positional value.
func slotName(parameter leaction.Parameter) string {
	if parameter.Keyword != "" {
		return parameter.Keyword
	}
	return "<" + parameter.Value + ">"
}

// blank reports whether text holds nothing a reader can see.
func blank(text string) bool {
	return strings.TrimSpace(text) == ""
}
