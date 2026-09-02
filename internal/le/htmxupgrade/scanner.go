// Design: docs/architecture/core-design.md -- the htmx upgrade scanner in Go
// Detail: rules.go -- the ordered migration rules this scanner applies.
// Detail: tree.go -- package discovery, explanation parsing, and gate verdicts.
//
// The upstream scanner has two halves. Markup builds a lenient tree so an
// attribute on an ancestor can be related to a request on a descendant. Raw
// text is then scanned for JavaScript, event, configuration, and header names.
// This file keeps that split so a text-only rewrite cannot silently replace the
// inheritance proof.

package htmxupgrade

import (
	"bytes"
	"errors"
	stdhtml "html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Issue is one scanner finding and one row of either action's answer.
type Issue struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

type attribute struct {
	name  string
	value string
}

type node struct {
	tag      string
	attrs    []attribute
	children []*node
	line     int
}

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

var (
	djangoBlock = regexp.MustCompile(`(?s:\{%.*?%\})`)
	djangoValue = regexp.MustCompile(`(?s:\{\{.*?\}\})`)
	phpBlock    = regexp.MustCompile(`(?s:<\?(?:php)?.*?\?>)`)
	erbBlock    = regexp.MustCompile(`(?s:<%.*?%>)`)
	csrfValue   = regexp.MustCompile(`(?i:csrf|xsrf)`)
	lineBreaks  = strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"\v", "\n",
		"\f", "\n",
		"\x1c", "\n",
		"\x1d", "\n",
		"\x1e", "\n",
		"\u0085", "\n",
		"\u2028", "\n",
		"\u2029", "\n",
	)
	allEventRenames  = slices.Concat(eventRenames, sseEventRenames, wsEventRenames)
	oldEventsByLower = lowerRenameIndex(allEventRenames)
	removedByLower   = lowerStringIndex(removedEvents)
)

func lowerRenameIndex(rules []renameRule) map[string]renameRule {
	found := make(map[string]renameRule, len(rules))
	for _, rule := range rules {
		found[strings.ToLower(rule.old)] = rule
	}
	return found
}

func lowerStringIndex(values []string) map[string]string {
	found := make(map[string]string, len(values))
	for _, value := range values {
		found[strings.ToLower(value)] = value
	}
	return found
}

func checkFile(path string) ([]Issue, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a path enumerated beneath the selected checkout
	if err != nil {
		return nil, err
	}
	text := strings.ToValidUTF8(string(raw), "\uFFFD")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	raw = []byte(strings.ReplaceAll(text, "\r", "\n"))

	issues := make([]Issue, 0)
	if !jsExtensions[strings.ToLower(filepath.Ext(path))] {
		root := parseMarkup(stripTemplateSyntax(raw))
		checkNodes(root, path, &issues)
		checkInheritance(root, path, &issues)
	}
	checkText(raw, path, &issues)

	return normalizeIssues(issues), nil
}

func normalizeIssues(issues []Issue) []Issue {
	seen := make(map[issueKey]bool, len(issues))
	unique := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		key := issueKey{line: issue.Line, category: issue.Category, message: issue.Message}
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, issue)
	}
	slices.SortStableFunc(unique, func(a, b Issue) int { return a.Line - b.Line })
	return unique
}

type issueKey struct {
	category string
	message  string
	line     int
}

func stripTemplateSyntax(raw []byte) []byte {
	cleaned := djangoBlock.ReplaceAllFunc(raw, keepNewlines)
	cleaned = djangoValue.ReplaceAllFunc(cleaned, func(match []byte) []byte {
		out := make([]byte, 1+bytes.Count(match, []byte{'\n'}))
		out[0] = 'X'
		for i := 1; i < len(out); i++ {
			out[i] = '\n'
		}
		return out
	})
	cleaned = phpBlock.ReplaceAllFunc(cleaned, keepNewlines)
	return erbBlock.ReplaceAllFunc(cleaned, keepNewlines)
}

func keepNewlines(match []byte) []byte {
	return bytes.Repeat([]byte{'\n'}, bytes.Count(match, []byte{'\n'}))
}

