package diagnostic

import "testing"

func TestRegisterDoctorCheck(t *testing.T) {
	ResetDoctorChecksForTest()
	defer ResetDoctorChecksForTest()

	check := DoctorCheck{
		Name:         "test-check",
		Phase:        DoctorPhasePostConfig,
		Order:        100,
		Component:    "test",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{DoctorPlatformAny},
		Codes:        []string{"doctor-test-ok"},
		Check:        func(DoctorCheckContext) []Diagnostic { return nil },
	}
	if err := RegisterDoctorCheck(check); err != nil {
		t.Fatalf("register: %v", err)
	}

	names := DoctorCheckNames()
	if len(names) != 1 || names[0] != "test-check" {
		t.Errorf("names = %v, want [test-check]", names)
	}

	checks := DoctorChecksForPhase(DoctorPhasePostConfig)
	if len(checks) != 1 {
		t.Fatalf("post-config checks = %d, want 1", len(checks))
	}
	if checks[0].Name != "test-check" {
		t.Errorf("check name = %q, want test-check", checks[0].Name)
	}
}

func TestRegisterDoctorCheckRejectsDuplicate(t *testing.T) {
	ResetDoctorChecksForTest()
	defer ResetDoctorChecksForTest()

	check := DoctorCheck{
		Name:         "dup-check",
		Phase:        DoctorPhasePreConfig,
		Order:        1,
		Component:    "test",
		Dependencies: []string{"file"},
		Platforms:    []string{DoctorPlatformAny},
		Codes:        []string{"doctor-test-dup"},
		Check:        func(DoctorCheckContext) []Diagnostic { return nil },
	}
	if err := RegisterDoctorCheck(check); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := RegisterDoctorCheck(check); err == nil {
		t.Error("second register should fail for duplicate name")
	}
}

func TestRegisterDoctorCheckValidation(t *testing.T) {
	ResetDoctorChecksForTest()
	defer ResetDoctorChecksForTest()

	tests := []struct {
		name  string
		check DoctorCheck
	}{
		{"empty name", DoctorCheck{
			Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"doctor-x"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"bad phase", DoctorCheck{
			Name: "bad-phase", Phase: "invalid", Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"doctor-x"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"no deps", DoctorCheck{
			Name: "no-deps", Phase: DoctorPhasePreConfig, Component: "test",
			Platforms: []string{DoctorPlatformAny},
			Codes:     []string{"doctor-x"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"no codes", DoctorCheck{
			Name: "no-codes", Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"bad code prefix", DoctorCheck{
			Name: "bad-code", Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"not-doctor"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"bad platform", DoctorCheck{
			Name: "bad-platform", Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{"system"},
			Codes: []string{"doctor-x"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"duplicate platform", DoctorCheck{
			Name: "duplicate-platform", Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny, DoctorPlatformAny},
			Codes: []string{"doctor-x"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"duplicate code", DoctorCheck{
			Name: "duplicate-code", Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"doctor-x", "doctor-x"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RegisterDoctorCheck(tt.check); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestDoctorChecksForPhaseOrdering(t *testing.T) {
	ResetDoctorChecksForTest()
	defer ResetDoctorChecksForTest()

	for _, c := range []DoctorCheck{
		{Name: "z-last", Phase: DoctorPhasePostConfig, Order: 200, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"doctor-z"}, Check: func(DoctorCheckContext) []Diagnostic { return nil }},
		{Name: "a-first", Phase: DoctorPhasePostConfig, Order: 100, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"doctor-a"}, Check: func(DoctorCheckContext) []Diagnostic { return nil }},
	} {
		if err := RegisterDoctorCheck(c); err != nil {
			t.Fatalf("register %s: %v", c.Name, err)
		}
	}

	checks := DoctorChecksForPhase(DoctorPhasePostConfig)
	if len(checks) != 2 {
		t.Fatalf("got %d checks, want 2", len(checks))
	}
	if checks[0].Name != "a-first" {
		t.Errorf("first check = %q, want a-first (lower order)", checks[0].Name)
	}
}
