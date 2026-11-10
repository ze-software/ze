// Package aliased is a doctor-coverage fixture: a finding written through an
// import alias of the diagnostic package.
package aliased

import diag "github.com/ze-software/ze/internal/core/diagnostic"

// Finding answers a literal that sets no Fix.
func Finding() diag.Diagnostic {
	return diag.Diagnostic{Message: "aliased: no fix"}
}