// parseMarkup consumes at most one token for each part of the input. The stack
// depth is bounded by the number of start tags. Traversal below is iterative so
// hostile nesting cannot consume the Go call stack.
func parseMarkup(raw []byte) *node {
	root := &node{tag: "root"}
	stack := []*node{root}
	line := 1
	tokens := xhtml.NewTokenizer(bytes.NewReader(raw))

	for {
		kind := tokens.Next()
		tokenRaw := tokens.Raw()
		startLine := line
		line += bytes.Count(tokenRaw, []byte{'\n'})

		switch kind {
		case xhtml.ErrorToken:
			if errors.Is(tokens.Err(), io.EOF) {
				return root
			}
			return root
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			token := tokens.Token()
			child := &node{
				tag:   strings.ToLower(token.Data),
				attrs: parseAttributes(tokenRaw),
				line:  startLine,
			}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, child)
			if kind == xhtml.StartTagToken && !voidElements[child.tag] {
				stack = append(stack, child)
			}
		case xhtml.EndTagToken:
			tag := strings.ToLower(tokens.Token().Data)
			for i := len(stack) - 1; i > 0; i-- {
				if stack[i].tag == tag {
					stack = stack[:i]
					break
				}
			}
		case xhtml.TextToken, xhtml.CommentToken, xhtml.DoctypeToken:
			// These tokens cannot change the element tree.
		}
	}
}

func parseAttributes(raw []byte) []attribute {
	index := 1
	if index < len(raw) && raw[index] == '/' {
		index++
	}
	for index < len(raw) && !htmlSpace(raw[index]) && raw[index] != '>' {
		index++
	}

	holder := node{}
	for index < len(raw) {
		for index < len(raw) && htmlSpace(raw[index]) {
			index++
		}
		if index >= len(raw) || raw[index] == '>' {
			break
		}
		if raw[index] == '/' && index+1 < len(raw) && raw[index+1] == '>' {
			break
		}

		start := index
		for index < len(raw) && !htmlSpace(raw[index]) &&
			raw[index] != '=' && raw[index] != '>' {
			if raw[index] == '/' && index+1 < len(raw) && raw[index+1] == '>' {
				break
			}
			index++
		}
		name := strings.ToLower(string(raw[start:index]))
		for index < len(raw) && htmlSpace(raw[index]) {
			index++
		}

		value := ""
		if index < len(raw) && raw[index] == '=' {
			index++
			for index < len(raw) && htmlSpace(raw[index]) {
				index++
			}
			value, index = parseAttributeValue(raw, index)
		}
		if name != "" {
			holder.setAttr(name, stdhtml.UnescapeString(value))
		}
	}
	return holder.attrs
}

func parseAttributeValue(raw []byte, index int) (string, int) {
	if index >= len(raw) {
		return "", index
	}
	if raw[index] == '\'' || raw[index] == '"' {
		quote := raw[index]
		index++
		start := index
		for index < len(raw) && raw[index] != quote {
			index++
		}
		value := string(raw[start:index])
		if index < len(raw) {
			index++
		}
		return value, index
	}

	start := index
	for index < len(raw) && !htmlSpace(raw[index]) && raw[index] != '>' {
		index++
	}
	return string(raw[start:index]), index
}

func htmlSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\f' || value == '\r'
}

func scannerSpace(value rune) bool {
	return unicode.IsSpace(value) || value >= '\x1c' && value <= '\x1f'
}

func (n *node) setAttr(name, value string) {
	for i := range n.attrs {
		if n.attrs[i].name == name {
			n.attrs[i].value = value
			return
		}
	}
	n.attrs = append(n.attrs, attribute{name: name, value: value})
}

func checkNodes(root *node, path string, issues *[]Issue) {
	stack := []*node{root}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		checkNode(current, path, issues)
		for _, child := range slices.Backward(current.children) {
			stack = append(stack, child)
		}
	}
}

