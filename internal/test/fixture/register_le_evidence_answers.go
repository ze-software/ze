package fixture

// register_le_evidence_answers.go registers the le evidence and deployment
// answers scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: ui_fixture_le_evidence_answers.go -- the driver.
func init() {
	Register("ui/le-evidence-answers", uiDriver(leEvidenceAnswers))
}
