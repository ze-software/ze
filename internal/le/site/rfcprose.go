// Design: website/AI.md -- the authored ledger prose, rendered rather than rewritten
// Overview: rfcledger.go -- the ledger entry whose authored text this cleans
package site

import (
	"regexp"
	"strings"
)

// rfcAuthoringNote is an HTML comment and the blanks before it. The author of a
// summary writes one for the next author, and Markdown never shows it.
var rfcAuthoringNote = regexp.MustCompile(`(?s)[ \t]*<!--.*?-->`)

// rfcAuthoringNotesRemoved answers authored text as a reader is shown it: every
// HTML comment removed, and every other character kept.
//
// A comment nothing closes is left as text, because the regexp needs the
// closing "-->" and a sentence is never cut at a guess.
func rfcAuthoringNotesRemoved(text string) string {
	return strings.TrimSpace(rfcAuthoringNote.ReplaceAllString(text, ""))
}
