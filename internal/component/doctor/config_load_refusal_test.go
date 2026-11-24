// Design: docs/architecture/doctor-and-health-checks.md -- loader refusals carry actionable doctor fixes.
// Related: doctor.go -- runChecks must split every config refusal into its message and fix.

package doctor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The missing use leaf is accepted by syntax parsing but refused by the real
// loader's plugin extraction. This reaches runChecks' loadErr arm rather than
// either earlier declaration/syntax refusal. The valid sibling must return its
// judged tree without inventing a parse finding.
func TestRunChecksGivesLoaderRefusalsAnActionableFix(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		refused bool
	}{
		{name: "loader-refusal", content: "bus {\n endpoint incomplete {\n }\n}\n", refused: true},
		{name: "valid-config", content: "# valid empty configuration\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestConfig(t, test.content)
			pinConfigDir(t, filepath.Dir(path))
			parsed, parseErr := config.ParseTreeForValidation(test.content)
			require.NoError(t, parseErr, "this fixture must reach validated loading after syntax parsing")
			require.NotNil(t, parsed)
			loaded, loadErr := config.ReadConfig(test.content, path, nil)
			if test.refused {
				require.ErrorContains(t, loadErr, "internal or external")
				require.Nil(t, loaded)
			} else {
				require.NoError(t, loadErr)
				require.NotNil(t, loaded)
			}

			diags, judged := runChecks(path)
			var parseFinding *diagnostic.Diagnostic
			count := 0
			for index := range diags {
				if diags[index].Code == doctorConfigParse {
					parseFinding = &diags[index]
					count++
				}
			}
			if !test.refused {
				assert.Zero(t, count, "valid loading must not produce a config-parse finding")
				assert.NotNil(t, judged, "valid loading must return the judged configuration")
				return
			}
			require.Equal(t, 1, count, "the loader refusal must produce one config-parse finding")
			require.NotNil(t, parseFinding)
			assert.Nil(t, judged, "a refused load must not reach the post-config/runtime tree")
			assert.Equal(t, diagnostic.SeverityError, parseFinding.Severity)
			assert.Contains(t, parseFinding.Message, `endpoint "incomplete"`)
			assert.Contains(t, parseFinding.Message, "must specify either internal or external")
			assert.Contains(t, parseFinding.Fix, "bus { endpoint incomplete")
			assert.NotContains(t, parseFinding.Message, "\nfix: ", "the fix belongs only on the finding's fix line")
		})
	}
}
