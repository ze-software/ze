// A plugin declaring its own diagnostic codes in const form. The const IS where
// each code enters the registry, because CodeMeta carries it there, so asking
// the author to derive it from the registry asks them to derive it from itself.
// Shaped after internal/plugins/ospf/doctor.go:23 and its registration in
// internal/plugins/ospf/register.go:156.
package fixture

const (
	codeTLSExpired = "doctor-tls-expired"
	codeTLSMissing = "doctor-tls-missing"
)
