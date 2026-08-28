package web

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/le/webassets"
)

// The tests here hold the markup contracts no single component owns. Each one
// walks a whole population: every .templ source, every captured fixture, or one
// rendered document. A defect that lives in the relation between two components
// is invisible to a test that renders one of them.

// htmxSwapStyles is every swap htmx implements, plus the bare "true" that
// names outerHTML. It is read from the vendored assets/htmx.min.js.
// TestHTMXSwapStylesMatchTheVendoredLibrary re-reads it, so an upgrade that
// drops one is a red test rather than a widened allow list.
var htmxSwapStyles = []string{
	"innerHTML", "outerHTML", "beforebegin", "afterbegin",
	"beforeend", "afterend", "delete", "none", "true",
}

// oobAttrPattern finds one out-of-band swap value in template source. templ
// writes an expression attribute with double quotes only, so a literal value is
// always in this form.
var oobAttrPattern = regexp.MustCompile(`hx-swap-oob="([^"]*)"`)

// idAttrPattern finds one id attribute in rendered markup.
var idAttrPattern = regexp.MustCompile(`\bid="([^"]*)"`)

// classAttrPattern finds one class attribute in rendered markup.
var classAttrPattern = regexp.MustCompile(`\bclass="([^"]*)"`)

// markupHasClass reports whether any element in markup carries the class.
//
// It reads the classes inside a class attribute rather than matching the whole
// attribute. An element names as many classes as it needs. A component that
// adds a modifier beside its base class still satisfies the selector. Matching
// class="<name>" verbatim went red on markup the browser still selects.
func markupHasClass(markup, class string) bool {
	for _, match := range classAttrPattern.FindAllStringSubmatch(markup, -1) {
		if slices.Contains(strings.Fields(match[1]), class) {
			return true
		}
	}

	return false
}

// TestOutOfBandSwapValuesNameASwapHTMXImplements verifies every hx-swap-oob
// literal in the component set names a swap the vendored htmx performs.
//
// VALIDATES: an out-of-band swap this repo writes actually swaps.
// PREVENTS: an attribute that reads as intent and does nothing. The error
// drawer carried hx-swap-oob with the value className. htmx implements no swap
// by that name, so a rejected edit appended to a list no operator sees. htmx
// reports an unknown swap style nowhere. It falls through, and the page reads
// as unresponsive.
func TestOutOfBandSwapValuesNameASwapHTMXImplements(t *testing.T) {
	allowed := make(map[string]bool, len(htmxSwapStyles))
	for _, style := range htmxSwapStyles {
		allowed[style] = true
	}

	sources, err := filepath.Glob(filepath.Join(".", "*.templ"))
	require.NoError(t, err)
	require.NotEmpty(t, sources, "the component set must not be empty")

	seen := 0

	for _, path := range sources {
		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading its own package
		require.NoError(t, readErr)

		for _, match := range oobAttrPattern.FindAllStringSubmatch(string(body), -1) {
			seen++

			// htmx accepts "<style>:<selector>" to swap somewhere else.
			style, _, _ := strings.Cut(match[1], ":")
			assert.True(t, allowed[style],
				"%s writes hx-swap-oob=%q and htmx implements no swap named %q", path, match[1], style)
		}
	}

	assert.Positive(t, seen, "no out-of-band swap was scanned; the pattern has stopped matching")
}

// TestHTMXSwapStylesMatchTheVendoredLibrary verifies the allow list above is
// grounded in the library that ships, not in a memory of it.
//
// VALIDATES: each accepted swap name is present in assets/htmx.min.js.
// PREVENTS: an htmx upgrade removing a swap style while the guard above keeps
// accepting it, which would turn that guard into a list of words.
func TestHTMXSwapStylesMatchTheVendoredLibrary(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("assets", "htmx.min.js"))
	require.NoError(t, err)

	for _, style := range htmxSwapStyles {
		assert.Contains(t, string(body), style,
			"assets/htmx.min.js does not mention the swap style %q", style)
	}
}

// TestRenderedPageCarriesNoDuplicateDOMID verifies one document names each id
// once.
//
// VALIDATES: no two components in one document claim the same id, over both
// shells: the Finder split, and the workbench shell around a page.
// PREVENTS: document.getElementById answering the wrong element. Four
// components hard-coded btn-add-entry, and mainSplit renders two of them side
// by side, so the add control inside the content area was unreachable by id.
// Rendering the Finder alone left the workbench composition unchecked, and the
// workbench table draws an add control of its own.
func TestRenderedPageCarriesNoDuplicateDOMID(t *testing.T) {
	data := webFragmentDataListTable(false)
	// Two finder columns each offering an add control is the shape a nested
	// list produces. buildColumns walks every prefix, and buildListColumn puts
	// a "+ new" item at the head of each list-level column.
	require.NotEmpty(t, data.Columns, "the fixture must carry a finder column")
	second := data.Columns[0]
	second.NamedItems = []ColumnItem{{Name: finderAddItemName, IsList: true, AddURL: "/config/add/bgp/peer/london/import/"}}
	data.Columns = append(data.Columns, second)

	renderer, err := NewRenderer()
	require.NoError(t, err)

	// The workbench shell wraps whatever a page rendered. A table page is the
	// one that draws an add control, so it is the composition under test.
	workbench := webWorkbenchData(false)
	workbench.Content = renderer.renderComponent("workbench_table", workbenchTable(webWorkbenchTable("rows")))
	require.Contains(t, string(workbench.Content), addEntryContentID,
		"the workbench page fixture must carry the add control, or this case checks nothing")

	for _, tc := range []struct {
		name string
		comp templ.Component
	}{
		{"full-content", fullContent(data)},
		{"oob-response", oobResponse(data)},
		{"workbench-page", pageWorkbench(workbench)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			require.NoError(t, tc.comp.Render(context.Background(), &buf))

			seen := make(map[string]bool)
			total := 0

			for _, match := range idAttrPattern.FindAllStringSubmatch(buf.String(), -1) {
				total++
				assert.False(t, seen[match[1]], "id %q appears more than once in one document", match[1])
				seen[match[1]] = true
			}

			assert.Positive(t, total, "no id was scanned; the document did not render")
		})
	}
}

