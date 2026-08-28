// Design: docs/architecture/core-design.md -- le's native development gates
// Related: anchors.go -- document-owner derivation

package speccitation

import "github.com/ze-software/ze/internal/core/textbuf"

// AnchorFinding is one omitted document and the source files that tie it to the
// spec.
type AnchorFinding struct {
	Document string   `json:"document"`
	Sources  []string `json:"sources"`
}

// AnchorReport is one spec document-owner audit. Owners change the verdict.
// Mentions are advisory.
type AnchorReport struct {
	Spec     string          `json:"spec"`
	Files    []string        `json:"files"`
	Owners   []AnchorFinding `json:"owners"`
	Mentions []AnchorFinding `json:"mentions"`
}

// Text renders the producer-compatible audit report.
func (r AnchorReport) Text() string {
	var tb textbuf.Buffer
	if len(r.Mentions) > 0 {
		tb.Str("note: ").Int(int64(len(r.Mentions))).Str(" document(s) mention this spec's code and are not named:\n")
		anchorFindingsText(&tb, r.Mentions, "mentioned by")
	}
	if len(r.Owners) == 0 {
		return tb.String()
	}
	if len(r.Mentions) > 0 {
		tb.Byte('\n')
	}
	tb.Str(r.Spec).Str(": ").Int(int64(len(r.Owners))).Str(" design document(s) DECLARED by this spec's own code are never named in it:\n")
	anchorFindingsText(&tb, r.Owners, "declared by")
	tb.Str("\n  Each file above carries `// Design: <doc>` in its header, naming that\n").
		Str("  document as its design. Changing the file without naming the doc is how a\n").
		Str("  design change ships with its design unwritten.\n").
		Str("  Name each one: list it under `## Files to Modify`, or record in the\n").
		Str("  Documentation Update Checklist why it is unaffected.\n")
	return tb.String()
}

func anchorFindingsText(tb *textbuf.Buffer, findings []AnchorFinding, edge string) {
	for _, finding := range findings {
		tb.Str("  ").Str(finding.Document).Str("\n      ").Str(edge).Str(": ")
		for index, source := range finding.Sources {
			if index > 0 {
				tb.Str(", ")
			}
			tb.Str(source)
		}
		tb.Byte('\n')
	}
}
