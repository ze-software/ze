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

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// requireEveryItemHasText panics naming the first item of roots that carries
// no text: an area with no summary, a pass-through area that names no program,
// an action with no Why, a live parameter with no Help, a declared value with
// no ValueHelp line, or a ValueHelp line for a value the parameter does not
// declare. Each is a Ze defect only the area's author can fix, so it is a BUG
// panic at startup rather than a blank line in help.
func requireEveryItemHasText(roots []registry.RootCommand) {
	for _, root := range roots {
		if blank(root.Meta.ShortHelp) {
			refuse(root.Name, "area \""+root.Name+"\" has no summary",
				"give its leroot.Register call a Meta ShortHelp saying what the area is for, in one sentence")
		}
		list, declared := ActionsOf(root.Name)
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
	if blank(row.Why) {
		refuse(area, "action \""+action+"\" has no Why",
			"fill Why in its leaction.Action, saying what the action does")
	}
	for _, parameter := range row.Parameters {
		if parameter.Retired != "" {
			continue
		}
		slot := action + " " + slotName(parameter)
		if blank(parameter.Help) {
			refuse(area, "parameter \""+slot+"\" has no Help",
				"fill Help in its leaction.Parameter, saying what the keyword does")
		}
		for _, value := range parameter.Values {
			if blank(parameter.ValueHelp[value]) {
				refuse(area, "value \""+value+"\" of \""+slot+"\" has no description",
					"add \""+value+"\": \"<what choosing it does>\" to that parameter's ValueHelp")
			}
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
