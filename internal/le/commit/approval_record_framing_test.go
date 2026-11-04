// Design: docs/architecture/testing/verify-freshness-scope.md -- verdict data cannot hide later person-act obligations.
package commit

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestNativeRFCVerdictCannotHideALaterLinuxLimitObligation records an ordinary
// judge verdict, prepares and executes the real native publication, and reads
// its actual Git message before deriving push debt. A plain reason is the
// accepting control; an embedded record separator must not erase the later
// automatically emitted Linux-limit obligation or let the push proceed.
func TestNativeRFCVerdictCannotHideALaterLinuxLimitObligation(t *testing.T) {
	for _, control := range []struct {
		name   string
		reason string
	}{
		{"plain", "judge: checked the exact assertion"},
		{"record separator", "judge: checked the exact assertion\x1e transport suffix"},
	} {
		t.Run(control.name, func(t *testing.T) {
			root := newOwedRepository(t)
			installNativeReviewChecker(t, root)
			commitOwed(t, root, "original tagged carrier", map[string]string{owedTaggedPath: taggedThingText("check(1)")}, nil)
			writeCommitFixture(t, root, owedTaggedPath, taggedThingText("check(2)"))
			writeCommitFixture(t, root, linuxLimitSummary, linuxLimitRow+linuxLimitAnnotation+"\n")
			if _, err := rfc.Approve(root, approvalSession, owedTaggedUnit, control.reason); err != nil {
				t.Fatalf("ordinary judge reason did not reach the native writer: %v", err)
			}
			prepared, err := Create(root, &Options{
				Session: approvalSession, Subject: "publish verdict and Linux limit",
				Files:  []string{owedTaggedPath, linuxLimitSummary},
				NoTest: "the native fixture checks publication and derived obligations",
			})
			if err != nil {
				t.Fatalf("native preparation did not reach the publication boundary: %v", err)
			}
			want := trailerLinuxLimitOwed + linuxLimitRID + ": LL-EXAMPLE"
			if !slices.Contains(prepared.Trailers, want) {
				t.Fatalf("native producer did not emit the required obligation: %v", prepared.Trailers)
			}
			if output, err := runScript(t, root, prepared.Script); err != nil {
				t.Fatalf("actual native publication failed before debt derivation: %v: %s", err, output)
			}
			carrier := revParse(t, root, "HEAD")
			body := runCommitGitOutput(t, root, "log", "-1", "--format=%B")
			if !strings.Contains(body, rfc.ApprovalTrailerLine(owedTaggedUnit, control.reason)) {
				t.Fatalf("publication did not preserve the exact ordinary judge reason: %q", body)
			}
			if !strings.Contains(body, "\n"+want+"\n") {
				t.Fatalf("the actual Git carrier lost the emitted Linux-limit trailer: %q", body)
			}
			debt := deriveOwed(t, root, "")
			held := false
			for _, item := range debt.Items {
				if item.Commit == carrier && item.Kind == owedItemLinuxLimit && item.Subject == linuxLimitRID {
					held = true
				}
			}
			if control.name == "plain" {
				assertOneItem(t, debt, carrier, owedItemLinuxLimit, linuxLimitRID)
				if len(debt.Refusals) != 0 {
					t.Fatalf("ordinary reason control was refused: %v", debt.Refusals)
				}
			}
			if !held && !owedSaid(debt.Refusals, "malformed Git person-act record") {
				t.Fatalf("ordinary judge reason hid the later emitted obligation without a framing refusal: items=%+v refusals=%q", debt.Items, debt.Refusals)
			}
			if err := debt.pushRefusal(); err == nil {
				t.Fatal("a push was admitted despite its emitted Linux-limit obligation")
			}
		})
	}
}