func checkNode(current *node, path string, issues *[]Issue) {
	var tb textbuf.Buffer
	for _, attr := range current.attrs {
		if rule, ok := findRename(removedAttrs, attr.name); ok {
			addIssue(issues, path, current.line, "removed-attr",
				tb.Reset().Str(attr.name).Str(" is removed → ").Str(rule.new).String())
		}
		if rule, ok := findRename(renamedAttrs, attr.name); ok {
			addIssue(issues, path, current.line, "renamed-attr",
				tb.Reset().Str(attr.name).Str(" → ").Str(rule.new).String())
		}
		if rule, ok := findRename(extensionAttrRenames, attr.name); ok {
			addIssue(issues, path, current.line, "ext",
				tb.Reset().Str(attr.name).Str(" → ").Str(rule.new).String())
		}
		if strings.HasPrefix(attr.name, "hx-on:") {
			checkHXOnEvent(current, attr.name, path, issues)
		}
		if attr.name == "hx-swap" && attr.value != "" {
			checkSwapSyntax(current, attr.value, path, issues)
		}
	}
}

func findRename(rules []renameRule, old string) (renameRule, bool) {
	for _, rule := range rules {
		if rule.old == old {
			return rule, true
		}
	}
	return renameRule{}, false
}

func addIssue(issues *[]Issue, path string, line int, category, message string) {
	*issues = append(*issues, Issue{Path: path, Category: category, Line: line, Message: message})
}

func checkHXOnEvent(current *node, attrName, path string, issues *[]Issue) {
	var tb textbuf.Buffer
	event, htmxEvent := strings.CutPrefix(attrName, "hx-on::")
	if htmxEvent {
		event = tb.Str("htmx:").Str(event).String()
	} else {
		event, _ = strings.CutPrefix(attrName, "hx-on:")
	}
	if rule, ok := oldEventsByLower[event]; ok {
		addIssue(issues, path, current.line, "renamed-event", tb.Reset().
			Str(attrName).Str(` uses old event name "`).Str(rule.old).
			Str(`" → use hx-on:`).Str(rule.new).String())
		return
	}
	if original, ok := removedByLower[event]; ok {
		addIssue(issues, path, current.line, "removed-event",
			tb.Reset().Str(attrName).Str(" uses removed event ").Str(original).String())
	}
}

func checkSwapSyntax(current *node, value, path string, issues *[]Issue) {
	var tb textbuf.Buffer
	kind, selector, position, ok := findSwapSyntax(value)
	if !ok {
		return
	}
	addIssue(issues, path, current.line, "swap-syntax", tb.Str("old ").Str(kind).
		Byte(':').Str(selector).Byte(':').Str(position).Str(" syntax → use ").
		Str(kind).Byte(':').Str(position).Byte(' ').Str(kind).Str("Target:").
		Str(selector).String())
}

func findSwapSyntax(value string) (string, string, string, bool) {
	for offset := range len(value) {
		var kind string
		switch {
		case strings.HasPrefix(value[offset:], "show:"):
			kind = "show"
		case strings.HasPrefix(value[offset:], "scroll:"):
			kind = "scroll"
		default:
			continue
		}

		start := offset + len(kind) + 1
		end := start
		for end < len(value) {
			current, width := utf8.DecodeRuneInString(value[end:])
			if current == ':' || scannerSpace(current) {
				break
			}
			end += width
		}
		if end == start || end >= len(value) || value[end] != ':' {
			continue
		}
		for _, position := range []string{"top", "bottom"} {
			if strings.HasPrefix(value[end+1:], position) {
				return kind, value[start:end], position, true
			}
		}
	}
	return "", "", "", false
}

// The tails htmx 4.0.0 writes on an inheritance finding, quoted from its
// upgrade checker so a reader can compare them word for word. The first is the
// whole message for a header carrier with no requesting descendant. The second
// is appended to either message when the carrier names a token.
const (
	headerCarrierAdvice = " needs :inherited suffix to reach descendants " +
		"(no request attribute in this file; if this is a base template, " +
		"the requests are in other files)"
	csrfAdvice = " (this looks like a CSRF token; without :inherited the header " +
		"does not reach child elements and the server rejects the request)"
)

