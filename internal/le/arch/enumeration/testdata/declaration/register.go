// A plugin declaring itself and its dependencies to the plugin registry. The
// three names are how two of those edges ENTER the registry, so the literal is
// the declaration this gate protects rather than a copy of one. Shaped after
// internal/component/bgp/plugins/rpki_decorator/register.go, where the value is
// built into a local and registered from it.
package fixture

func init() {
	reg := registry.Registration{
		Name:         "bgp-rpki-decorator",
		Dependencies: []string{"bgp", "bgp-rpki"},
	}
	if err := registry.Register(reg); err != nil {
		return
	}
}

// The other shape: the value is built where it is registered.
func init() {
	registry.RegisterBuiltinFamilies("builtin", []string{"ipv4/unicast", "ipv6/unicast"})
}

// The third shape: a value of a type NOT named for registration, built into a
// local and handed to a registrar two lines later. Shaped after
// internal/component/lg/register.go:24, whose Codes field names two diagnostic
// codes the pki component owns.
func init() {
	check := diagnostic.DoctorCheck{
		Name:  "lg-tls-certificate",
		Codes: []string{"doctor-tls-reference", "doctor-tls-expired"},
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		return
	}
}
