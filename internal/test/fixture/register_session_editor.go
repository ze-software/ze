// Design: docs/functional-tests.md -- compiled fixtures a .ci drives

package fixture

// The session editor driver of spec-session-editor-file-mode-parity. Its body
// and its script grammar are plugin_fixture_session_editor.go.
func init() {
	Register("plugin/session-editor", sessionEditorDriver)
}
