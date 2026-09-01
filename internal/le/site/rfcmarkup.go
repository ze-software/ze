// Design: website/AI.md -- the markup and markdown primitives the RFC pages share
// Overview: rfcdetail.go -- the per-RFC page these build; rfccompliance.go -- its index
//
// One requirement id, one table and one escaped cell are each rendered ONCE
// here, for the page and for its Markdown mirror. A second speller of a
// requirement anchor is a link that resolves on one rendering and not on the
// other (ai/rules/principles.md).
package site

import (
	"html"
	"strconv"
	"strings"
)

// rfcAnchor is the id one requirement's row carries and the fragment every
// other mention of that requirement links to.
//
// Lowercased, because a fragment is typed, mailed and pasted by a reader, and
// the requirement id is the only thing that has to be stable about it.
func rfcAnchor(rid string) string { return strings.ToLower(rid) }

// rfcRequirementRefHTML names one requirement wherever it is mentioned away
// from its own row: the id, linked to that row, and the requirement's own text
// beside it.
//
// A bare id tells a reader which line of a shard to go and read. This page
// carries the line, so the mention carries it too (owner review, 2026-09-01).
func rfcRequirementRefHTML(rid, text string) string {
	out := `<a href="#` + html.EscapeString(rfcAnchor(rid)) + `"><code>` +
		html.EscapeString(rid) + "</code></a>"
	if text == "" {
		return out
	}
	return out + " " + html.EscapeString(text)
}

// rfcRequirementRefMirror states the same mention in the mirror.
func rfcRequirementRefMirror(rid, text string) string {
	out := "[`" + rid + "`](#" + rfcAnchor(rid) + ")"
	if text == "" {
		return out
	}
	return out + " " + text
}

// rfcTableHTML wraps one table in the container that scrolls it.
//
// These tables are wide by nature: a requirement row carries quoted RFC prose
// beside a list of test paths, and a path is one unbreakable token. The
// container scrolls, so the page body never does.
func rfcTableHTML(head, rows string) string {
	return `<div class="rfc-table-wrap">` + "\n<table>\n<thead><tr>" + head +
		"</tr></thead>\n<tbody>\n" + rows + "</tbody>\n</table>\n</div>"
}

// rfcHeadCells answers one table's header row.
func rfcHeadCells(labels ...string) string {
	var out strings.Builder
	for _, label := range labels {
		out.WriteString("<th>" + html.EscapeString(label) + "</th>")
	}
	return out.String()
}

// rfcRowCells answers one table row from cells that are already markup.
func rfcRowCells(cells ...string) string {
	var out strings.Builder
	out.WriteString("<tr>")
	for _, cell := range cells {
		out.WriteString("<td>" + cell + "</td>\n")
	}
	out.WriteString("</tr>\n")
	return out.String()
}

// rfcMirrorRow answers one markdown row from cells that are already escaped for
// a table.
func rfcMirrorRow(cells ...string) string {
	return "| " + strings.Join(cells, " | ") + " |\n"
}

// rfcMirrorHead answers one markdown header row and its alignment line.
func rfcMirrorHead(labels ...string) string {
	rule := make([]string, len(labels))
	for index := range rule {
		rule[index] = "---"
	}
	return rfcMirrorRow(labels...) + "|" + strings.Join(rule, "|") + "|\n"
}

// rfcInlineHTML renders one shard cell as HTML: every character escaped, with
// the cell's own backtick spans and bold runs turned into markup.
//
// The cells arrive as the markdown the generated shard carries, which is what
// makes the site and the repository state one thing about a requirement. Every
// segment between the markers is escaped, so a pipe, an angle bracket or an
// ampersand in quoted RFC prose lands inside its own cell and breaks neither
// the row nor the markup (AC-12).
func rfcInlineHTML(cell string) string {
	if cell == "" {
		return "-"
	}
	var out strings.Builder
	for index, segment := range strings.Split(cell, "`") {
		if index%2 == 1 {
			out.WriteString("<code>" + html.EscapeString(segment) + "</code>")
			continue
		}
		out.WriteString(rfcBoldHTML(segment))
	}
	return out.String()
}

