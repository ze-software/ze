// Design: (none -- test utility, no architecture doc)

package golden

import "strings"

// NormalizeHTML rewrites markup into the form a reader receives it as.
//
// It is the instrument AC-2 of plan/spec-web-templ-migration.md requires. A
// port to templ is proven by rendering the same data through both engines and
// comparing the normalized forms.
//
// Byte identity is not reachable, because templ normalizes its own output and
// no flag turns that off. generateDocType lowercases the doctype. isInlineOrText
// drops the whitespace between two nodes unless both are inline or text
// (vendor/github.com/a-h/templ/generator/generator.go).
//
// Four rules, applied to both sides:
//
//   - the doctype is lowercased, which is all generateDocType does to it.
//   - whitespace inside a tag, between attributes and outside quoted values,
//     becomes one space.
//   - a run of whitespace in a text node becomes one space. That space is
//     dropped where HTML drops it: against a block element, at the start of an
//     element's content, and at its end.
//   - text inside <pre> or <textarea> is kept byte for byte. Every byte there
//     reaches the reader, so a newline and a space are different content.
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
// internal/component/web/templates/commit.html writes one newline after each
// </span> inside <pre class="commit-diff-output">, and those newlines ARE the
// line breaks of the config diff. A collapse here would call that port faithful
// and ship the diff on one line. A port keeps such a newline by writing it as an
// expression, { "\n" }, which reaches the output through templ.EscapeString.
//
// This function holds no baseline. The pre-port bytes are read out of git,
// which is why nothing here can fossilize into a copy that outlives the port.
func NormalizeHTML(src string) string {
	tokens := tokenizeHTML(src)

	var b strings.Builder

	depth := 0

	for i, tok := range tokens {
		if tok.tag {
			b.WriteString(tok.text)

			depth = preserveDepth(depth, tok)

			continue
		}

		if depth > 0 {
			b.WriteString(tok.text)

			continue
		}

		b.WriteString(normalizeText(tok.text, prevOf(tokens, i), nextOf(tokens, i)))
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
	var b strings.Builder

	space := false

	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			space = true

			continue
		}

		if space {
			b.WriteByte(' ')

			space = false
		}

		b.WriteRune(r)
	}

	if space {
		b.WriteByte(' ')
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
	tok.text = collapseTagSpace(raw)

	return tok
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

// collapseTagSpace collapses whitespace between attributes. It leaves a quoted
// value alone.
func collapseTagSpace(raw string) string {
	var (
		b     strings.Builder
		quote byte
		space bool
	)

	for i := range len(raw) {
		c := raw[i]

		if quote != 0 {
			b.WriteByte(c)

			if c == quote {
				quote = 0
			}

			continue
		}

		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			space = true

			continue
		}

		if space {
			b.WriteByte(' ')

			space = false
		}

		if c == '"' || c == '\'' {
			quote = c
		}

		b.WriteByte(c)
	}

	return b.String()
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