func checkInheritance(root *node, path string, issues *[]Issue) {
	boosted := collectBoosted(root)
	stack := []*node{root}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		checkInheritedAttrs(current, boosted, path, issues)
		for _, child := range slices.Backward(current.children) {
			stack = append(stack, child)
		}
	}
}

func checkInheritedAttrs(current *node, boosted map[*node]bool, path string, issues *[]Issue) {
	var tb textbuf.Buffer
	for _, attr := range current.attrs {
		if !inheritableAttrs[attr.name] || strings.Contains(attr.name, ":inherited") {
			continue
		}
		if attr.name == attrBoost {
			if descendant := firstDescendantTag(current, "a", "form"); descendant != nil {
				addIssue(issues, path, current.line, "inheritance", tb.Reset().
					Str(attr.name).Str(" needs :inherited suffix (descendant <").
					Str(descendant.tag).Str("> on line ").Int(int64(descendant.line)).
					Str(" will no longer inherit it)").String())
			}
			continue
		}
		descendant, source := firstRequestDescendant(current, boosted)
		if descendant != nil {
			addIssue(issues, path, current.line, "inheritance", tb.Reset().
				Str(attr.name).Str(" needs :inherited suffix (descendant on line ").
				Int(int64(descendant.line)).Str(" has ").Str(source).Byte(')').
				Str(csrfHint(attr.value)).String())
			continue
		}
		if attr.name != attrHeaders {
			continue
		}
		// No request attribute in this file, and hx-headers is still almost
		// always a defect here. The htmx 2 CSRF recipe puts hx-headers on
		// <body> in a base template, and the requests live in the child
		// templates. A one-file scan cannot see them.
		addIssue(issues, path, current.line, "inheritance", tb.Reset().
			Str(attr.name).Str(headerCarrierAdvice).Str(csrfHint(attr.value)).String())
	}
}

// csrfHint returns the warning htmx 4.0.0 appends when the carrier's value
// holds a CSRF token, and an empty string when it does not.
func csrfHint(value string) string {
	if !csrfValue.MatchString(value) {
		return ""
	}
	return csrfAdvice
}

// collectBoosted returns the nodes htmx 2 boosts. hx-boost was implicitly
// inherited, so an <a href> or a <form> makes a request when any ancestor
// turns boost on. That makes it a request source although it carries no
// hx-get or hx-post. The walk is iterative because the tree depth is the
// scanned file's, not this scanner's.
func collectBoosted(root *node) map[*node]bool {
	type frame struct {
		current *node
		boost   bool
	}

	boosted := make(map[*node]bool)
	stack := []frame{{current: root}}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last].current
		boost := boostState(current, stack[last].boost)
		stack = stack[:last]
		if boost && isBoostable(current) {
			boosted[current] = true
		}
		for _, child := range slices.Backward(current.children) {
			stack = append(stack, frame{current: child, boost: boost})
		}
	}
	return boosted
}

// boostState answers whether boost is on below this node. A suffixed
// hx-boost:inherited counts, so the name is matched before its suffix. Every
// value other than "false" turns boost on, and a valueless attribute is one
// of them.
func boostState(current *node, inherited bool) bool {
	boost := inherited
	for _, attr := range current.attrs {
		name, _, _ := strings.Cut(attr.name, ":")
		if name != attrBoost {
			continue
		}
		boost = strings.ToLower(strings.TrimSpace(attr.value)) != "false"
	}
	return boost
}

func isBoostable(current *node) bool {
	if current.tag == "form" {
		return true
	}
	if current.tag != "a" {
		return false
	}
	for _, attr := range current.attrs {
		if attr.name == "href" {
			return true
		}
	}
	return false
}

func firstDescendantTag(parent *node, tags ...string) *node {
	stack := descendantStack(parent)
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if slices.Contains(tags, current.tag) {
			return current
		}
		for _, child := range slices.Backward(current.children) {
			stack = append(stack, child)
		}
	}
	return nil
}

