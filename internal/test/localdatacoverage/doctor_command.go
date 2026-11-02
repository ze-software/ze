package localdatacoverage

import "io"

// runDoctorCommand covers `doctor` through its kernel-capabilities mode, the
// one answer that reads no store and no configuration, so the walk does not
// depend on what the private workspace holds. The check proves the answer is
// the kernel report (a ready verdict and a row per enrolled capability), so
// an empty answer or the readiness report in its place is refused.
func runDoctorCommand(output io.Writer) error {
	payload, err := localJSON("doctor kernel-capabilities | json compact", "doctor", output)
	if err != nil {
		return err
	}
	document, err := exactObject(payload, "doctor kernel-capabilities", "ready", "capabilities")
	if err != nil {
		return err
	}
	if _, ok := document["ready"].(bool); !ok {
		return require(false, "doctor kernel-capabilities ready is %#v, want a boolean", document["ready"])
	}
	capabilities, err := rows(document, "capabilities")
	if err != nil {
		return err
	}
	return require(len(capabilities) > 0, "doctor kernel-capabilities answered no capability row")
}