// tablePattern, headerCellPattern, bodyRowPattern and dataCellPattern read one
// captured table apart. A capture is already-rendered markup. The regexps read
// a fixed string. They are not an HTML parser.
var (
	tablePattern      = regexp.MustCompile(`(?s)<table[^>]*>.*?</table>`)
	theadPattern      = regexp.MustCompile(`(?s)<thead[^>]*>(.*?)</thead>`)
	tbodyPattern      = regexp.MustCompile(`(?s)<tbody[^>]*>(.*?)</tbody>`)
	headerCellPattern = regexp.MustCompile(`<th[\s>]`)
	bodyRowPattern    = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	dataCellPattern   = regexp.MustCompile(`<td([^>]*)>`)
	colspanPattern    = regexp.MustCompile(`colspan="(\d+)"`)
)

// rowWidth is how many header columns one body row covers, counting colspan.
func rowWidth(row string) int {
	width := 0

	for _, cell := range dataCellPattern.FindAllStringSubmatch(row, -1) {
		span := 1
		if m := colspanPattern.FindStringSubmatch(cell[1]); m != nil {
			span, _ = strconv.Atoi(m[1])
		}
		width += span
	}

	return width
}

// TestCapturedTableRowsCoverTheirHeader verifies every captured table body row
// is as wide as its own header.
//
// VALIDATES: one td per th in every fixture that holds a table, colspan
// counted.
// PREVENTS: columns that stop lining up. Each table component ranges its
// columns for the header and the row's own cells for the body, so the two agree
// only while every producer keeps them equal. A row one cell short shifts every
// value after it under the wrong heading, which an operator reads as data.
func TestCapturedTableRowsCoverTheirHeader(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	scanned := 0

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading its own fixtures
		if readErr != nil {
			return readErr
		}

		for _, table := range tablePattern.FindAllString(string(body), -1) {
			head := theadPattern.FindStringSubmatch(table)
			if head == nil {
				continue // no header, so no width for a row to cover
			}

			// A tbody is optional markup. Where a component omits it, the rows
			// are direct children of the table, and dropping the header leaves
			// exactly those. Skipping such a table left it unchecked and said so
			// nowhere.
			rows := strings.Replace(table, head[0], "", 1)
			if body := tbodyPattern.FindStringSubmatch(table); body != nil {
				rows = body[1]
			}

			columns := len(headerCellPattern.FindAllString(head[1], -1))
			for _, row := range bodyRowPattern.FindAllStringSubmatch(rows, -1) {
				scanned++
				assert.Equal(t, columns, rowWidth(row[1]),
					"%s: a body row covers %d of %d header columns", path, rowWidth(row[1]), columns)
			}
		}

		return nil
	})
	require.NoError(t, err)
	assert.Positive(t, scanned, "no table row was scanned; the capture directory holds no table")
}

// TestGoldenFixturesCarryNoEmptyClassAttribute verifies no captured component
// writes class with nothing in it.
//
// VALIDATES: a class attribute names at least one class.
// PREVENTS: a row rendering class="" because its only class is a conditional
// modifier. The list table did that for every row with no pending change. Those
// rows carried no styling hook at all. The sibling workbench table names a base
// class and appends the modifier.
func TestGoldenFixturesCarryNoEmptyClassAttribute(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	scanned := 0

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading its own fixtures
		if readErr != nil {
			return readErr
		}

		scanned++

		assert.NotContains(t, string(body), `class=""`, "%s renders an empty class attribute", path)

		return nil
	})
	require.NoError(t, err)
	assert.Positive(t, scanned, "no fixture was scanned; the capture directory is missing")
}

// triggerAttrPattern finds one hx-trigger literal in template source. templ
// writes an expression attribute with double quotes only, so a literal value is
// always in this form.
var triggerAttrPattern = regexp.MustCompile(`hx-trigger="([^"]*)"`)

