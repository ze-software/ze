// Design: docs/guide/configuration.md -- System Host Name: syntax and validation
// Related: hostname.go -- ValidateHostName, validateSystemHostTree, hostnameMismatch

package system

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// hostTree is a config whose system host is value.
func hostTree(value string) *config.Tree {
	tree := config.NewTree()
	sys := config.NewTree()
	sys.Set("host", value)
	tree.SetContainer("system", sys)
	return tree
}

// TestHostnameValidation walks the host name rule and its boundaries.
// VALIDATES: AC-9 -- a value over 64 bytes, a label over 63, an empty label, a
// byte outside letters, digits and hyphens, or a hyphen at a label edge is
// refused, naming the value, the rule and the fix; the last valid lengths and a
// digit first (RFC 1123 Section 2.1) are accepted.
// PREVENTS: sethostname answering EINVAL at run time for a committed value.
func TestHostnameValidation(t *testing.T) {
	label63 := strings.Repeat("a", 63)
	name64 := strings.Repeat("a", 31) + "." + strings.Repeat("b", 32)
	accepted := []string{"cpe-lab", "CPE-Lab2", "1router", "123", "a", "cpe.example.net", label63, name64}
	for _, name := range accepted {
		if len(name) > 64 {
			t.Fatalf("test fixture %q is %d bytes", name, len(name))
		}
		if err := ValidateHostName(name); err != nil {
			t.Errorf("ValidateHostName(%q) = %v, want accepted", name, err)
		}
		if err := validateSystemHostTree(hostTree(name)); err != nil {
			t.Errorf("tree validator refused %q: %v", name, err)
		}
	}

	refused := []struct{ name, rule string }{
		{"", "empty"},
		{strings.Repeat("a", 64), "at most 63"},
		{name64 + "c", "at most 64"},
		{"cpe_lab", "letters, digits and hyphens"},
		{"cpe lab", "letters, digits and hyphens"},
		{"-cpe", "starting with a hyphen"},
		{"cpe-", "ending with a hyphen"},
		{"cpe..lab", "empty label"},
		{"cpe.", "empty label"},
		{".cpe", "empty label"},
		{"cp\u00e9", "letters, digits and hyphens"},
	}
	for _, tt := range refused {
		name, rule := tt.name, tt.rule
		err := ValidateHostName(name)
		if err == nil {
			t.Errorf("ValidateHostName(%q) accepted, want refused (%s)", name, rule)
			continue
		}
		if !strings.Contains(err.Error(), rule) {
			t.Errorf("ValidateHostName(%q) = %q, want it to say %q", name, err, rule)
		}
		if name == "" {
			continue // An empty leaf value is the YANG layer's to refuse.
		}
		treeErr := validateSystemHostTree(hostTree(name))
		if treeErr == nil {
			t.Errorf("tree validator accepted %q", name)
			continue
		}
		for _, want := range []string{"system host", name, "fix:"} {
			if !strings.Contains(treeErr.Error(), want) {
				t.Errorf("tree validator error %q does not carry %q", treeErr, want)
			}
		}
	}
}

// TestHostnameValidationExpandsEnv judges the value after $ENV_VAR expansion.
// VALIDATES: AC-9 -- an expanded value that is not a host name is refused and
// the error shows both forms; an unset variable is left to the daemon.
// PREVENTS: a valid-looking reference hiding an invalid name.
func TestHostnameValidationExpandsEnv(t *testing.T) {
	t.Setenv("ZE_TEST_SYSTEM_HOST", "bad_name")
	err := validateSystemHostTree(hostTree("$ZE_TEST_SYSTEM_HOST"))
	if err == nil || !strings.Contains(err.Error(), "bad_name (from $ZE_TEST_SYSTEM_HOST)") {
		t.Fatalf("expanded invalid value: err = %v", err)
	}
	t.Setenv("ZE_TEST_SYSTEM_HOST", "cpe-lab")
	if err := validateSystemHostTree(hostTree("$ZE_TEST_SYSTEM_HOST")); err != nil {
		t.Fatalf("expanded valid value refused: %v", err)
	}
	if err := validateSystemHostTree(hostTree("$ZE_TEST_SYSTEM_HOST_UNSET")); err != nil {
		t.Fatalf("unset reference refused offline: %v", err)
	}
}

// TestDoctorReportsHostnameMismatch drives the appliance hostname doctor check
// over a fake kernel name.
// VALIDATES: AC-13 doctor half -- a kernel name that differs from system host
// is a warning naming both; an equal name or an unset leaf reports nothing; an
// unreadable kernel name is reported as unknown, never as a match.
// PREVENTS: a failed sethostname that only the log ever mentions.
func TestDoctorReportsHostnameMismatch(t *testing.T) {
	original := kernelHostname
	t.Cleanup(func() { kernelHostname = original })

	run := func(tree *config.Tree, kernel string, kernelErr error) []diagnostic.Diagnostic {
		kernelHostname = func() (string, error) { return kernel, kernelErr }
		return checkKernelHostname(diagnostic.DoctorCheckContext{Tree: tree})
	}

	got := run(hostTree("cpe-lab"), "ze", nil)
	if len(got) != 1 || got[0].Code != codeSystemHostNotApplied || got[0].Expected != "cpe-lab" || got[0].Actual != "ze" {
		t.Fatalf("mismatch findings = %+v", got)
	}
	if got := run(hostTree("cpe-lab"), "cpe-lab", nil); len(got) != 0 {
		t.Fatalf("matching name reported: %+v", got)
	}
	if got := run(config.NewTree(), "ze", nil); len(got) != 0 {
		t.Fatalf("unset leaf reported: %+v", got)
	}
	got = run(hostTree("cpe-lab"), "", errors.New("read failed"))
	if len(got) != 1 || got[0].Actual != "unknown" {
		t.Fatalf("unreadable kernel name findings = %+v", got)
	}
}
