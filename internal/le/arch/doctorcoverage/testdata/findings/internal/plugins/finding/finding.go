// Package finding is a doctor-coverage fixture: one diagnostic.Diagnostic
// composite literal of each Fix shape, in each literal shape the tree holds.
package finding

import "github.com/ze-software/ze/internal/core/diagnostic"

// Findings answers two elided elements of a []diagnostic.Diagnostic literal.
func Findings() []diagnostic.Diagnostic {
	return []diagnostic.Diagnostic{
		{
			Code:    "doctor-finding-fixed",
			Message: "finding: fix set",
			Fix:     "ze start",
		},
		{
			Code:    "doctor-finding-empty",
			Message: "finding: fix empty",
			Fix:     "",
		},
	}
}

// Missing answers an address-of literal that sets no Fix.
func Missing() *diagnostic.Diagnostic {
	return &diagnostic.Diagnostic{
		Code:    "doctor-finding-missing",
		Message: "finding: no fix",
	}
}

// Computed answers a literal whose Fix is not a literal: it counts as set.
func Computed(command string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Message: "finding: fix computed", Fix: command}
}

// None answers the zero value beside false: no finding, so no site.
func None() (diagnostic.Diagnostic, bool) {
	return diagnostic.Diagnostic{}, false
}
