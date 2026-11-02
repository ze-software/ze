// Tests for an ExaBGP traffic-class above 63. ExaBGP 4.2 and 5.0 accept one
// up to 255 and send it as one octet unchanged: the 4.2 conf-flow fixture's
// traffic-class 101 went out as 0x65 (test/exabgp-compat/old-4.2/conf-flow.ci).
// RFC 8955 Section 4.2.2.11 reads the DSCP from "The six least significant
// bits" of that octet, and "All other bits SHOULD be treated as 0", which is
// how ExaBGP 6.0 decodes it (flow.py _dscp). The migration writes that DSCP.

package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trafficClassConfig is an ExaBGP config with one IPv6 flow route matching
// the traffic-class written in match.
func trafficClassConfig(match string) string {
	return `neighbor 127.0.0.1 {
	router-id 10.0.0.1;
	local-address 127.0.0.1;
	local-as 65500;
	peer-as 65500;
	family {
		ipv6 flow;
	}
	flow {
		route tc {
			match {
				destination 2001:db8::1/128;
				traffic-class ` + match + `;
			}
			then {
				discard;
			}
		}
	}
}
`
}

// migrateTrafficClass migrates trafficClassConfig(match) and answers the
// serialized ze config and the warnings.
func migrateTrafficClass(t *testing.T, match string) (string, []string) {
	t.Helper()
	tree, err := ParseExaBGPConfig(trafficClassConfig(match))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := MigrateFromExaBGP(tree)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return SerializeTree(result.Tree), result.Warnings
}

// TestMigrateFlowTrafficClassSixBits proves a traffic-class operand above 63
// is written as the DSCP its six low bits carry, with every operator kept.
//
// VALIDATES: 101 becomes 37 (0x65 & 0x3F), 64 becomes 0 and 255 becomes 63;
// an operand from 0 to 63 is written unchanged; a list and an & join keep
// their operators; the operator is told, naming the route and both values.
// PREVENTS: `ze exabgp migrate` writing a traffic-class ze refuses (value 101
// exceeds max 63) for a config ExaBGP 4.2 and 5.0 load and announce.
func TestMigrateFlowTrafficClassSixBits(t *testing.T) {
	cases := []struct {
		match string
		want  string
		warn  bool
	}{
		{match: "101", want: "traffic-class 37", warn: true},
		{match: "64", want: "traffic-class 0", warn: true},
		{match: "255", want: "traffic-class 63", warn: true},
		{match: "63", want: "traffic-class 63"},
		{match: "46", want: "traffic-class 46"},
		{match: "0", want: "traffic-class 0"},
		{match: "[ =101 >200 ]", want: "traffic-class [ =37 >8 ]", warn: true},
		{match: ">=100&<=110", want: "traffic-class >=36&<=46", warn: true},
	}
	for _, tc := range cases {
		t.Run(tc.match, func(t *testing.T) {
			output, warnings := migrateTrafficClass(t, tc.match)
			if !strings.Contains(output, tc.want) {
				t.Errorf("migrated config lacks %q:\n%s", tc.want, output)
			}
			var said []string
			for _, w := range warnings {
				if strings.Contains(w, "traffic-class") {
					said = append(said, w)
				}
			}
			if !tc.warn {
				if len(said) != 0 {
					t.Errorf("a traffic-class from 0 to 63 is migrated unchanged, yet warned: %q", said)
				}
				return
			}
			if len(said) != 1 {
				t.Fatalf("want one traffic-class warning, got %q", warnings)
			}
			for _, word := range []string{"flow route tc", tc.match, strings.TrimPrefix(tc.want, "traffic-class "), "RFC 8955 Section 4.2.2.11"} {
				if !strings.Contains(said[0], word) {
					t.Errorf("warning lacks %q: %q", word, said[0])
				}
			}
		})
	}
}

// TestMigrateFlowTrafficClassAboveOctet proves an operand no octet holds is
// left as written, for ze to refuse.
//
// VALIDATES: traffic-class 256 and 300 reach the migrated config unchanged,
// with no traffic-class warning.
// PREVENTS: the migration inventing a DSCP for a value ExaBGP 4.2 and 5.0 do
// not send either: their IOperationByte.encode packs the value into one
// octet, which a value from 256 up does not fit.
func TestMigrateFlowTrafficClassAboveOctet(t *testing.T) {
	for _, match := range []string{"256", "300"} {
		t.Run(match, func(t *testing.T) {
			output, warnings := migrateTrafficClass(t, match)
			if want := "traffic-class " + match; !strings.Contains(output, want) {
				t.Errorf("migrated config lacks %q:\n%s", want, output)
			}
			for _, w := range warnings {
				if strings.Contains(w, "traffic-class") {
					t.Errorf("unexpected traffic-class warning: %q", w)
				}
			}
		})
	}
}

// TestMigrateFlowTrafficClassOldFixtures proves the 4.2 and 5.0 conf-flow
// fixtures, whose IPv6 route matches traffic-class 101, migrate to DSCP 37.
//
// VALIDATES: both migrated configs carry traffic-class 37 and not 101.
// PREVENTS: the ported fixtures' migrated config being refused at load, the
// AC-12 finding of spec-config-validate-every-repo-config (closed).
func TestMigrateFlowTrafficClassOldFixtures(t *testing.T) {
	for _, version := range []string{"old-4.2", "old-5.0"} {
		t.Run(version, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "test", "exabgp-compat", version, "etc", "conf-flow.conf")
			data, err := os.ReadFile(path) //nolint:gosec // Test data path.
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			tree, err := ParseExaBGPConfig(string(data))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			result, err := MigrateFromExaBGP(tree)
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}
			output := SerializeTree(result.Tree)
			if !strings.Contains(output, "traffic-class 37") {
				t.Errorf("migrated config lacks traffic-class 37:\n%s", output)
			}
			if strings.Contains(output, "traffic-class 101") {
				t.Errorf("migrated config still carries traffic-class 101:\n%s", output)
			}
		})
	}
}
