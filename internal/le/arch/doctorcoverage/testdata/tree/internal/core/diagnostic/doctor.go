package diagnostic

// DoctorCheck is a fixture: the in-tree doctor check declaration, reduced to
// the two fields the covered set reads.
type DoctorCheck struct {
	Name  string
	Codes []string
}

// registered holds the checks RegisterDoctorCheck received.
var registered []DoctorCheck

// RegisterDoctorCheck records one check, as the product registry does.
func RegisterDoctorCheck(check DoctorCheck) {
	registered = append(registered, check)
}

// FixtureListCodes is a code list held in an exported package variable, the
// shape a check whose codes another package shares uses.
var FixtureListCodes = []string{CodeDoctorFixtureList}
