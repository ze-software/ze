// Package unlinked is a fixture: a component no composition root imports. Its
// doctor check still covers the code it declares, because the covered set is
// read from the source, not from the registries linked into le.
package unlinked

import "github.com/ze-software/ze/internal/core/diagnostic"

// unlinkedDoctorCheck declares the covered code through another package's
// exported constant.
var unlinkedDoctorCheck = diagnostic.DoctorCheck{
	Name:  "fixture-check",
	Codes: []string{diagnostic.CodeDoctorFixtureCovered},
}
