// A doctor runner in the shape of the real one: two checks reached by writing
// their names out, one reached through the registry.
package doctor

func runChecks() []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	diags = append(diags, checkDisk()...)
	diags = append(diags, checkClock()...)
	diags = append(diags, runDoctorChecks(phasePostConfig)...)
	return diags
}
