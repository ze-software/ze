// A plugin building its registration in a local, filling a FIELD of it, and
// handing the value to the registrar. The field literal is where those
// diagnostic codes enter the registry, so it is the declaration and not a copy
// of one. Shaped after internal/plugins/as112/register.go:143.
package fixture

func init() {
	reg := registry.Registration{Name: "as112"}
	reg.DoctorChecks = []registry.DoctorCheckDef{
		{
			Name:  "as112-tls-cert",
			Codes: []string{"doctor-tls-missing", "doctor-tls-expired"},
		},
	}
	registry.Register(reg)
}
