// Design: (none -- test utility, no architecture doc)

package golden

import (
	"html"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// normalizeHTML rewrites markup into the form a reader receives it as.
//
// It is the instrument AC-2 of spec-web-templ-migration requires. A
// port to templ is proven by rendering the same data through both engines and
// comparing the normalized forms.
//
// Byte identity is not reachable, because templ normalizes its own output and
// no flag turns that off. generateDocType lowercases the doctype. isInlineOrText
// drops the whitespace between two nodes unless both are inline or text
// (vendor/github.com/a-h/templ/generator/generator.go).
//
// Five rules, applied to both sides. AC-2 names the same five.
//
//   - the doctype is lowercased, which is all generateDocType does to it.
//   - whitespace inside a tag becomes one space, and none against the bracket.
//   - a run of whitespace in a text node becomes one space.
//   - an attribute value is written in double quotes.
//   - a character reference is decoded and written again in one spelling.
//
// The text-node space is dropped where HTML drops it. That is against a block
// element, at the start of an element's content, and at its end.
//
// The delimiter rule follows templ. writeExpressionAttribute writes an
// expression attribute one way only. A single-quoted source value therefore
// moves the delimiter and nothing else.
//
// The reference rule follows the two escapers. html/template escapes +, = and `
// in its own table. templ.EscapeString escapes five characters and none of
// those three. The two also disagree on a bare & inside an attribute. Every
// such pair decodes to the same text.
//
// Text inside <pre> or <textarea> keeps its whitespace byte for byte. Every
// byte there reaches the reader, so a newline and a space are different
// content. The character-reference rule still applies inside it, because a
// config diff line starts with a + that one engine spells &#43;.
//
// Decoding does not hide a double escape. `&amp;lt;x&amp;gt;` decodes to
// `&lt;x&gt;` and is written again as `&amp;lt;x&amp;gt;`, which is not what a
// value escaped once becomes. Raw markup does not hide either: a real <script>
// is a tag, and an escaped one is text, so the two never meet.
//
// Between two INLINE neighbors that space survives. A space between </a> and
// <a>, or between an expression and the <span> after it, still counts. That is
// the case a port can break, so it is the case this keeps.
//
// The block set is templ's own (parser/v2/types.go, blockElements), so the
// pre-port side is normalized by the same rule the generator applies to the
// ported one.
//
// The <pre> exemption is load-bearing. templ knows two raw elements, style and
// script (parser/v2/templateparser.go, rawElements), and <pre> is neither.
// writeWhitespaceTrailer rewrites a vertical space to a horizontal one
// everywhere (generator/generator.go), and it writes nothing at all where the
// next node is a block. A port that leaves a newline as source layout loses it.
//
// configCommit (internal/component/web/config_commit.templ) writes one newline
// after each </span> inside <pre class="commit-diff-output">, and those newlines
// ARE the line breaks of the config diff. A collapse here would call that port
// faithful and ship the diff on one line. A port keeps such a newline by
// writing it as an expression, { "\n" }, which reaches the output through
// templ.EscapeString.
//
// This function holds no baseline. The pre-port bytes are read out of git,
// which is why nothing here can fossilize into a copy that outlives the port.
func normalizeHTML(src string) string {
	tokens := tokenizeHTML(src)

	var b textbuf.Buffer

	depth := 0

	for i, tok := range tokens {
		if tok.tag {
			b.Str(tok.text)

			depth = preserveDepth(depth, tok)

			continue
		}

		if depth > 0 {
			b.Str(canonicalRefs(tok.text))

			continue
		}

		b.Str(normalizeText(canonicalRefs(tok.text), prevOf(tokens, i), nextOf(tokens, i)))
	}

	return b.String()
}

// preserveElements are the elements whose text content is content. HTML renders
// every byte between <pre> and </pre>, and a textarea's body is the value the
// reader edits.
var preserveElements = map[string]bool{"pre": true, "textarea": true}

// preserveDepth tracks how many preserved elements enclose the next token. An
// unclosed <pre> holds the depth open to the end of the input, so the text it
// covers is compared byte for byte. That direction is the safe one: it reports a
// difference the port must explain, rather than erasing one.
func preserveDepth(depth int, tok htmlToken) int {
	if !preserveElements[tok.name] {
		return depth
	}

	if tok.close {
		if depth == 0 {
			return 0
		}

		return depth - 1
	}

	if strings.HasSuffix(tok.text, "/>") {
		return depth
	}

	return depth + 1
}

// htmlToken is one tag or one run of text.
type htmlToken struct {
	tag   bool
	open  bool
	close bool
	name  string
	text  string
}

func prevOf(tokens []htmlToken, i int) *htmlToken {
	if i == 0 {
		return nil
	}

	return &tokens[i-1]
}

func nextOf(tokens []htmlToken, i int) *htmlToken {
	if i+1 >= len(tokens) {
		return nil
	}

	return &tokens[i+1]
}

// normalizeText collapses one text run and drops the space HTML drops.
func normalizeText(text string, prev, next *htmlToken) string {
	collapsed := collapseSpace(text)
	if collapsed == "" {
		return ""
	}

	// The start of the input, the inside edge of an open tag, and any block
	// element boundary all swallow a leading space.
	dropLeading := prev == nil || prev.open || isBlockElement(prev.name)
	dropTrailing := next == nil || next.close || isBlockElement(next.name)

	if strings.TrimSpace(collapsed) == "" {
		if dropLeading || dropTrailing {
			return ""
		}

		return " "
	}

	if dropLeading {
		collapsed = strings.TrimPrefix(collapsed, " ")
	}

	if dropTrailing {
		collapsed = strings.TrimSuffix(collapsed, " ")
	}

	return collapsed
}

// collapseSpace turns every run of whitespace into one space.
func collapseSpace(s string) string {
	var b textbuf.Buffer

	space := false

	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			space = true

			continue
		}

		if space {
			b.Byte(' ')

			space = false
		}

		b.WriteRune(r)
	}

	if space {
		b.Byte(' ')
	}

	return b.String()
}

