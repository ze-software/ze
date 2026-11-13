// Design: docs/architecture/testing/ci-format.md -- API candidate fixtures.
// Related: runner.go -- fixture steps run in order with typed commands.

package testing

import (
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

// apiSetFixture retains the token boundaries of an absolute setting path.
// The editor API permits a leaf value until commit-time validation, so this
// fixture can exercise a refused commit without weakening keystroke checks.
type apiSetFixture struct {
	path  []string
	value string
}

func (tc *testCase) parseFixture(rest string) error {
	path, found := strings.CutPrefix(rest, "api-set:path=")
	if !found {
		return opererr.New("the editor test fixture is not declared correctly",
			"the candidate fixture needs api-set, an absolute setting path and a value",
			"fixture=api-set:path=<full setting path>:value=<value>")
	}
	path, value, found := strings.Cut(path, ":value=")
	if !found {
		return opererr.New("the editor test fixture has no value",
			"api-set needs the value after its setting path, including an empty value when testing length",
			"fixture=api-set:path=<full setting path>:value=<value>")
	}
	words, err := config.ParseEditPath(path)
	if err != nil {
		return opererr.Wrap(err, "the editor test fixture has an unreadable setting path",
			"path uses the editor's quoted word syntax",
			"fixture=api-set:path=<full setting path>:value=<value>")
	}
	if len(words) == 0 {
		return opererr.New("the editor test fixture has no setting path",
			"api-set names the setting from the configuration root",
			"fixture=api-set:path=<full setting path>:value=<value>")
	}
	tc.Fixtures = append(tc.Fixtures, apiSetFixture{path: words, value: value})
	tc.Steps = append(tc.Steps, Step{Type: StepFixture, FixtureIndex: len(tc.Fixtures) - 1})
	return nil
}

func (hm *headlessModel) applyFixture(fixture apiSetFixture) error {
	if hm.editor == nil {
		return opererr.New("the editor test fixture has no configuration editor",
			"api-set prepares a candidate and cannot run in operational mode",
			"option=file:path=test.conf")
	}
	last := len(fixture.path) - 1
	return hm.editor.SetValue(fixture.path[:last], fixture.path[last], fixture.value)
}
