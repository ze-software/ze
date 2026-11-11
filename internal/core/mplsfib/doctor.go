// Design: docs/architecture/mpls/mpls-kernel.md -- a label entry no writer installed
// Related: register.go -- registers the code's explanation
// Related: events.go -- Apply and ApplyReplay, whose refusal the code names
//
// A label source learns from Apply or ApplyReplay that the writer of the
// selected data plane did not install an entry: no writer took it, or the
// writer refused it. The code is declared here, beside the event, because
// every label source and every label writer names the same problem: a
// source's doctor check reports the entries it wants that no writer holds,
// and a writer's refusal log line points at that check.

package mplsfib

import "github.com/ze-software/ze/internal/core/diagnostic"

// CodeDoctorNoWriter names a label entry a label source wants installed, or
// removed, that no writer of the selected data plane carried out.
const CodeDoctorNoWriter = "doctor-fib-mpls-no-writer"

// noWriterCodeMeta is the explanation `ze show diagnostic` prints for
// CodeDoctorNoWriter.
var noWriterCodeMeta = diagnostic.CodeMeta{
	Code:  CodeDoctorNoWriter,
	Title: "A label entry is not installed in the data plane",
	Description: "A label source (LDP, RSVP-TE, OSPF Segment Routing, BGP EPE) asked the writer of the data plane that `interface { backend }` selects " +
		"to install or remove a label-switching entry, and no writer did. Either no writer was running to take it " +
		"(VPP unreachable, no writer loaded for the selected data plane, or a source run as an external plugin with no event bus to reach one), or the writer refused the entry and said why: " +
		"an in-label another source owns, a label stack the data plane cannot impose, a private push table VPP has no form for, " +
		"or the data plane's own error. Labeled traffic for that FEC or LSP is then not forwarded the way the label source " +
		"advertises. This is an ERROR. Each finding names the entry and the writer's cause. A label source applies the entries " +
		"it wants again when a writer opens, so fixing the cause the finding names clears it.",
	Examples: []string{"ze show doctor", "ze show diagnostic doctor-fib-mpls-no-writer"},
}
