// Package diagnostic is a doctor-coverage fixture: the diagnostic record,
// reduced to the fields the scan reads, and a finding the package itself
// answers, written unqualified.
package diagnostic

// Diagnostic is a fixture of the diagnostic record.
type Diagnostic struct {
	Code    string
	Message string
	Fix     string
}

// Own answers a finding the diagnostic package builds itself, as its own
// doctor checks do.
func Own() []Diagnostic {
	return []Diagnostic{{Message: "core: own finding", Fix: "ze doctor"}}
}
