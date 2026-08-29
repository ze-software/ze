// Design: website/AI.md -- the site publishes an RSS feed beside each dated section
// Detail: blog.go writes the article feed; changes.go the weekly one, at two paths.
package site

import (
	"fmt"
	"strings"
	"time"
)

// feedDeclaration opens every feed this site publishes.
const feedDeclaration = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

// feedEpoch is the build date a feed states when it carries no dated entry at
// all. A reader's client sorts by the entries, so the value only has to be a
// real date and to be the same on every build of one unchanged tree.
const feedEpoch = "2026-01-01"

// feedDateLayout is RFC 822 as RSS spells it, with the four-digit year RFC 1123
// corrected it to. Every entry is dated to a day, so the time is midnight UTC.
const feedDateLayout = "Mon, 02 Jan 2006 00:00:00 +0000"

// feedDate answers one YYYY-MM-DD date in the form a feed states.
//
// A date this cannot parse is a programmer error rather than an operating one:
// every caller validates the date when it reads the source, so reaching here
// with a bad one means a producer skipped that check.
func feedDate(date string) string {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		panic("BUG: site.feedDate: unvalidated date " + date + " reached the feed")
	}
	return parsed.Format(feedDateLayout)
}

// xmlEscaper spells the three characters that cannot appear literally in XML
// text. An apostrophe and a quotation mark can, because nothing this site
// writes puts authored text inside an attribute of a feed.
var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// xmlText answers one authored string as XML character data.
func xmlText(text string) string {
	return xmlEscaper.Replace(text)
}

// feedAlternateLink is the head element that points a reader's client at the
// feed of the section this page belongs to.
func feedAlternateLink(title, href string) string {
	return `        <link rel="alternate" type="application/rss+xml" title="` + title +
		`" href="` + href + `" />` + "\n"
}

// trimIntroWhitespace answers one authored intro as a single line, which is
// what a card, a meta description and a feed entry each show.
func trimIntroWhitespace(intro string) string {
	return strings.Join(strings.Fields(intro), " ")
}

// truncateRunes answers at most the first count runes of text.
//
// The unit is runes rather than bytes because the limit exists to bound what a
// reader sees, and a byte limit would also cut a multi-byte character in half.
func truncateRunes(text string, count int) string {
	runes := []rune(text)
	if len(runes) <= count {
		return text
	}
	return string(runes[:count])
}

// paragraphText answers the inline HTML of one Markdown paragraph, with the
// paragraph element itself removed, for a caller that needs the markup inside
// an element of its own.
func paragraphText(markdown string) (string, error) {
	body, _, err := renderMarkdown([]byte(markdown))
	if err != nil {
		return "", fmt.Errorf("render lead: %w", err)
	}
	body = strings.TrimSpace(body)
	body = strings.TrimPrefix(body, "<p>")
	return strings.TrimSuffix(body, "</p>"), nil
}
