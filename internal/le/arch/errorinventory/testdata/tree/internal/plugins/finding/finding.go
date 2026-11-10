// Fixture: the doctor site, a finding that names its fix.
package finding

import "github.com/ze-software/ze/internal/core/diagnostic"

// Finding answers the fixture's one doctor finding.
func Finding() diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Code: "fixture-down", Message: "fixture: daemon down", Fix: "ze start"}
}
