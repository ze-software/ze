// Design: docs/architecture/core-design.md -- the checks this gate runs itself
// Overview: checks.go -- checkLogSubsystemKeys, subsystemReadingsInGo
//
// Goal: prove the ci-log-subsystem-key gate refuses exactly the hyphenated
// ze.log keys that name no subsystem in any spelling. Method: build a fixture
// tree whose Go source declares a plugin name, a dotted logger name and a
// literal hyphenated logger name, write one .ci per key form, and run the
// check on each.

package docwiring

import (
	"strings"
	"testing"
)

// TestALogKeyIsJudgedInEverySpellingEnvReads holds the gate to the env
// reading: a dot, a hyphen and an underscore are one separator.
//
// VALIDATES: a hyphenated key whose reading is a declared plugin name, a
// declared dotted logger name, a literal hyphenated logger name or a leading
// run of one of them passes; a hyphenated key that names nothing in any
// spelling is refused, and the refusal names the file and the key without
// proposing the dotted spelling as a fix.
// PREVENTS: the gate refusing ze.log.bgp-rib, which env reads as bgp.rib since
// the hyphen became a separator, and the gate passing a key nothing reads.
func TestALogKeyIsJudgedInEverySpellingEnvReads(t *testing.T) {
	const source = "package x\n\n" +
		"var plugin = \"bgp-filter-prefix\"\n" +
		"var logger = \"bgp.reactor.session\"\n" +
		"var literal = \"l2tp-shaper\"\n"

	for _, tc := range []struct {
		name    string
		key     string
		refused bool
	}{
		{"plugin name with hyphens", "bgp-filter-prefix", false},
		{"dotted logger, hyphenated key", "bgp-reactor-session", false},
		{"hyphenated logger literal", "l2tp-shaper", false},
		{"leading run of a plugin name", "bgp-filter", false},
		{"mixed separators", "bgp.reactor-session", false},
		{"names no subsystem", "bgp-no-such-thing", true},
		{"a segment past every subsystem", "l2tp-shaper-extra", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rel := "test/plugin/key.ci"
			root := tree(t, map[string]string{
				"internal/x/x.go": source,
				rel:               "option=env:var=ze.log." + tc.key + ":value=debug\n# ze.log.commented-no-such-key is a comment\n",
			})
			g := &checker{root: root, report: Report{Changed: []string{rel}}}

			result := g.checkLogSubsystemKeys()
			if result.Failed != tc.refused {
				t.Fatalf("ze.log.%s: refused = %v, want %v (%s %v)", tc.key, result.Failed, tc.refused, result.Message, result.Violations)
			}
			if !tc.refused {
				return
			}
			joined := strings.Join(result.Violations, "\n")
			if !strings.Contains(joined, rel+":1: ze.log."+tc.key) {
				t.Errorf("the refusal does not name %s line 1 and the key: %s", rel, joined)
			}
			if strings.Contains(joined, "commented-no-such-key") {
				t.Errorf("a key quoted in a comment was judged: %s", joined)
			}
			if strings.Contains(joined, "did you mean") {
				t.Errorf("the refusal proposes another spelling of the same key as a fix: %s", joined)
			}
		})
	}
}
