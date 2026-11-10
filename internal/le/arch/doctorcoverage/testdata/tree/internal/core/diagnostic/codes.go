// Package diagnostic is a fixture: the doctor codes the fixture sites name.
package diagnostic

// The fixture codes. Covered is declared by the doctor check of
// internal/component/unlinked; List by a check whose code list is
// FixtureListCodes; Orphan is a code no doctor check declares.
const (
	CodeDoctorFixtureCovered = "doctor-fixture-covered"
	CodeDoctorFixtureList    = "doctor-fixture-list"
	CodeDoctorFixtureOrphan  = "doctor-fixture-orphan"
)
