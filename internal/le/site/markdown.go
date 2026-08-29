// Design: website/AI.md -- one Markdown pipeline renders every Markdown-sourced page
// Detail: shell.go wraps the body this file produces; mirror.go writes the Markdown sibling.
package site

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// markdownEngine converts a page source into the body HTML a page shell wraps.
//
// Every option is written out rather than left to a library default, so this
// call states the pipeline it is.
//
//   - Table is the one extension. The retired renderer ran python-markdown with
//     tables, fenced_code, sane_lists and toc; goldmark parses fenced code and
//     CommonMark lists itself, and this file builds the table of contents, so
//     the table syntax is all that is left to add. Strikethrough, linkify and
//     task lists are NOT added: python-markdown parsed none of them, so adding
//     them would change what a published page says.
//   - WithAutoHeadingID gives every heading the id its table of contents links
//     to. The slugs are goldmark's own and differ from python-markdown's; the
//     owner accepted that on 2026-08-29, and the page's own contents list is
//     built from the same ids, so it stays self-consistent.
//   - WithUnsafe passes raw HTML through, which python-markdown also did. Every
//     source is a file in this repository, so the input is trusted; a page
//     rendered from a source outside this repository would need this reviewed.
var markdownEngine = goldmark.New(
	goldmark.WithExtensions(extension.Table),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
)

// docHeading is one entry of a page's table of contents.
type docHeading struct {
	Level int
	ID    string
	Label string
}

// renderMarkdown converts one page source into body HTML and the headings its
// table of contents is built from.
//
// The source is parsed once and rendered from the same tree, so the heading ids
// in the answer are the ids the body carries.
func renderMarkdown(source []byte) (string, []docHeading, error) {
	document := markdownEngine.Parser().Parse(text.NewReader(source))
	headings := documentHeadings(document, source)
	var body bytes.Buffer
	if err := markdownEngine.Renderer().Render(&body, source, document); err != nil {
		return "", nil, fmt.Errorf("render markdown: %w", err)
	}
	return body.String(), headings, nil
}

// documentHeadings answers every heading of one parsed page, in reading order.
//
// The walk is over a tree this repository's own Markdown produced, so its depth
// is bounded by the nesting of that source rather than by anything a peer
// chooses.
func documentHeadings(document ast.Node, source []byte) []docHeading {
	var headings []docHeading
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, isHeading := node.(*ast.Heading)
		if !isHeading {
			return ast.WalkContinue, nil
		}
		id, hasID := heading.AttributeString("id")
		if !hasID {
			return ast.WalkSkipChildren, nil
		}
		identifier, isBytes := id.([]byte)
		if !isBytes {
			return ast.WalkSkipChildren, nil
		}
		headings = append(headings, docHeading{
			Level: heading.Level,
			ID:    string(identifier),
			Label: inlineText(heading, source),
		})
		return ast.WalkSkipChildren, nil
	})
	return headings
}

