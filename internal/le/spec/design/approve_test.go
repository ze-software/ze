// VALIDATES: `le spec design approve` refuses without an interactive terminal
// through the owner-terminal gate, and leaves tmp/spec-design/ as it was; from
// a terminal it refuses a spec that fails the check or lacks a current judge
// approval, and otherwise writes the digest-bound record the one lookup reads
// (spec-design-needs-owner-approval, AC-17 to
// AC-19).
// PREVENTS: an agent recording Thomas's approval of a design, and an
// approval of a design nobody judged.
package specdesign_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// TestApproveRefusesWithoutTerminal is the wiring row "`./le spec design
// approve` without a terminal". Method: a test process has no terminal on
// stdin and stdout; the approve action exits 2 with human.RequireAuthorization's
// refusal and writes nothing under tmp/spec-design/.
func TestApproveRefusesWithoutTerminal(t *testing.T) {
	root := widgetRoot(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "approve-session")

	var code int
	stderr := captureStderr(t, func() {
		_, code = specdesign.Answer([]string{"approve", "spec", widgetSpec, "words", "approved"})
	})
	if code != 2 {
		t.Fatalf("approve without a terminal answered %d, want 2:\n%s", code, stderr)
	}
	requireContains(t, "the approve refusal", stderr,
		"owner authorization requires interactive stdin and stdout", "spec design approve")
	requireAbsent(t, root, "tmp/spec-design")
}

// approvedRecord is the widget spec's approval record.
const approvedRecord = "tmp/spec-design/approved/spec-widget.md"

// departingSpecText is the complete widget spec whose decision row departs,
// with the departure presented: a spec the approval records a departure of.
var departingSpecText = strings.Replace(strings.Replace(completeSpecText,
	"| agrees | The surface renders |", "| departs | The widget renders text no more |", 1),
	"### Documents departed from\n\nnone\n",
	"### Documents departed from\n\n`"+widgetPage+"`: before, the widget renders; after, the surface does.\n", 1)

// runApprove runs the approve action over the widget spec written as text.
func runApprove(t *testing.T, root, text, words string) (string, int) {
	t.Helper()
	writeTree(t, root, map[string]string{widgetSpec: text})
	var answer any
	var code int
	stderr := captureStderr(t, func() {
		answer, code = specdesign.Answer([]string{"approve", "spec", widgetSpec, "words", words})
	})
	if answer == nil {
		return stderr, code
	}
	return answerText(t, answer) + stderr, code
}

// TestApproveRefusesIncompleteOrUnjudgedSpec is AC-18. Method: from a real
// terminal, a spec that fails the check, one no judge judged, one the judge
// refused, and one the judge judged in another text each exit 2 naming the
// failing line or the judge step, and none writes an approval record.
func TestApproveRefusesIncompleteOrUnjudgedSpec(t *testing.T) {
	root := widgetRoot(t)
	humantest.Streams(t, "terminal")
	cases := []struct {
		name, text, judged, verdict, want string
	}{
		{"fails the check", widgetSpecText, widgetSpecText, "approve", "## " + "Design presentation"},
		{"no judge record", completeSpecText, "", "", "no judge has judged this design"},
		{"the judge refused", completeSpecText, completeSpecText, "refuse", "the judge refused this design"},
		{"judged another text", completeSpecText, departingSpecText, "approve", "the judge judged another text"},
	}
	for _, tc := range cases {
		if err := os.RemoveAll(filepath.Join(root, "tmp", "spec-design")); err != nil {
			t.Fatal(err)
		}
		if tc.judged != "" {
			writeJudgeRecord(t, root, tc.judged, tc.verdict)
		}
		output, code := runApprove(t, root, tc.text, "approved")
		if code != 2 {
			t.Errorf("%s: approve answered %d, want 2:\n%s", tc.name, code, output)
		}
		requireContains(t, tc.name, output, "is not ready for approval", tc.want)
		requireAbsent(t, root, approvedRecord)
	}
}

// TestApproveWritesDigestBoundRecord is AC-19 and the survivors of AC-18.
// Method: from a real terminal, approve the judged departing widget spec with
// Status in-progress; the record carries the spec, the design digest, the
// date, the machine, the words, the judge's digest and one Departure line,
// and the one lookup and the departure lookup read it back. A second approval
// replaces this spec's record and leaves another spec's record as it was.
func TestApproveWritesDigestBoundRecord(t *testing.T) {
	root := widgetRoot(t)
	gitInit(t, root)
	humantest.Streams(t, "terminal")
	text := strings.Replace(departingSpecText, "| Status | design |", "| Status | in-progress |", 1)
	writeJudgeRecord(t, root, text, "approve")
	other := "tmp/spec-design/approved/spec-other.md"
	otherBody := "Spec: spec-other.md\nDigest: 0\nDate: 2026-11-01\nMachine: m\nOwner words: w\nJudge: 0 2026-11-01 s\n"
	writeTree(t, root, map[string]string{other: otherBody})

	output, code := runApprove(t, root, text, "first words")
	if code != 0 {
		t.Fatalf("approve answered %d, want 0:\n%s", code, output)
	}
	output, code = runApprove(t, root, text, "approved, the surface renders")
	if code != 0 {
		t.Fatalf("the second approve answered %d, want 0:\n%s", code, output)
	}
	requireContains(t, "the approve answer", output, `"spec":"spec-widget.md"`, `"departs":1`, `"agrees":0`)
	digest := specdesign.DesignDigest(text)
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	record := readFixture(t, root, approvedRecord)
	requireContains(t, "the approval record", record,
		"Spec: spec-widget.md\n", "Digest: "+digest+"\n", "Date: "+time.Now().Format(time.DateOnly)+"\n",
		"Machine: "+hostname+"\n", "Owner words: approved, the surface renders\n", "Judge: "+digest+" 2026-11-20 every verdict holds\n",
		"Departure: "+widgetPage+"\tDesign decisions\t\""+widgetDecision+"\"\n")
	if strings.Contains(record, "first words") {
		t.Errorf("the second approval did not replace the first:\n%s", record)
	}
	if got := readFixture(t, root, other); got != otherBody {
		t.Errorf("approving spec-widget changed spec-other's record:\n%s", got)
	}

	approval, found, err := specdesign.Approved(root, "spec-widget", text)
	if err != nil || !found {
		t.Fatalf("Approved over the approved text: found %v, err %v", found, err)
	}
	if approval.Words != "approved, the surface renders" || len(approval.Departures) != 1 {
		t.Errorf("Approved answered %+v", approval)
	}
	changed := strings.Replace(text, "Render the widget in the surface.", "Render the widget twice.", 1)
	if _, found, err := specdesign.Approved(root, "spec-widget", changed); err != nil || found {
		t.Errorf("Approved over a changed Task: found %v, err %v; want not found", found, err)
	}
	row := docconsistency.DecisionRow{Decision: widgetDecision}
	if approved, err := specdesign.DepartureApproved(root, widgetPage, row); err != nil || !approved {
		t.Errorf("DepartureApproved for the approved departure: %v, %v", approved, err)
	}
	if approved, err := specdesign.DepartureApproved(root, "docs/architecture/other.md", row); err != nil || approved {
		t.Errorf("DepartureApproved for another page: %v, %v; want false", approved, err)
	}
}

// readFixture answers the text of path under root.
func readFixture(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