// TestNoTriggerFilterNeedsEval verifies no trigger in this package asks htmx to
// compile a bracketed filter.
//
// This test proves MARKUP. The behavior behind it was proven in a browser.
// Three keystrokes in an inline editor produced three POSTs of a partial value.
// Three keystrokes in the terminal ran three partial commands.
//
// VALIDATES: no hx-trigger carries a [ ] filter clause.
// PREVENTS: a trigger that fires on every event instead of the one it names.
// htmx builds the filter as source and runs it through Function() (nt,
// assets/htmx.min.js). setSecurityHeaders (auth.go) sends script-src 'self'
// with no unsafe-eval, so the call throws. htmx catches it, fires
// htmx:syntax:error and returns null, and the caller assigns only a truthy
// filter. gt then reads no filter and ignores no event, so keyup[key=='Enter']
// degrades to a bare keyup rather than failing closed.
//
// Enter is delivered by initEnterSubmit (assets/cli.js) instead. It reads the
// element's own hx-trigger and dispatches ze-enter, so the markup states the
// contract and the script compiles nothing.
func TestNoTriggerFilterNeedsEval(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join(".", "*.templ"))
	require.NoError(t, err)
	require.NotEmpty(t, sources, "the component set must not be empty")

	seen := 0
	enterUsers := 0

	for _, path := range sources {
		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading its own package
		require.NoError(t, readErr)

		for _, match := range triggerAttrPattern.FindAllStringSubmatch(string(body), -1) {
			seen++

			assert.NotContains(t, match[1], "[",
				"%s writes hx-trigger=%q, and the Content-Security-Policy stops htmx compiling that filter", path, match[1])

			if slices.Contains(strings.Fields(strings.ReplaceAll(match[1], ",", " ")), "ze-enter") {
				enterUsers++
			}
		}
	}

	assert.Positive(t, seen, "no trigger was scanned; the pattern has stopped matching")

	script, err := os.ReadFile(filepath.Join("assets", "cli.js"))
	require.NoError(t, err)

	dispatch := jsBlock(string(script), "function initEnterSubmit()")
	require.NotEmpty(t, dispatch, "assets/cli.js defines no initEnterSubmit, so ze-enter reaches nothing")
	assert.Contains(t, dispatch, "'ze-enter'", "initEnterSubmit dispatches an event no trigger names")
	assert.Contains(t, string(script), "initEnterSubmit();", "initEnterSubmit is never called on page load")
	assert.Positive(t, enterUsers, "no component asks for ze-enter, so the listener drives nothing")
}

// focusableTagPattern finds one start tag a browser can put the caret in.
// Every one of the four takes focus, and each is a control an operator types or
// clicks in.
var focusableTagPattern = regexp.MustCompile(`<(?:input|select|textarea|button)\b[^>]*>`)

// selfSwapTargetPattern reads the hx-target of a control that replaces itself.
// "closest <selector>" names an ancestor and "this" names the element, so both
// swaps take the control out of the document with the target.
var selfSwapTargetPattern = regexp.MustCompile(`hx-target="(closest [^"]+|this)"`)

// cssUnsafeIDPattern finds a character that ends an id inside a CSS selector.
// The set is what a config path can carry: isYANGIdentChar (handler.go) admits
// a dot and a colon, and joining the path adds the slash.
var cssUnsafeIDPattern = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// TestSelfReplacingControlsCarryAStableID verifies every captured control that
// swaps itself away names an id, and that the id is one a CSS selector reads
// whole.
//
// This test proves MARKUP. The behavior behind it was proven in a browser. The
// caret survives a swap of the workbench field, of a list-table cell, and of a
// field whose list key carries a dot.
//
// VALIDATES: an id attribute on each control inside its own swap target.
// PREVENTS: the caret and the focus leaving the field an operator is typing
// in. htmx keeps the focused element, its selectionStart and its selectionEnd
// across a swap. It then re-finds the element by id and restores all three
// (assets/htmx.min.js, the swap function $e). With no id the lookup is
// getElementById(""), which answers null, and nothing is restored. Every
// inline editor here replaces itself with the response, so an operator lost the
// caret on every field of the config editor.
//
// The id must also survive a CSS selector. The vendored htmx is 2.0.4 and it
// calls CSS.escape nowhere. The settle matcher (Le) builds an attribute
// selector and the out-of-band lookup (He) builds "#" + id. Upstream 2.0.10
// escapes both. A VLAN interface is keyed eth0.100, so a raw path would put a
// dot in an id here.
func TestSelfReplacingControlsCarryAStableID(t *testing.T) {
	root := filepath.Join("testdata", "golden")

	checked := 0

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading its own fixtures
		if readErr != nil {
			return readErr
		}

		for _, tag := range focusableTagPattern.FindAllString(string(body), -1) {
			if !selfSwapTargetPattern.MatchString(tag) {
				continue
			}

			checked++

			id := idAttrPattern.FindStringSubmatch(tag)
			if !assert.NotNil(t, id, "%s: a control that swaps itself away carries no id: %s", path, tag) {
				continue
			}

			assert.NotEmpty(t, id[1], "%s: a control that swaps itself away carries an empty id: %s", path, tag)
			assert.NotRegexp(t, cssUnsafeIDPattern, id[1],
				"%s: the id %q ends early in a CSS selector, and htmx 2.0.4 escapes none", path, id[1])
		}

		return nil
	})
	require.NoError(t, err)
	assert.Positive(t, checked, "no self-replacing control was scanned; the capture directory is missing")
}