// tokenizeHTML splits markup into tags and text runs.
func tokenizeHTML(src string) []htmlToken {
	var tokens []htmlToken

	for i := 0; i < len(src); {
		if src[i] != '<' {
			end := strings.IndexByte(src[i:], '<')
			if end < 0 {
				tokens = append(tokens, htmlToken{text: src[i:]})

				break
			}

			tokens = append(tokens, htmlToken{text: src[i : i+end]})
			i += end

			continue
		}

		end := tagEnd(src, i)
		tokens = append(tokens, tagToken(src[i:end]))
		i = end
	}

	return tokens
}

// tagEnd returns the index just past the tag that starts at i. It skips a '>'
// inside a quoted attribute value.
func tagEnd(src string, i int) int {
	var quote byte

	for j := i; j < len(src); j++ {
		c := src[j]

		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return j + 1
		}
	}

	return len(src)
}

// tagToken normalizes one tag and reads its element name.
func tagToken(raw string) htmlToken {
	tok := htmlToken{tag: true}

	if strings.HasPrefix(raw, "<!") {
		// A doctype or a comment. Both are a boundary, and the doctype's case
		// is the one templ changes.
		tok.text = raw
		if strings.HasPrefix(strings.ToLower(raw), "<!doctype") {
			tok.text = strings.ToLower(raw)
		}

		return tok
	}

	body := raw[1:]
	tok.close = strings.HasPrefix(body, "/")
	tok.open = !tok.close

	if tok.close {
		body = body[1:]
	}

	tok.name = strings.ToLower(elementName(body))
	tok.text = normalizeTag(raw)

	return tok
}

// canonicalRefs decodes every character reference and writes the result again
// in one spelling. It is how two escapers with different tables are compared by
// the text they produce for a reader rather than by the bytes they chose.
//
// html.EscapeString covers &, ', <, > and ". A + or an = stays literal, which
// is the templ spelling. The function is idempotent: a second pass decodes what
// the first one wrote and writes it again the same way.
func canonicalRefs(s string) string {
	if !strings.ContainsAny(s, "&<>'\"") {
		return s
	}

	return html.EscapeString(html.UnescapeString(s))
}

// elementName reads the leading name of a tag body.
func elementName(body string) string {
	end := strings.IndexFunc(body, func(r rune) bool {
		inName := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-'

		return !inName
	})
	if end < 0 {
		return body
	}

	return body[:end]
}

