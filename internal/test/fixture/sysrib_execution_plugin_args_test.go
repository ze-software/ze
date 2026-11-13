// Design: docs/guide/route-policy.md -- producer placement is independent of the system RIB.
// Related: sysrib_execution_fixture_test.go -- original nine RIB argv controls remain intact.

package fixture

import "testing"

func TestSysribExecutionPluginArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"configured static", []string{"ze", "plugin", "static"}, true},
		{"static with trailing flags", []string{"/tmp/zebin/ze", "plugin", "static", "--log-level", "info"}, true},
		{"RIB child is not static producer", []string{"ze", "plugin", "rib"}, false},
		{"shell text is not static child", []string{"sh", "-c", "ze plugin static"}, false},
		{"plugin suffix in different command", []string{"ze", "other", "plugin", "static"}, false},
		{"missing plugin", []string{"ze", "plugin"}, false},
		{"empty", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sysribExecutionPluginArgs(tc.args, "static"); got != tc.want {
				t.Fatalf("static invocation %q = %t, want %t", tc.args, got, tc.want)
			}
		})
	}
}
