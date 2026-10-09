package yang_test

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestResolvedZeroValueIsABug: Go admits the empty literal, new() and nil of
// yang.Resolved in any package (the positive control of probe P-2), and every
// operation on one ends in a named BUG panic rather than answering an empty
// module set.
//
// VALIDATES: AC-4 for T1, the documented contract of the zero value.
// PREVENTS: a zero Resolved answering "no modules", which a caller cannot tell
// from a module set that holds none, and which once let a command tree be
// built from a loader whose resolution had failed.
func TestResolvedZeroValueIsABug(t *testing.T) {
	values := map[string]*yang.Resolved{
		"empty literal": {},
		"new":           new(yang.Resolved),
		"nil":           nil,
	}
	operations := map[string]func(*yang.Resolved){
		"GetModule":        func(r *yang.Resolved) { r.GetModule("ze-types") },
		"GetEntry":         func(r *yang.Resolved) { r.GetEntry("ze-types") },
		"ModuleNames":      func(r *yang.Resolved) { r.ModuleNames() },
		"ConfModuleNames":  func(r *yang.Resolved) { r.ConfModuleNames() },
		"APIModuleNames":   func(r *yang.Resolved) { r.APIModuleNames() },
		"BuildCommandTree": func(r *yang.Resolved) { yang.BuildCommandTree(r) },
		"PathToArgDefs":    func(r *yang.Resolved) { yang.PathToArgDefs(r) },
		"PublishedRPCs":    func(r *yang.Resolved) { _, _ = yang.PublishedRPCs(r) },
	}
	for valueName, value := range values {
		for operationName, operation := range operations {
			message := recoveredPanic(func() { operation(value) })
			if !strings.HasPrefix(message, "BUG: ") {
				t.Errorf("%s on the %s Resolved: panic %q, want a BUG panic", operationName, valueName, message)
			}
		}
	}
}

// recoveredPanic runs operation and answers the text of its panic, or "" when
// it returned.
func recoveredPanic(operation func()) (message string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if text, isText := recovered.(string); isText {
				message = text
				return
			}
			message = "non-string panic"
		}
	}()
	operation()
	return ""
}
