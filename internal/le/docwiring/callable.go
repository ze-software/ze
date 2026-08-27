// Design: docs/architecture/core-design.md -- linked documentation gate owners
//
// Package doccheck calls these owners directly. Keeping the callables here means
// the standalone actions and the changed-file router execute the same code.

package docwiring

// Verify runs the complete documentation gate over root.
func Verify(root string) (any, int) {
	return answerDocVerify(root)
}

// TemplOutput checks the generated templ outputs under root.
func TemplOutput(root string) (any, int) {
	return answerTemplOutput(root)
}

// DesignReferences reports broken Go Design references under root.
func DesignReferences(root string) ([]string, error) {
	return designReferenceFindings(root)
}