// normalizeTag writes one tag in a canonical spelling. It writes the element
// name, then one space before each attribute, then the closing bracket. No
// space sits against that bracket. Every value is written in double quotes, and
// every character reference inside it in one encoding.
//
// The space rule is the text rule carried to the end of the tag. An HTML
// tokenizer reads whitespace there in the before-attribute-name state and emits
// no attribute for it. `<a href="x" >` and `<a href="x">` are one element.
// Markup that puts a conditional attribute on its own line leaves that space
// behind whenever the condition is false. templ writes none, so without this
// rule every such tag reads as a difference.
//
// The quoting rule has the same shape. writeExpressionAttribute
// (vendor/github.com/a-h/templ/generator/generator.go) writes an expression
// attribute in double quotes, and escapes the value. A source value in single
// quotes therefore reaches the browser in another delimiter, unchanged.
// hx-vals carries JSON in this repository, which is where that pair is met.
//
// A tag with no closing bracket keeps none. Truncated output must read as a
// difference, so this function never supplies a bracket the input lacks.
func normalizeTag(raw string) string {
	var b textbuf.Buffer

	body := strings.TrimSuffix(strings.TrimSuffix(raw, ">"), "/")
	end := raw[len(body):]

	i := 1

	b.Byte('<')

	if strings.HasPrefix(body[i:], "/") {
		b.Byte('/')

		i++
	}

	name := elementName(body[i:])
	b.Str(name)

	i += len(name)

	for i < len(body) {
		for i < len(body) && isTagSpace(body[i]) {
			i++
		}

		if i >= len(body) {
			break
		}

		attr, next := tagAttribute(body, i)
		i = next

		b.Byte(' ').Str(attr)
	}

	b.Str(end)

	return b.String()
}

// tagAttribute reads one attribute at i and returns it in canonical form,
// together with the index after it. A valueless attribute keeps its bare name,
// because dropping one is a real difference (hidden, selected, disabled).
func tagAttribute(body string, i int) (string, int) {
	start := i

	for i < len(body) && !isTagSpace(body[i]) && body[i] != '=' {
		i++
	}

	name := body[start:i]

	for i < len(body) && isTagSpace(body[i]) {
		i++
	}

	if i >= len(body) || body[i] != '=' {
		return name, i
	}

	i++

	for i < len(body) && isTagSpace(body[i]) {
		i++
	}

	value, next := tagValue(body, i)

	return name + `="` + canonicalRefs(value) + `"`, next
}

// tagValue reads an attribute value at i, quoted or bare, and returns it with
// the index after it. An unterminated quoted value runs to the end of the tag,
// which is what an HTML tokenizer does with it.
func tagValue(body string, i int) (string, int) {
	if i < len(body) && (body[i] == '"' || body[i] == '\'') {
		quote := body[i]

		end := strings.IndexByte(body[i+1:], quote)
		if end < 0 {
			return body[i+1:], len(body)
		}

		return body[i+1 : i+1+end], i + end + 2
	}

	start := i
	for i < len(body) && !isTagSpace(body[i]) {
		i++
	}

	return body[start:i], i
}

// isTagSpace reports whether c separates two parts of a tag.
func isTagSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// blockElements is templ's own block set
// (vendor/github.com/a-h/templ/parser/v2/types.go, blockElements). The pre-port
// side is normalized by the rule the generator applies to the ported one.
//
// pre is in this set and in preserveElements. The two rules govern different
// sides of the same tag. This set drops the space OUTSIDE <pre>, which is what
// templ does there. preserveElements keeps every byte INSIDE it.
var blockElements = map[string]bool{
	"address": true, "article": true, "aside": true, "body": true,
	"blockquote": true, "canvas": true, "dd": true, "div": true, "dl": true,
	"dt": true, "fieldset": true, "figcaption": true, "figure": true,
	"footer": true, "form": true, "h1": true, "h2": true, "h3": true,
	"h4": true, "h5": true, "h6": true, "head": true, "header": true,
	"hr": true, "html": true, "li": true, "main": true, "meta": true,
	"nav": true, "noscript": true, "ol": true, "p": true, "pre": true,
	"script": true, "section": true, "table": true, "template": true,
	"tfoot": true, "turbo-stream": true, "ul": true, "video": true,
	"title": true, "style": true, "link": true, "td": true, "th": true,
	"tr": true, "br": true,
}

// isBlockElement reports whether a name is a block element. An empty name is a
// doctype, a comment or a malformed tag, and each is a boundary.
func isBlockElement(name string) bool {
	if name == "" {
		return true
	}

	return blockElements[name]
}