// rfcBoldHTML escapes one run of prose and turns its `**...**` pairs into
// <strong>. An unpaired marker is escaped and left visible rather than opening
// an element the rest of the page has to close.
func rfcBoldHTML(text string) string {
	parts := strings.Split(text, "**")
	if len(parts)%2 == 0 {
		return html.EscapeString(text)
	}
	var out strings.Builder
	for index, part := range parts {
		if index%2 == 1 {
			out.WriteString("<strong>" + html.EscapeString(part) + "</strong>")
			continue
		}
		out.WriteString(html.EscapeString(part))
	}
	return out.String()
}

// rfcOrUnstated answers a value, or says the record states none. An empty cell
// reads as a rendering fault rather than as a fact.
func rfcOrUnstated(value string) string {
	if strings.TrimSpace(value) == "" {
		return "not stated"
	}
	return value
}

// rfcPlain turns one already-escaped HTML fact back into the plain text the
// mirror states.
//
// The at-a-glance facts are built once and read by both renderings (AC-9), and
// the HTML one is the source because it is the one that has to be escaped. The
// mirror unwinds the two things that list puts in: a code span, and the five
// entities html.EscapeString writes.
func rfcPlain(value string) string {
	replacer := strings.NewReplacer(
		"<code>", "`", "</code>", "`",
		"&amp;", "&", "&lt;", "<", "&gt;", ">", "&#34;", `"`, "&#39;", "'")
	return replacer.Replace(value)
}

// The code host one repository path resolves to. blobRoot is the whole
// repository rather than one subtree of it, because two callers need two
// subtrees: a documentation link resolves under docs/, and a test tag resolves
// wherever the test lives.
const (
	codeHostRoot = "https://github.com/ze-software/ze/"
	codeHostBlob = codeHostRoot + "blob/main/"
	codeHostTree = codeHostRoot + "tree/main/"
)

// repositoryBlobURL answers where one repository path is published, and
// repositoryLineURL addresses one line inside it.
//
// A line number is what turns a link into an answer: a reader following a tag
// lands on the assertion rather than on a 900-line file. A line of zero, which
// is what a tag with no recorded line answers, addresses the file.
func repositoryBlobURL(repoPath string) string { return codeHostBlob + repoPath }

func repositoryLineURL(repoPath string, line int) string {
	if line <= 0 {
		return repositoryBlobURL(repoPath)
	}
	return repositoryBlobURL(repoPath) + "#L" + strconv.Itoa(line)
}

// rfcFoldOver is the width past which prose goes behind a disclosure rather
// than in front of the reader.
//
// The public ledger's cells run from six words to nine hundred, and the long
// ones are what made the page unreadable (owner review, 2026-09-01). Folding
// every cell would hide a sentence a reader could have taken at a glance, and
// folding none reproduces the wall. This is where a paragraph stops being a
// sentence: about three printed lines at the page's own measure.
const rfcFoldOver = 240

// rfcFoldMarkupHTML folds ALREADY-RENDERED markup, deciding by the width of the
// prose it came from.
//
// The fold is about how much a reader is asked to take at once, which is a
// property of the words rather than of the tags around them. Measuring the
// markup would fold a short sentence carrying two links.
func rfcFoldMarkupHTML(label, prose, markup string) string {
	if len(prose) <= rfcFoldOver {
		return "<p><strong>" + html.EscapeString(label) + ":</strong></p>\n" + markup
	}
	return "<details class=\"rfc-fold\"><summary>" + html.EscapeString(label) +
		"</summary>\n" + markup + "\n</details>"
}

// rfcFoldMarkupMirror states the same, with the label as a heading a mirror can
// carry.
func rfcFoldMarkupMirror(label, prose, body string) string {
	if len(prose) <= rfcFoldOver {
		return "**" + label + ":**\n\n" + body + "\n"
	}
	return "**" + label + "**\n\n" + body + "\n"
}

// rfcSpanRow answers one row whose single cell spans a whole table.
//
// The column count comes from the header labels rather than from a number
// written twice: this table gained and lost a column in one afternoon, and a
// hardcoded colspan is wrong the moment either happens again.
func rfcSpanRow(labels []string, cell string) string {
	return `<tr class="rfc-span"><td colspan="` + strconv.Itoa(len(labels)) + `">` +
		cell + "</td></tr>\n"
}