// jsBlock returns the source of one brace-delimited JavaScript block, starting
// at header. It answers an empty string when the header is absent, so a caller
// that asserts on the result fails rather than passing over nothing.
func jsBlock(source, header string) string {
	start := strings.Index(source, header)
	if start < 0 {
		return ""
	}

	return balancedBlockAt(source, start)
}

// balancedBlockAt returns the brace-delimited block that opens at or after
// start, and an empty string when no block closes.
//
// It skips a quoted string and a comment, because a brace inside either one is
// text rather than structure. Counting raw bytes ended the block at the first
// closing brace inside a string. Every assertion on the truncated result then
// passed over source it never read. Two callers read it: JavaScript and YANG.
// Both use the same quote forms and the same comment forms.
func balancedBlockAt(source string, start int) string {
	depth := 0

	for i := start; i < len(source); i++ {
		switch {
		case source[i] == '\'' || source[i] == '"' || source[i] == '`':
			i = skipQuoted(source, i)
		case source[i] == '/' && i+1 < len(source) && (source[i+1] == '/' || source[i+1] == '*'):
			i = skipComment(source, i)
		case source[i] == '/' && opensRegex(source, i):
			i = skipRegex(source, i)
		case source[i] == '{':
			depth++
		case source[i] == '}':
			depth--
			if depth == 0 {
				return source[start : i+1]
			}
		}
	}

	return ""
}

// skipQuoted answers the index of the closing quote of the literal that opens
// at i, or the last index when the literal never closes.
func skipQuoted(source string, i int) int {
	quote := source[i]

	for j := i + 1; j < len(source); j++ {
		if source[j] == '\\' {
			j++
			continue
		}
		if source[j] == quote {
			return j
		}
	}

	return len(source) - 1
}

// skipComment answers the index of the last byte of the comment that opens at
// i, or i itself when the slash opens no comment.
func skipComment(source string, i int) int {
	if i+1 >= len(source) {
		return i
	}

	switch source[i+1] {
	case '/':
		if end := strings.IndexByte(source[i:], '\n'); end >= 0 {
			return i + end
		}
	case '*':
		if end := strings.Index(source[i+2:], "*/"); end >= 0 {
			return i + 2 + end + 1
		}
	default:
		return i
	}

	return len(source) - 1
}

// withoutQuoted answers source with every quoted literal removed, so a caller
// reads the statements a block makes and never the prose it carries. A YANG
// description that names an extension would otherwise satisfy a check for the
// extension itself.
func withoutQuoted(source string) string {
	var out strings.Builder

	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\'', '"', '`':
			i = skipQuoted(source, i)
		default:
			out.WriteByte(source[i])
		}
	}

	return out.String()
}

// regexPrefixBytes is every byte a JavaScript regular expression CAN follow.
// A slash after a value is division. A slash after an operator or an opening
// bracket starts a pattern.
const regexPrefixBytes = "(,=:[!&|?{};+-*%~^<>"

// opensRegex reports whether the slash at i starts a regular expression.
func opensRegex(source string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch source[j] {
		case ' ', '\t', '\n', '\r':
			continue
		}

		return strings.IndexByte(regexPrefixBytes, source[j]) >= 0
	}

	return true
}

// skipRegex answers the index of the closing slash of the pattern that opens at
// i, or the last index when the pattern never closes. A slash inside a character
// class needs no escape, so the class is tracked.
func skipRegex(source string, i int) int {
	class := false

	for j := i + 1; j < len(source); j++ {
		switch source[j] {
		case '\\':
			j++
		case '[':
			class = true
		case ']':
			class = false
		case '\n':
			return i
		case '/':
			if !class {
				return j
			}
		}
	}

	return len(source) - 1
}

// TestBalancedBlockReadsPastTextAndComments verifies the block reader ends a
// block on structure, never on a brace that is text.
//
// VALIDATES: a brace inside a string, a line comment, a block comment or a
// regular expression does not close the block, and an unterminated block reads
// as absent.
// PREVENTS: an assertion that passes over source it never read. The reader
// counted raw bytes. One closing brace inside a string literal truncated the
// block. Every Contains check on the remainder then went quiet, not red.
func TestBalancedBlockReadsPastTextAndComments(t *testing.T) {
	for name, tc := range map[string]struct{ source, want string }{
		"plain":         {"f() {\n  a();\n}\nafter", "f() {\n  a();\n}"},
		"string":        {"f() {\n  var s = '}';\n  a();\n}\nafter", "f() {\n  var s = '}';\n  a();\n}"},
		"template":      {"f() {\n  var s = `${x}}`;\n  a();\n}\nafter", "f() {\n  var s = `${x}}`;\n  a();\n}"},
		"escaped quote": {"f() {\n  var s = '\\'}';\n  a();\n}\nafter", "f() {\n  var s = '\\'}';\n  a();\n}"},
		"line comment":  {"f() {\n  // }\n  a();\n}\nafter", "f() {\n  // }\n  a();\n}"},
		"block comment": {"f() {\n  /* } */\n  a();\n}\nafter", "f() {\n  /* } */\n  a();\n}"},
		"regex":         {"f() {\n  s.replace(/[}]/, '');\n  a();\n}\nafter", "f() {\n  s.replace(/[}]/, '');\n  a();\n}"},
		"regex slashes": {"f() {\n  s.replace(/^\\/x\\//, '');\n  a();\n}\nafter", "f() {\n  s.replace(/^\\/x\\//, '');\n  a();\n}"},
		"division":      {"f() {\n  var n = a / b;\n  c();\n}\nafter", "f() {\n  var n = a / b;\n  c();\n}"},
		"unterminated":  {"f() {\n  a();\n", ""},
		"yang leaf":     {"leaf p {\n  description \"a } brace\";\n  ze:sensitive;\n}\nafter", "leaf p {\n  description \"a } brace\";\n  ze:sensitive;\n}"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, balancedBlockAt(tc.source, 0))
		})
	}

	assert.Empty(t, jsBlock("function other() {}", "function initErrorPanel()"),
		"a header that is absent must read as absent, so its caller fails")
}

