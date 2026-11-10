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

// TestRegisterDoctorCheckAcceptsForeignNamespace registers a check that
// declares the codes another surface already owns.
//
// checkSemantics (internal/component/config/doctor.go) returns
// config.ValidateSemantics verbatim, and that function emits config-mcp-invalid
// and its three siblings. A doctor- alias for them would be a second code for
// one fact, so the registry takes the codes the check really emits.
//
// VALIDATES: a doctor check declares the code its own output carries, whatever
// namespace that code belongs to.
// PREVENTS: a check that cannot be registered at all, leaving it reachable only
// by a hand-written call in the runner.
func TestRegisterDoctorCheckAcceptsForeignNamespace(t *testing.T) {
	ResetDoctorChecksForTest()
	defer ResetDoctorChecksForTest()

	check := DoctorCheck{
		Name:         "semantics",
		Phase:        DoctorPhasePostConfig,
		Order:        10,
		Component:    "test",
		Dependencies: []string{"config"},
		Platforms:    []string{DoctorPlatformAny},
		Codes:        []string{"config-mcp-invalid", "config-gnmi-invalid"},
		Check:        func(DoctorCheckContext) []Diagnostic { return nil },
	}
	if err := RegisterDoctorCheck(check); err != nil {
		t.Fatalf("register a check declaring config- codes: %v", err)
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
		{"malformed code", DoctorCheck{
			Name: "malformed-code", Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"Doctor Code"}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}},
		{"empty code", DoctorCheck{
			Name: "empty-code", Phase: DoctorPhasePreConfig, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{""}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
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

// TestRegisterDoctorCheckRuntimePhase registers two runtime checks and lists
// them by phase.
//
// VALIDATES: the runtime phase registers and sorts like the other phases.
// PREVENTS: a runtime check refused at registration, or the phase-rank sort
// panicking on a phase it does not know.
func TestRegisterDoctorCheckRuntimePhase(t *testing.T) {
	ResetDoctorChecksForTest()
	defer ResetDoctorChecksForTest()

	for _, name := range []string{"runtime-b", "runtime-a"} {
		check := DoctorCheck{
			Name: name, Phase: DoctorPhaseRuntime, Component: "test",
			Dependencies: []string{"x"}, Platforms: []string{DoctorPlatformAny},
			Codes: []string{"doctor-" + name}, Check: func(DoctorCheckContext) []Diagnostic { return nil },
		}
		if err := RegisterDoctorCheck(check); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	checks := DoctorChecksForPhase(DoctorPhaseRuntime)
	if len(checks) != 2 {
		t.Fatalf("runtime checks = %d, want 2", len(checks))
	}
	if checks[0].Name != "runtime-a" {
		t.Errorf("first runtime check = %q, want runtime-a", checks[0].Name)
	}
	if got := DoctorChecksForPhase(DoctorPhasePostConfig); len(got) != 0 {
		t.Errorf("post-config checks = %d, want 0: a runtime check leaked into another phase", len(got))
	}
}
