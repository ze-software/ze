package web

import (
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
// hook in make ze-verify. What a browser adds over this is layout and event
// order. It would also catch a selector that matches the wrong element, which
// no component here can produce.
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
	assert.Contains(t, initPanel, `addEventListener('htmx:oobAfterSwap'`,
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
