// Design: docs/architecture/fleet-interface.md -- operator text is escaped wherever a component writes it

package fleetui

import (
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

// hostileText is operator text that opens an element and breaks out of an
// attribute when a component writes it unescaped: a device name, a label, a
// task target, a toast, a command answer or a confirmation. Every
// component that writes operator text renders it in a "hostile" golden
// variant, and TestHostileTextIsEscaped refuses any element or attribute it
// would create.
const hostileText = `"><img src=x onerror=alert(1)>'`

// hostileHref is an address carrying hostileText in its query.
const hostileHref = "/fleet/estate/devices/?q=" + hostileText

// hostileTableRow answers a row of table whose name, id, eye address and
// first cell carry hostileText.
func hostileTableRow(table TableView) RowView {
	row := table.Rows[0]
	row.ID = hostileText
	row.Name = hostileText
	row.EyeHref = hostileHref
	row.Cells = slices.Clone(row.Cells)
	row.Cells[0] = CellView{Kind: CellText, Text: hostileText, Title: hostileText, Href: hostileHref}

	return row
}

// hostileTreeRow answers a tree row whose key, title and address carry
// hostileText.
func hostileTreeRow(base TreeRowView) TreeRowView {
	base.Key = hostileText
	base.Title = hostileText
	base.Href = hostileHref
	base.Children = nil

	return base
}

// hostileToast is a toast whose every line and link carries hostileText.
func hostileToast() ToastView {
	return ToastView{
		Level: ToastError, Message: hostileText, Detail: hostileText,
		Link: &LinkView{Label: hostileText, Href: hostileHref},
	}
}

// hostileTerminalEntry is a command and an answer that both carry
// hostileText, the answer also trying to close its block.
func hostileTerminalEntry() TerminalEntryView {
	return TerminalEntryView{Command: hostileText, Output: hostileText + "\n</pre><script>alert(1)</script>"}
}

// hostileAction is a confirmed action whose label, field, reason and every
// line of its confirmation carry hostileText.
func hostileAction() ActionView {
	return ActionView{
		Label: hostileText, Form: manageActionFormID, Name: paramItem, Value: hostileText, Reason: hostileText,
		Confirm: &ConfirmView{Action: hostileText, Targets: hostileText, Detail: hostileText, Expect: hostileText, Destructive: true},
	}
}

// hostileSearch is a search answer whose text and result carry hostileText.
func hostileSearch(found bool) SearchResultsView {
	view := SearchResultsView{Text: hostileText}
	if found {
		view.Results = []SearchResultView{{Title: hostileText, Detail: hostileText, Href: hostileHref, Icon: iconCheck}}
	}

	return view
}

// hostileComponents answers every component a hostile variant renders.
func hostileComponents(t *testing.T) map[string]templ.Component {
	t.Helper()

	table := NewGalleryView().Tables.Full.Table
	tree := goldenTree(t, "")

	return map[string]templ.Component{
		"tableRow":            tableRow(table, hostileTableRow(table)),
		"treeRow":             treeRow(hostileTreeRow(tree.Rows[0])),
		"Toast":               Toast(hostileToast()),
		"TerminalEntry":       TerminalEntry(hostileTerminalEntry()),
		"actionControl":       actionControl(hostileAction(), actionLookButton),
		"searchResults found": searchResults(hostileSearch(true)),
		"searchResults none":  searchResults(hostileSearch(false)),
	}
}

// TestHostileTextIsEscaped renders every component that writes operator text
// with text that would open an element or break out of an attribute, and
// reads the markup back as a browser would.
//
// VALIDATES: operator text reaches the markup as text or as one attribute's
// value, never as an element or an attribute of its own.
// PREVENTS: a future templ.Raw, a string-built attribute or a script-built
// fragment that turns a device name into markup, which no other fixture
// would show, since none carries hostile text.
func TestHostileTextIsEscaped(t *testing.T) {
	for name, component := range hostileComponents(t) {
		t.Run(name, func(t *testing.T) {
			markup := componentMarkup(t, component)

			for _, opened := range []string{"<img", "<script"} {
				if strings.Contains(markup, opened) {
					t.Fatalf("%s writes the operator's text as markup: %q appears in\n%s", name, opened, markup)
				}
			}

			reached := false
			tokens := html.NewTokenizer(strings.NewReader(markup))
			for {
				kind := tokens.Next()
				if kind == html.ErrorToken {
					break
				}
				token := tokens.Token()
				switch kind {
				case html.TextToken:
					if strings.Contains(token.Data, hostileText) {
						reached = true
					}
				case html.StartTagToken, html.SelfClosingTagToken:
					if token.Data == "img" || token.Data == "script" {
						t.Fatalf("%s opens a <%s> element from the operator's text:\n%s", name, token.Data, markup)
					}
					for _, attribute := range token.Attr {
						if strings.HasPrefix(attribute.Key, "on") || attribute.Key == "src" || strings.ContainsAny(attribute.Key, `"'<>`) {
							t.Fatalf("%s breaks out of an attribute: <%s> carries %q:\n%s", name, token.Data, attribute.Key, markup)
						}
						if strings.Contains(attribute.Val, hostileText) {
							reached = true
						}
					}
				case html.EndTagToken, html.CommentToken, html.DoctypeToken, html.ErrorToken:
					// An end tag, a comment or a doctype carries no operator text
					// this test reads; the error token ended the loop above.
				}
			}

			if !reached {
				t.Fatalf("%s never wrote the hostile text, so the case proves nothing:\n%s", name, markup)
			}
		})
	}
}
