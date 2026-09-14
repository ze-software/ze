// Related: conntrack.go -- the log-invalid table this test reads

package system

import (
	"maps"
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang, which declares the log-invalid leaf
)

// TestLogInvalidProtocolsMatchTheModel ties the protocols logInvalidProtocols
// knows to the enumeration the log-invalid leaf declares.
//
// The Go side is the declaration: each name is paired with the value the
// kernel reads from nf_conntrack_log_invalid (a protocol number, or 255 for
// all), which the model states only in prose. ConntrackSysctlKeys writes no
// key for a name the table lacks, so a value only the model carried would be
// accepted at commit and then silently not applied. The model is the copy,
// and this test keeps it honest in both directions.
//
// VALIDATES: the enumeration at system/conntrack/log-invalid and the names the
// table maps are one set, and each declared name reaches the sysctl.
// PREVENTS: a log-invalid value an operator commits that sets nothing.
func TestLogInvalidProtocolsMatchTheModel(t *testing.T) {
	const path = "system/conntrack/log-invalid"

	declared, err := configyang.EnumValues(path)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", path, err)
	}
	if len(declared) == 0 {
		t.Fatalf("the model declares no protocol at %s", path)
	}

	known := slices.Sorted(maps.Keys(logInvalidProtocols))
	if !slices.Equal(declared, known) {
		t.Errorf("the model declares %v at %s, and logInvalidProtocols maps %v", declared, path, known)
	}
	for _, name := range declared {
		cc := ConntrackConfig{LogInvalid: name}
		if _, set := cc.ConntrackSysctlKeys()["net.netfilter.nf_conntrack_log_invalid"]; !set {
			t.Errorf("the model declares log-invalid %q and ConntrackSysctlKeys writes no sysctl for it", name)
		}
	}
}
