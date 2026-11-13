// Design: docs/functional-tests.md -- Editor Tests.
// Related: runner.go -- ordered editor test steps

package testing

import (
	"strings"
	"testing"
)

// TestAPISetFixtureReachesCommit seeds a candidate through the API's editor
// writer, which accepts values for commit-time validation, then types commit.
// A set keystroke would refuse this value before it could test the commit view.
func TestAPISetFixtureReachesCommit(t *testing.T) {
	result := runETTest(`tmpfs=test.conf:terminator=EOF_CONF
bgp {
 router-id 1.2.3.4;
 session { asn { local 65000; } }
 peer peer1 {
  connection { remote { ip 1.1.1.1; } }
  session { asn { remote 65001; } }
 }
}
EOF_CONF
option=file:path=test.conf
fixture=api-set:path=bgp peer peer1 session capability route-refresh:value=requre
input=type:text=commit now
input=enter
expect=status:contains=commit blocked: 1 error(s)
expect=viewport:contains=fix: set bgp peer peer1 session capability route-refresh require
expect=content:contains=requre
`)
	if !result.Passed {
		t.Fatal(result.Error)
	}
}

// TestAPISetFixtureRefusesBadDeclarations proves misspelled fixtures and a
// missing path or value fail discovery instead of silently skipping setup.
func TestAPISetFixtureRefusesBadDeclarations(t *testing.T) {
	for _, fixture := range []string{
		"fixture=api-seet:path=bgp router-id:value=1.2.3.4",
		"fixture=api-set:path=:value=one",
		"fixture=api-set:path=bgp router-id",
		"fixture=api-set:value=one",
		`fixture=api-set:path=bgp peer "unterminated:value=one`,
	} {
		t.Run(fixture, func(t *testing.T) {
			_, err := parseETFile(fixture)
			if err == nil {
				t.Fatal("bad fixture was accepted")
			}
			if !strings.Contains(err.Error(), "fixture") {
				t.Fatalf("error does not name the fixture: %v", err)
			}
		})
	}
}

// TestAPISetFixtureKeepsTokenAndValueBoundaries checks that IPv6 keys and
// empty or colon-bearing values are data, not fixture separators.
func TestAPISetFixtureKeepsTokenAndValueBoundaries(t *testing.T) {
	for _, value := range []string{"", "target:65000:1"} {
		t.Run(value, func(t *testing.T) {
			tc, err := parseETFile("fixture=api-set:path=policy prefix-list sample entry 2001:db8::/32 action:value=" + value)
			if err != nil {
				t.Fatal(err)
			}
			if len(tc.Fixtures) != 1 || len(tc.Steps) != 1 {
				t.Fatal("fixture was not retained as an ordered step")
			}
			fixture := tc.Fixtures[0]
			if fixture.path[4] != "2001:db8::/32" || fixture.value != value {
				t.Fatalf("fixture boundaries changed: %+v", fixture)
			}
			if tc.Steps[0].Type != StepFixture || tc.Steps[0].FixtureIndex != 0 {
				t.Fatalf("fixture step = %+v", tc.Steps[0])
			}
		})
	}
}

// TestAPISetFixtureNeedsAnEditor proves operational-only cases cannot
// silently ignore a candidate fixture.
func TestAPISetFixtureNeedsAnEditor(t *testing.T) {
	result := runETTest("option=mode:value=operational\nfixture=api-set:path=bgp router-id:value=192.0.2.1\n")
	if result.Passed || !strings.Contains(result.Error, "fixture has no configuration editor") {
		t.Fatalf("fixture result = %+v", result)
	}
}