// TestMarkupHasClassReadsOneClassInsideTheAttribute verifies the class check
// reads a class, not a whole attribute.
//
// VALIDATES: a class beside its siblings matches, and a longer name that merely
// contains it does not.
// PREVENTS: a guard that goes red on correct markup. Matching class="<name>"
// verbatim required the element to carry exactly one class, so adding a modifier
// beside the base class failed a selector the browser still matches. A sibling
// test is pushing this package's markup toward base-plus-modifier.
func TestMarkupHasClassReadsOneClassInsideTheAttribute(t *testing.T) {
	assert.True(t, markupHasClass(`<li class="error-item">x</li>`, "error-item"))
	assert.True(t, markupHasClass(`<li class="error-item error-item--new">x</li>`, "error-item"))
	assert.True(t, markupHasClass(`<li class="row error-item">x</li>`, "error-item"))
	assert.False(t, markupHasClass(`<li class="error-items">x</li>`, "error-item"))
	assert.False(t, markupHasClass(`<li data-x="error-item">x</li>`, "error-item"))
}

// jsGetElementByID and jsQuerySelectorAll read the ids and classes one block of
// JavaScript looks up. A selector is a contract with the markup, and a contract
// with nothing on the other side is what these tests exist to find.
var (
	jsGetElementByID   = regexp.MustCompile(`getElementById\('([^']+)'\)`)
	jsQuerySelectorAll = regexp.MustCompile(`querySelectorAll\('\.([^']+)'\)`)
	markupDataAction   = regexp.MustCompile(`data-action="([^"]+)"`)
)

// TestErrorDrawerWiringHoldsTogether verifies the error drawer's markup and the
// script that drives it name the same ids, classes, actions and events.
//
// VALIDATES: initErrorPanel is registered on page load, every element the
// script looks up is emitted by a component, every data-action the markup
// carries has a branch, and the event dismiss-error fires has a listener.
// PREVENTS: a drawer nothing can open. #error-toggle carried no data-action, so
// initActions never saw it, and the Content-Security-Policy forbids an inline
// handler. cli.js also dispatched ze-error-update with no listener anywhere.
// The first version of this test asserted four literals. It would have passed
// with the toggle branch deleted, with initErrorPanel never called, and with
// syncErrorPanel reading a class errorItem does not write.
//
// No browser runs here, and none is available to run. The .wb harness drives
// HTTP and executes no script. agent-browser is an interactive tool with no
// hook in `./le verify current mode full`. What a browser adds over this is
// layout and event order. It would also catch a selector matching the wrong
// element, which no component here can produce.
func TestErrorDrawerWiringHoldsTogether(t *testing.T) {
	var panel, item, oob strings.Builder
	require.NoError(t, errorPanel().Render(context.Background(), &panel))
	require.NoError(t, errorItem(ErrorData{ID: "7", Path: "bgp/as", Message: "bad"}).Render(context.Background(), &item))
	require.NoError(t, oobError(ErrorData{ID: "7", Path: "bgp/as", Message: "bad"}).Render(context.Background(), &oob))

	markup := panel.String() + item.String() + oob.String()

	script, err := os.ReadFile(filepath.Join("assets", "cli.js"))
	require.NoError(t, err)

	source := string(script)

	// The init function is registered, so the listeners below are ever bound.
	onLoad := jsBlock(source, "document.addEventListener('DOMContentLoaded'")
	require.NotEmpty(t, onLoad, "cli.js must register a DOMContentLoaded handler")
	assert.Contains(t, onLoad, "initErrorPanel();",
		"initErrorPanel must run on page load, or nothing binds the drawer's listeners")

	// The listeners the drawer needs, and the event its own button fires.
	initPanel := jsBlock(source, "function initErrorPanel()")
	require.NotEmpty(t, initPanel, "cli.js must define initErrorPanel")
	assert.Contains(t, initPanel, `addEventListener('ze-error-update'`,
		"the event dismiss-error dispatches must have a listener")
	assert.Contains(t, initPanel, `addEventListener('htmx:after:settle'`,
		"an incoming error must open the drawer")

	actions := jsBlock(source, "function initActions()")
	require.NotEmpty(t, actions, "cli.js must define initActions")
	assert.Contains(t, actions, `dispatchEvent(new Event('ze-error-update'))`,
		"dismissing an error must fire the event initErrorPanel listens for")

	// Every action the markup carries reaches a branch.
	for _, match := range markupDataAction.FindAllStringSubmatch(markup, -1) {
		assert.Contains(t, actions, `action === '`+match[1]+`'`,
			"the markup carries data-action=%q and initActions has no branch for it", match[1])
	}

	// Every element and class the drawer's script looks up is markup a
	// component writes.
	sync := jsBlock(source, "function syncErrorPanel()")
	require.NotEmpty(t, sync, "cli.js must define syncErrorPanel")

	lookups := 0

	for _, block := range []string{sync, initPanel, actions} {
		for _, match := range jsGetElementByID.FindAllStringSubmatch(block, -1) {
			if !strings.HasPrefix(match[1], "error") {
				continue // another feature's element, checked by its own test
			}
			lookups++
			assert.Contains(t, markup, `id="`+match[1]+`"`,
				"cli.js reads #%s and no error component emits it", match[1])
		}

		for _, match := range jsQuerySelectorAll.FindAllStringSubmatch(block, -1) {
			if !strings.HasPrefix(match[1], "error") {
				continue // another feature's class, checked by its own test
			}
			lookups++
			assert.True(t, markupHasClass(markup, match[1]),
				"cli.js counts .%s and no error component emits it", match[1])
		}
	}

	assert.Positive(t, lookups, "no selector was checked; the patterns have stopped matching")
}

