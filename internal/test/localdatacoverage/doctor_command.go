// Design: docs/architecture/doctor-and-health-checks.md -- Linux-only kernel enrollment.
// Related: doctor_command_test.go -- platform and report-shape acceptance controls.

package localdatacoverage

import (
	"fmt"
	"io"
	"runtime"
)

// runDoctorCommand covers the configuration-free kernel report through the
// actual CLI pipe. Linux requires enrolled rows; an off-Linux build must
// report an empty enrollment as not ready, never as a successful kernel probe.
func runDoctorCommand(output io.Writer) error {
	payload, err := localJSON("doctor kernel-capabilities | json compact", "doctor", output)
	if err != nil {
		return err
	}
	return validateDoctorKernelReport(payload, runtime.GOOS == "linux")
}

// validateDoctorKernelReport checks delivered data independently of the report
// producer. linux is supplied explicitly so both platform contracts are tested.
func validateDoctorKernelReport(payload any, linux bool) error {
	document, err := exactObject(payload, "doctor kernel-capabilities", "ready", "capabilities")
	if err != nil {
		return err
	}
	ready, ok := document["ready"].(bool)
	if !ok {
		return fmt.Errorf("doctor kernel-capabilities ready is %#v, want a boolean", document["ready"])
	}
	capabilities, err := rows(document, "capabilities")
	if err != nil {
		return err
	}
	if len(capabilities) == 0 {
		if linux {
			return fmt.Errorf("doctor kernel-capabilities answered no capability row on Linux")
		}
		return require(!ready, "doctor kernel-capabilities reported an empty enrollment as ready")
	}
	expectedReady := true
	for index, value := range capabilities {
		row, err := rowObject(value)
		if err != nil {
			return err
		}
		subsystem, ok := row["subsystem"].(string)
		if !ok {
			return fmt.Errorf("kernel capability row %d has no subsystem string: %#v", index, row)
		}
		if subsystem == "" {
			return fmt.Errorf("kernel capability row %d has an empty subsystem", index)
		}
		kernel, ok := row["kernel"].(string)
		if !ok {
			return fmt.Errorf("kernel capability row %d has no kernel string: %#v", index, row)
		}
		if kernel == "" {
			return fmt.Errorf("kernel capability row %d has an empty kernel symbol", index)
		}
		state, ok := row["state"].(string)
		if !ok {
			return fmt.Errorf("kernel capability row %d has no state string: %#v", index, row)
		}
		// These are the report's wire values. ProbeAll turns an unspecified probe
		// into unknown; no unspecified or unrecognized state can be delivered.
		switch state {
		case "present":
		case "absent", "unknown", "denied":
			expectedReady = false
		default:
			return fmt.Errorf("kernel capability row %d has invalid state %q", index, state)
		}
	}
	return require(ready == expectedReady, "doctor kernel-capabilities ready=%t disagrees with its rows, want %t", ready, expectedReady)
}
