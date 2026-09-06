// Design: docs/architecture/diagnostics/crash-capture.md -- memory-image config refusal
//
// VALIDATES: AC-12 and assumption A-6: `system crash-dump memory-image enabled
//            true` is refused on a build that stages no capture kernel, and the
//            refusal names the architecture.
// PREVENTS:  an arm64 appliance accepting a leaf that commits, appears in
//            `show configuration`, and captures nothing -- so the operator
//            believes the box is covered until the day it faults.

package config

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

func TestMemoryImageRefusedOnNonAmd64(t *testing.T) {
	validator := crashMemoryImageValidator()

	previous := crashCaptureArch
	t.Cleanup(func() { crashCaptureArch = previous })

	cases := []struct {
		name  string
		arch  string
		value any
		want  string
	}{
		{name: "enabled on arm64 is refused", arch: "arm64", value: "true", want: "arm64"},
		{name: "enabled on riscv64 is refused", arch: "riscv64", value: "true", want: "riscv64"},
		{name: "enabled on amd64 is accepted", arch: "amd64", value: "true"},
		{name: "disabled on arm64 is accepted", arch: "arm64", value: "false"},
		{name: "disabled as a bool on arm64 is accepted", arch: "arm64", value: false},
		{name: "a value that is not a boolean is refused", arch: "amd64", value: "maybe", want: "true or false"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			crashCaptureArch = tc.arch
			err := validator.ValidateFn("system crash-dump memory-image enabled", tc.value)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("value %v on %s: %v, want accepted", tc.value, tc.arch, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("value %v on %s was accepted, want a refusal", tc.value, tc.arch)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestCrashMemoryImageValidatorIsRegistered(t *testing.T) {
	// The YANG leaf binds ze:validate "crash-memory-image". A validator that
	// exists and is not registered under that name never runs, and the leaf
	// would accept every value in silence.
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	cv := reg.Get("crash-memory-image")
	if cv == nil {
		t.Fatal("crash-memory-image is not in the validator registry, so the leaf accepts every value")
	}
	if cv.ValidateFn == nil {
		t.Fatal("crash-memory-image has a nil ValidateFn, so it cannot refuse anything")
	}
}
