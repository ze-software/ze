package fixture

// register_le_derived_artifact_lifecycle.go registers the derived-artifact
// lifecycle scenario.
//
// The registration is here rather than beside the driver because the native
// pretool-writeedit check refuses Register inside init() in any file whose name
// does not start with "register" (internal/le/hookruntime/writeedit.go).
//
// Related: misc_fixture_runner_derived.go -- the driver.
func init() {
	Register("runner/le-derived-artifact-lifecycle", leDerivedArtifactLifecycleDriver)
}