// inlineText answers the text a reader sees inside one inline container, with
// every mark dropped. A heading that reads "`ze bgp` state" gives back
// "ze bgp state", which is what the contents list shows.
func inlineText(node ast.Node, source []byte) string {
	var label strings.Builder
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := child.(type) {
		case *ast.Text:
			label.Write(typed.Segment.Value(source))
		case *ast.String:
			label.Write(typed.Value)
		case *ast.RawHTML, *ast.AutoLink:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(label.String())
}

// frontMatterFence opens and closes the metadata block a page source may carry
// before its Markdown body.
const frontMatterFence = "---"

// parseFrontMatter splits one page source into its scalar metadata and its
// Markdown body. A source with no metadata block answers an empty map and the
// source unchanged.
//
// A malformed block is an operating error rather than a programmer error: the
// source is a file an author edits, so a missing colon or a repeated key is a
// mistake a build must name rather than pass over. Every value is a string;
// nothing in this site's front matter is a list or a nested map.
func parseFrontMatter(source []byte) (map[string]string, []byte, error) {
	metadata := map[string]string{}
	text := string(source)
	rest, opened := strings.CutPrefix(text, frontMatterFence+"\n")
	if !opened {
		return metadata, source, nil
	}
	block, body, closed := strings.Cut(rest, "\n"+frontMatterFence+"\n")
	if !closed {
		return nil, nil, fmt.Errorf("front matter opens with %s and never closes", frontMatterFence)
	}
	for offset, line := range strings.Split(block, "\n") {
		number := offset + 2
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, separated := strings.Cut(trimmed, ":")
		if !separated {
			return nil, nil, fmt.Errorf("front matter line %d must be `key: value`", number)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return nil, nil, fmt.Errorf("front matter line %d has an empty key", number)
		}
		if _, repeated := metadata[key]; repeated {
			return nil, nil, fmt.Errorf("duplicate front matter key: %s", key)
		}
		metadata[key] = unquoteFrontMatter(strings.TrimSpace(value))
	}
	return metadata, []byte(body), nil
}

// unquoteFrontMatter drops one matched pair of surrounding quotes, so a value
// that needs a leading space or a colon can be written quoted.
func unquoteFrontMatter(value string) string {
	if len(value) < 2 {
		return value
	}
	quote := value[0]
	if quote != '\'' && quote != '"' {
		return value
	}
	if value[len(value)-1] != quote {
		return value
	}
	return value[1 : len(value)-1]
}

// renderDocTOC builds the "On this page" navigation from a page's headings.
//
// Only level 2 and deeper appear: the level 1 heading is the page title and
// linking a page to its own top adds nothing. A heading with no id or no text
// is dropped, and a page left with no heading gets no navigation rather than an
// empty one.
func renderDocTOC(headings []docHeading) string {
	var listed []docHeading
	for _, heading := range headings {
		if heading.Level >= 2 && heading.ID != "" && heading.Label != "" {
			listed = append(listed, heading)
		}
	}
	if len(listed) == 0 {
		return ""
	}
	items, _ := tocItems(listed, 0, listed[0].Level)
	if items == "" {
		return ""
	}
	return `<nav class="doc-toc" aria-labelledby="doc-toc-title">` +
		`<h2 id="doc-toc-title">On this page</h2>` +
		"<ol>\n" + items + "\n</ol></nav>"
}

// tocItems renders the headings at one level and answers the index it stopped
// at, so the caller can continue from the first heading it did not consume.
//
// The recursion is over heading levels, which run from 2 to 6, so it is five
// frames deep at most and the source that feeds it is a file in this
// repository rather than input a peer chooses.
func tocItems(headings []docHeading, from, level int) (string, int) {
	var items []string
	index := from
	for index < len(headings) {
		heading := headings[index]
		if heading.Level < level {
			break
		}
		if heading.Level > level {
			nested, next := tocItems(headings, index, heading.Level)
			index = next
			if nested == "" {
				continue
			}
			if len(items) == 0 {
				items = append(items, nested)
				continue
			}
			items[len(items)-1] = strings.TrimSuffix(items[len(items)-1], "</li>") +
				"\n<ol>\n" + nested + "\n</ol></li>"
			continue
		}
		index++
		items = append(items, fmt.Sprintf(`<li><a href="#%s">%s</a></li>`,
			html.EscapeString(heading.ID), html.EscapeString(heading.Label)))
	}
	return strings.Join(items, "\n"), index
}

// insertDocTOC splices the navigation after the FIRST literal "</div>" of the
// body, which is where the page's hero block closes.
//
// The rule is literal on purpose. The retired renderer searched for that exact
// string, so a body whose first "</div>" belongs to something else got its
// navigation there, and copying the rule keeps every published page where it
// was. A body with no "</div>" takes the navigation at the top.
func insertDocTOC(body, toc string) string {
	if toc == "" {
		return body
	}
	const marker = "</div>"
	position := strings.Index(body, marker)
	if position < 0 {
		return toc + "\n" + body
	}
	end := position + len(marker)
	return body[:end] + "\n" + toc + body[end:]
}
