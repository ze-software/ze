package doctor_test

import (
	// Trigger plugin init() registrations needed by the doctor check tests.
	//
	// It sits in the EXTERNAL test package on purpose. plugin/all now
	// blank-imports internal/component/doctor, because the generated
	// composition root names every package that registers a command, so the
	// same import from the internal test package is a cycle in test. Both
	// files compile into one test binary, so the registrations still fire.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)