// webPackagePrefix is how the generator names a template of this package. The
// key is the source file's path from the repository root, because that is what
// the generator walks and what a reader greps for.
const webPackagePrefix = "internal/component/web/"

// htmxAttributePattern finds one htmx attribute in rendered markup: a
// whitespace byte, the attribute name, then the equals sign a value follows.
// Requiring both ends keeps a file name such as sse-client.js out of the match.
// htmx 4 names an extension's attributes with a colon, as hx-sse:connect does.
var htmxAttributePattern = regexp.MustCompile(`\s(hx-[a-z:-]+)=`)

// scriptSrcPattern finds one served asset in a page's head.
var scriptSrcPattern = regexp.MustCompile(`<script src="/assets/([^"]+)"`)

// htmxAssetFor names the asset that implements one rendered attribute, or "".
//
// The core attributes are htmx itself. The SSE attributes come from a separate
// extension file, which is the reason the import set is per page rather than
// one list for the whole UI.
func htmxAssetFor(attribute string) string {
	switch {
	case strings.HasPrefix(attribute, "hx-sse:"):
		return "hx-sse.min.js"
	case strings.HasPrefix(attribute, "hx-"):
		return "htmx.min.js"
	default:
		return ""
	}
}

// matchedGroups returns the first capture of every match, sorted and without
// repeats.
func matchedGroups(pattern *regexp.Regexp, body []byte) []string {
	var found []string

	for _, match := range pattern.FindAllSubmatch(body, -1) {
		found = append(found, string(match[1]))
	}

	slices.Sort(found)

	return slices.Compact(found)
}

// derivedPageAssets returns the per-page asset sets the generator derives,
// keyed by the page template's path from the repository root.
func derivedPageAssets(t *testing.T) map[string][]string {
	t.Helper()

	root, err := filepath.Abs("../../..")
	require.NoError(t, err, "resolve the repository root")

	sets, err := webassets.Pages(root)
	require.NoError(t, err, "derive the per-page asset sets")
	return map[string][]string(sets)
}

// vendorHTMXDir holds the served library, from this package's directory. It is
// the source of truth every consumer copy is synced from
// (third_party/web/MANIFEST.md).
const vendorHTMXDir = "../../../third_party/web/htmx"

// htmx2CorePattern finds htmx 2's version literal in the minified core. htmx 2
// writes `version:"2.0.4"`; htmx 4 writes the bare string `"4.0.0-beta6"`.
var htmx2CorePattern = regexp.MustCompile(`version:"2\.`)

// htmx4CorePattern finds htmx 4's version literal in the minified core.
var htmx4CorePattern = regexp.MustCompile(`"4\.\d+\.\d+`)

// htmxCoreAsset is the file every page loads the library from. The name did
// not change at the cutover; the bytes behind it did.
const htmxCoreAsset = "htmx.min.js"

// htmx2SSEExtension is the file name htmx 2 published its SSE extension under.
// htmx 4 holds its extensions in the core package and names this one
// hx-sse.min.js, so the name alone settles which library a page would load.
const htmx2SSEExtension = "sse.js"

