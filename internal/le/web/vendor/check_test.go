// Related: check.go -- licenceCheck, the refusal these tests drive
// Related: vendorweb.go -- licenceExempt, licenceSuffixes

package webvendor

import (
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: a vendored web directory that holds no licence file is refused,
// by name, with the fix in the message (spec-thanks-third-party-credits AC-14).
// PREVENTS: a third-party asset reaching a consumer, and so a shipped binary,
// with no licence text beside it. The binary then redistributes the asset
// without the notice its licence asks to travel with it, and no gate says so.
//
// The fixture is the clean tree with htmx's licence taken away from the source
// and from every consumer, so the only problem left is the missing licence: a
// MISSING or DRIFT row cannot be what made the check refuse.
func TestCheckRefusesAVendorDirectoryWithoutALicence(t *testing.T) {
	const licence = "htmx.LICENSE"
	unlicensed := filepath.Join(vendorDir, "htmx")

	root := fixture(t)
	removePath(t, root, filepath.Join(unlicensed, licence))
	for _, consumer := range consumers {
		removePath(t, root, filepath.Join(consumer, licence))
	}

	report, err := Check(root, false)
	if err == nil {
		t.Fatalf("Check accepted a vendored directory with no licence file\n%s", report.Text())
	}

	found := problemsOf(report, ProblemUnlicensed)
	if len(found) != 1 || found[0].File != unlicensed {
		t.Fatalf("Check reported %v, want one %s for %s", report.Problems, ProblemUnlicensed, unlicensed)
	}
	if len(report.Problems) != 1 {
		t.Errorf("Check reported %v, want the missing licence alone", report.Problems)
	}
	if !strings.Contains(err.Error(), unlicensed) {
		t.Errorf("Check refused with %q, which does not name %s", err, unlicensed)
	}
	if !strings.Contains(err.Error(), "./le web vendor update") {
		t.Errorf("Check refused with %q, which does not name the command that finishes the fix", err)
	}
	if !strings.Contains(report.Text(), ProblemUnlicensed+": "+unlicensed) {
		t.Errorf("the rendering does not name the unlicensed directory:\n%s", report.Text())
	}
}

// VALIDATES: third_party/web/ze/ passes without a licence file because the
// code declares it exempt, and for no other reason.
// PREVENTS: an exemption by accident. Ze's own logo carries no third-party
// licence, and the check must say why it accepts that; with the declaration
// taken away the same tree is refused, which proves the declaration is what
// lets it pass.
func TestCheckExemptsZesOwnLogoByDeclarationOnly(t *testing.T) {
	const own = "ze"

	reason, declared := licenceExempt[own]
	if !declared || reason == "" {
		t.Fatalf("licenceExempt does not declare %s/ with a reason: %q", own, reason)
	}

	root := fixture(t)
	if _, err := Check(root, false); err != nil {
		t.Fatalf("Check refused the clean fixture, whose %s/ holds no licence file: %v", own, err)
	}

	delete(licenceExempt, own)
	t.Cleanup(func() { licenceExempt[own] = reason })

	report, err := Check(root, false)
	if err == nil {
		t.Fatalf("Check accepted %s/ with no licence file and no exemption\n%s", own, report.Text())
	}
	found := problemsOf(report, ProblemUnlicensed)
	if len(found) != 1 || found[0].File != filepath.Join(vendorDir, own) {
		t.Errorf("Check reported %v, want one %s for %s", report.Problems, ProblemUnlicensed, filepath.Join(vendorDir, own))
	}
}
