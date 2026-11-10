// Fixture: doctor-coverage's three Fix shapes (AC-10): Fix set, Fix the
// empty string literal, and no Fix.
package finding

import "github.com/ze-software/ze/internal/core/diagnostic"

// Findings answers two elided elements of a []diagnostic.Diagnostic literal.
func Findings() []diagnostic.Diagnostic {
	return []diagnostic.Diagnostic{
		{Code: "doctor-finding-fixed", Message: "finding: fix set", Fix: "ze start"}, // finding:fixed
		{Code: "doctor-finding-empty", Message: "finding: fix empty", Fix: ""},       // finding:empty
	}
}

// Missing answers an address-of literal that sets no Fix.
func Missing() *diagnostic.Diagnostic {
	return &diagnostic.Diagnostic{Code: "doctor-finding-missing", Message: "finding: no fix"} // finding:missing
}