// VALIDATES: AC-1 -- every page of web, lg and chaos loads the htmx 4 core, and
// no page can reach htmx 2's core or its separately published SSE extension.
// PREVENTS: a page that serves the old library while the rest of the cutover
// assumes the new one. The served name survives the version change
// (`htmx.min.js` before the cutover and after it), so this reads the BYTES:
// a name test would pass over htmx 2 wearing htmx 4's file name.
func TestPagesServeHtmx4(t *testing.T) {
	derived := derivedPageAssets(t)
	require.NotEmpty(t, derived,
		"the generator derived no page at all, so this check would prove nothing")

	var streaming, plain int

	for page, assets := range derived {
		if len(assets) == 0 {
			continue
		}

		carriesSSE := false

		t.Run(page, func(t *testing.T) {
			// An extension calls htmx.registerExtension when it runs, so it
			// throws when the core has not run first. The order the generator
			// emits is the order the head block renders.
			assert.Equalf(t, htmxCoreAsset, assets[0],
				"%s loads %s before the core, and an extension needs the core loaded", page, assets[0])

			for _, asset := range assets {
				assert.NotEqualf(t, htmx2SSEExtension, asset,
					"%s loads %s, which is htmx 2's SSE extension; htmx 4 publishes hx-sse",
					page, asset)

				body, err := os.ReadFile(filepath.Join(vendorHTMXDir, asset))
				require.NoErrorf(t, err,
					"%s loads %s and third_party/web/htmx holds no such file", page, asset)

				if strings.Contains(asset, "sse") {
					carriesSSE = true

					continue // the extension carries no version literal
				}

				assert.Falsef(t, htmx2CorePattern.Match(body),
					"%s loads %s, whose bytes are htmx 2", page, asset)
				assert.Truef(t, htmx4CorePattern.Match(body),
					"%s loads %s, whose bytes carry no htmx 4 version", page, asset)
			}
		})

		if carriesSSE {
			streaming++
		} else {
			plain++
		}
	}

	// AC-1's second half: the SSE extension is per page, not global. A run where
	// every page streamed, or none did, would pass every assertion above while
	// proving nothing about which pages load the extension.
	assert.Positive(t, streaming, "no page loads an SSE extension, so no page streams")
	assert.Positive(t, plain, "every page loads an SSE extension, so the set is not per page")
}

// webPageFixture is one captured page and the template that rendered it.
type webPageFixture struct {
	// Template is the templ source, from the repository root.
	Template string
	// Path is the fixture on disk.
	Path string
	// Body is the rendered markup.
	Body []byte
}

// webPageFixtures returns every captured fixture that renders a whole page.
//
// A page is a fixture carrying a head, because the head is where a page states
// what it loads. Every other fixture is a fragment served into a page that has
// already loaded its assets.
func webPageFixtures(t *testing.T) []webPageFixture {
	t.Helper()

	root := filepath.Join("testdata", "golden")

	files := make([]string, 0, len(webTemplGoldenSpec))
	for file := range webTemplGoldenSpec {
		files = append(files, file)
	}

	slices.Sort(files)

	var pages []webPageFixture

	for _, file := range files {
		for _, unit := range webTemplGoldenSpec[file] {
			for _, variant := range unit.Variants {
				path := webTemplGolden.FixturePath(root, file, unit.FixtureName(variant))

				body, err := os.ReadFile(path)
				require.NoErrorf(t, err, "read the captured fixture %s", path)

				if !bytes.Contains(body, []byte("<head>")) {
					continue
				}

				pages = append(pages, webPageFixture{
					Template: webPackagePrefix + file,
					Path:     path,
					Body:     body,
				})
			}
		}
	}

	require.NotEmptyf(t, pages, "no captured fixture renders a head, so this check would pass over nothing")

	return pages
}

// zeSurfaceDirs are the three packages that serve a page of their own, from the
// repository root. internal/le/webassets/webassets.go names the same three. The
// repetition is deliberate: a checker that read its population from the
// generator could not report a surface the generator forgot.
var zeSurfaceDirs = []string{
	"internal/component/web",
	"internal/component/lg",
	"internal/chaos/web",
}

// htmxEventLiteral finds one event name in the vendored library. A name that
// ends in a colon is a PREFIX the library completes at run time, as
// "htmx:process:" plus an extension's name is, and the trailing colon says so.
var htmxEventLiteral = regexp.MustCompile(`htmx:[a-z][a-zA-Z:]*`)

// zeHtmxEventReference finds one event name Ze's own source names in a string
// literal. A comment naming an event is prose; a quoted name is what
// addEventListener is given.
var zeHtmxEventReference = regexp.MustCompile(`['"](htmx:[a-zA-Z:]+)['"]`)

// htmxListenerFloor is the least number of quoted event names Ze's sources
// hold. They hold 14 on 2026-08-15, over the web assets, the looking-glass
// theme script and the chaos dashboard's inline script. A walk that finds
// fewer has stopped reading them, and every assertion over it would pass.
const htmxListenerFloor = 10

// dispatchedHtmxEvents reads every event the vendored library dispatches: the
// literal names, and the prefixes it completes at run time.
func dispatchedHtmxEvents(t *testing.T) (map[string]bool, []string) {
	t.Helper()

	entries, err := os.ReadDir(vendorHTMXDir)
	require.NoError(t, err, "read the vendored htmx directory")

	var prefixes []string

	names := map[string]bool{}

	for _, entry := range entries {
		body, readErr := os.ReadFile(filepath.Join(vendorHTMXDir, entry.Name()))
		require.NoErrorf(t, readErr, "read the vendored %s", entry.Name())

		for _, name := range htmxEventLiteral.FindAllString(string(body), -1) {
			if strings.HasSuffix(name, ":") {
				prefixes = append(prefixes, name)

				continue
			}

			names[name] = true
		}
	}

	require.NotEmpty(t, names,
		"the vendored library names no event at all, so every listener would read as valid")

	return names, prefixes
}