// firstRequestDescendant returns the first descendant that makes a request,
// and the reason it makes one: the request attribute it carries, or the boost
// it inherits.
func firstRequestDescendant(parent *node, boosted map[*node]bool) (*node, string) {
	var tb textbuf.Buffer
	stack := descendantStack(parent)
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		for _, attr := range current.attrs {
			if requestAttrs[attr.name] {
				return current, attr.name
			}
		}
		if boosted[current] {
			return current, tb.Str("boosted <").Str(current.tag).Byte('>').String()
		}
		for _, child := range slices.Backward(current.children) {
			stack = append(stack, child)
		}
	}
	return nil, ""
}

func descendantStack(parent *node) []*node {
	stack := make([]*node, 0, len(parent.children))
	for _, child := range slices.Backward(parent.children) {
		stack = append(stack, child)
	}
	return stack
}

func checkText(raw []byte, path string, issues *[]Issue) {
	var tb textbuf.Buffer
	lines := splitLines(string(raw))
	for index, line := range lines {
		lineNumber := index + 1
		for _, rule := range allEventRenames {
			if !strings.Contains(line, rule.old) {
				continue
			}
			if hasHXOnAttribute(line) {
				continue
			}
			addIssue(issues, path, lineNumber, "old-event", tb.Reset().
				Str(`old event name "`).Str(rule.old).Str(`" → "`).Str(rule.new).
				Byte('"').String())
		}
		for _, event := range removedEvents {
			if !strings.Contains(line, event) {
				continue
			}
			if hasHXOnAttribute(line) {
				continue
			}
			addIssue(issues, path, lineNumber, "removed-event", tb.Reset().
				Str(`removed event "`).Str(event).Byte('"').String())
		}
		for _, rule := range removedJSAPI {
			if hasCall(line, rule.old) {
				addIssue(issues, path, lineNumber, "old-api", tb.Reset().Str(rule.old).
					Str("() is removed → ").Str(rule.new).String())
			}
		}
		for _, rule := range configRenames {
			if hasConfigKey(line, rule.old) {
				addIssue(issues, path, lineNumber, "renamed-config", tb.Reset().
					Str(`config "`).Str(rule.old).Str(`" → "`).Str(rule.new).
					Byte('"').String())
			}
		}
		for _, key := range removedConfig {
			if hasConfigKey(line, key) {
				addIssue(issues, path, lineNumber, "removed-config", tb.Reset().
					Str(`config "`).Str(key).Str(`" is removed`).String())
			}
		}
		for _, rule := range removedResponseHeaders {
			if strings.Contains(line, rule.old) {
				addIssue(issues, path, lineNumber, "removed-header", tb.Reset().Byte('"').
					Str(rule.old).Str(`" is removed → `).Str(rule.new).String())
			}
		}
	}
}

func splitLines(text string) []string {
	text = lineBreaks.Replace(text)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func hasHXOnAttribute(line string) bool {
	remaining := line
	for {
		index := strings.Index(remaining, "hx-on")
		if index < 0 {
			return false
		}
		after := remaining[index+len("hx-on"):]
		for _, current := range after {
			if scannerSpace(current) {
				break
			}
			if current == '=' {
				return true
			}
		}
		remaining = after
	}
}

func hasCall(line, name string) bool {
	remaining := line
	for {
		index := strings.Index(remaining, name)
		if index < 0 {
			return false
		}
		after := remaining[index+len(name):]
		after = strings.TrimLeftFunc(after, scannerSpace)
		if strings.HasPrefix(after, "(") {
			return true
		}
		remaining = remaining[index+len(name):]
	}
}

func hasConfigKey(line, key string) bool {
	for _, prefix := range []string{"config.", `"`, `'`} {
		remaining := line
		needle := prefix + key
		for {
			index := strings.Index(remaining, needle)
			if index < 0 {
				break
			}
			after := remaining[index+len(needle):]
			if strings.HasPrefix(after, `"`) || strings.HasPrefix(after, `'`) {
				return true
			}
			after = strings.TrimLeftFunc(after, scannerSpace)
			if strings.HasPrefix(after, "=") || strings.HasPrefix(after, ":") {
				return true
			}
			remaining = remaining[index+len(needle):]
		}
	}
	return false
}
