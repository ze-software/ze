// Design: docs/architecture/fleet-config.md -- the hub's session editor refuses a client no serving block declares

package fixture

// The one-hub driver for the session editor's declared-client guard. Its body
// is misc_fixture_managed_hub_editor.go.
func init() {
	Register("managed/fleet-hub-editor-declared", managedHubEditorDeclaredDriver)
}