// zeSurfaceSources returns every source of the three surfaces that can hold a
// listener, keyed by its path from the repository root. The vendored library is
// skipped: it dispatches the events rather than listening for them. So is
// testdata, which holds captured output rather than source.
func zeSurfaceSources(t *testing.T) map[string]string {
	t.Helper()

	root, err := filepath.Abs("../../..")
	require.NoError(t, err, "resolve the repository root")

	vendored := map[string]bool{}

	err = filepath.WalkDir(filepath.Join(root, "third_party", "web"), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if !d.IsDir() {
			vendored[d.Name()] = true
		}

		return nil
	})
	require.NoError(t, err, "walk third_party/web for the vendored file names")

	sources := map[string]string{}

	for _, dir := range zeSurfaceDirs {
		err = filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			name := d.Name()

			switch {
			case d.IsDir():
				if name == "testdata" {
					return filepath.SkipDir
				}

				return nil
			case vendored[name], strings.HasSuffix(name, "_test.go"), strings.HasSuffix(name, "_templ.go"):
				return nil
			case filepath.Ext(name) != ".js" && filepath.Ext(name) != ".go":
				return nil
			}

			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}

			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}

			sources[filepath.ToSlash(relative)] = string(body)

			return nil
		})
		require.NoErrorf(t, err, "walk %s for the sources that can hold a listener", dir)
	}

	return sources
}

// VALIDATES: AC-9 -- every htmx event Ze's own sources name is one the served
// library dispatches, so no listener waits for an htmx 2 name.
// PREVENTS: the silent half of the cutover. A listener on a name htmx 4 no
// longer fires is never called: no console error, no failing fixture, and the
// feature it drives simply stops. The library is read rather than a list of
// htmx 2 names kept here, so a typo in an htmx 4 name fails too.
func TestNoListenerUsesAHtmx2EventName(t *testing.T) {
	dispatched, prefixes := dispatchedHtmxEvents(t)

	referenced := 0

	for path, body := range zeSurfaceSources(t) {
		for _, match := range zeHtmxEventReference.FindAllStringSubmatch(body, -1) {
			name := match[1]
			referenced++

			if dispatched[name] {
				continue
			}

			if slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(name, prefix) }) {
				continue
			}

			assert.Failf(t, "a listener names an event htmx 4 never fires",
				"%s names %s, and the vendored library dispatches no such event", path, name)
		}
	}

	assert.GreaterOrEqualf(t, referenced, htmxListenerFloor,
		"the walk found %d quoted event names, want at least %d; it has stopped reading the sources",
		referenced, htmxListenerFloor)
}

// zeSurfaceFloor is the least number of surfaces the derived sets cover. Three
// serve a page on 2026-08-15: web, the looking glass and chaos.
const zeSurfaceFloor = 3

// VALIDATES: AC-11 -- every asset a derived page set names is a file the
// package serving that page holds, over every surface the generator derives.
// PREVENTS: a page whose script tag 404s. The page renders, the server reports
// 200, and only the browser sees that nothing works. The per-package checks
// resolve against the SERVED filesystem and are stronger for the package they
// cover (markupcheck.AssertAssetsResolve for web and lg, TestEmbeddedAssets for
// chaos). None of them covers a surface that has no such check, which is what
// this one reads the whole derived population for.
func TestEveryPageAssetResolves(t *testing.T) {
	derived := derivedPageAssets(t)
	require.NotEmpty(t, derived, "the generator derived no page at all, so this check would prove nothing")

	root, err := filepath.Abs("../../..")
	require.NoError(t, err, "resolve the repository root")

	surfaces := map[string]bool{}
	resolved := 0

	for page, assets := range derived {
		source, _, _ := strings.Cut(page, "#")
		dir := filepath.Dir(source)
		surfaces[dir] = true

		for _, asset := range assets {
			resolved++

			_, statErr := os.Stat(filepath.Join(root, dir, "assets", asset))
			assert.NoErrorf(t, statErr,
				"%s loads %s and %s/assets holds no such file", page, asset, dir)
		}
	}

	assert.GreaterOrEqualf(t, len(surfaces), zeSurfaceFloor,
		"the derived sets cover %d surfaces, want at least %d", len(surfaces), zeSurfaceFloor)
	assert.Positive(t, resolved, "no page names an asset, so no path was resolved")
}

// VALIDATES: every htmx attribute a page renders has its asset in the set the
// generator derived for that page, and in the set that page's head loads.
// PREVENTS: a page that renders correctly and does nothing in the browser. The
// generator derives the import set from source and over-approximates; this
// reads the rendered bytes and reports an attribute the set does not cover.
func TestPageImportsCoverRenderedAttributes(t *testing.T) {
	derived := derivedPageAssets(t)

	for _, page := range webPageFixtures(t) {
		t.Run(filepath.Base(page.Path), func(t *testing.T) {
			assets, ok := derived[page.Template]
			require.Truef(t, ok,
				"the generator derived no asset set for %s; it must name every template that renders a head",
				page.Template)

			loaded := matchedGroups(scriptSrcPattern, page.Body)

			for _, attribute := range matchedGroups(htmxAttributePattern, page.Body) {
				want := htmxAssetFor(attribute)
				if want == "" {
					continue
				}

				assert.Containsf(t, assets, want,
					"%s renders %s, which %s implements, but the generator derived %v for it",
					page.Path, attribute, want, assets)
				assert.Containsf(t, loaded, want,
					"%s renders %s, which %s implements, but its head loads %v",
					page.Path, attribute, want, loaded)
			}
		})
	}
}
