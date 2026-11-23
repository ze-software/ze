// Design: docs/guide/ipsec.md -- application override control
// Related: application_override.go -- overrideGuardFaultOf, the classifier under test
// Related: rfc4301_application_override_test.go -- TestApplyFailsWhenOverrideGuardCannotAttach, the refusal per class

package engine

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/privilege"
)

// VALIDATES: overrideGuardFaultOf classifies the guard's refusal: EPERM, which
// bpf_prog_load answers a process without CAP_BPF or CAP_NET_ADMIN, is the
// capability; dataplane.ErrNoCgroup2 the missing mount; EINVAL (a verifier before
// Linux 6.10), EACCES (a security module or the verifier) and an error with no errno
// are Unknown.
// METHOD: wrap each answer as attach wraps it, and classify it.
// PREVENTS: a capability fix for an answer no capability changes.
func TestOverrideGuardFaultOfClassifiesTheAnswer(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want overrideGuardFault
	}{
		{"EPERM", fmt.Errorf("load the setsockopt guard: %w", syscall.EPERM), overrideGuardFaultCapability},
		{"no cgroup2", fmt.Errorf("attach: %w", dataplane.ErrNoCgroup2), overrideGuardFaultNoCgroup2},
		{"EINVAL", fmt.Errorf("load the setsockopt guard: %w", syscall.EINVAL), overrideGuardFaultUnknown},
		{"EACCES", fmt.Errorf("load the setsockopt guard: %w", syscall.EACCES), overrideGuardFaultUnknown},
		{"no errno", errors.New("operation not permitted"), overrideGuardFaultUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := overrideGuardFaultOf(tc.err); got != tc.want {
				t.Fatalf("overrideGuardFaultOf(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// VALIDATES: every overrideGuardFault class has its own cause and fix under both
// memberships, the capability fix is privilege.CapabilityRemedy, the refusal fix of
// every class offers the allow statement, and the zero value and an undeclared class
// panic rather than answer.
// METHOD: walk every declared class through overrideGuardCauseOf and
// overrideGuardFixOf, and each producer through the error of its class.
// PREVENTS: a class added without a text, or a refusal that answers for a class Ze
// never produces.
func TestOverrideGuardEveryClassIsAnswered(t *testing.T) {
	for _, member := range []privilege.ServiceMembership{privilege.MembershipZeService, privilege.MembershipOther} {
		t.Run(member.String(), func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(member, nil))
			causes := make(map[string]overrideGuardFault)
			fixes := make(map[string]overrideGuardFault)
			for fault := overrideGuardFaultUnspecified + 1; fault <= overrideGuardFaultUnknown; fault++ {
				cause, fix := overrideGuardCauseOf(fault), overrideGuardFixOf(fault)
				if cause == "" || fix == "" {
					t.Errorf("class %d answers cause %q, fix %q", fault, cause, fix)
				}
				if other, dup := causes[cause]; dup {
					t.Errorf("classes %d and %d have one cause: %q", other, fault, cause)
				}
				if other, dup := fixes[fix]; dup {
					t.Errorf("classes %d and %d have one fix: %q", other, fault, fix)
				}
				causes[cause], fixes[fix] = fault, fault
			}
			perm := fmt.Errorf("load the setsockopt guard: %w", syscall.EPERM)
			if got, want := overrideGuardFix(perm), privilege.CapabilityRemedy("CAP_BPF", "CAP_NET_ADMIN"); got != want {
				t.Errorf("the capability fix = %q, want %q", got, want)
			}
			for _, err := range []error{perm, dataplane.ErrNoCgroup2, errors.New("operation not permitted")} {
				if fix := overrideGuardRefusalFix(err); !strings.Contains(fix, "set `vpn ipsec application-override allow`") {
					t.Errorf("the refusal fix for %v does not offer allow: %q", err, fix)
				}
			}
		})
	}
	for name, call := range map[string]func(){
		"cause unspecified": func() { overrideGuardCauseOf(overrideGuardFaultUnspecified) },
		"cause undeclared":  func() { overrideGuardCauseOf(overrideGuardFaultUnknown + 1) },
		"fix unspecified":   func() { overrideGuardFixOf(overrideGuardFaultUnspecified) },
		"fix undeclared":    func() { overrideGuardFixOf(overrideGuardFaultUnknown + 1) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s answered instead of panicking", name)
				}
			}()
			call()
		})
	}
}
