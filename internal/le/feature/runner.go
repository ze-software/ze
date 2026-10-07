// Design: docs/contributing/feature-maturity.md -- which real-path tests count, and who runs them
// Related: recordrun.go -- record-run runs each item through the runner answered here
// Related: check.go -- testItem refuses an item no runner runs
//
// A real-path test file counts only when a runner of the repository runs it,
// and the answer comes from each runner's own declaration, never from a list
// kept here: the functional suite table (testfunctional.Suites), the editor
// runner's directory and suffix, the `le test bgp` runner's directories, and the
// `le test <dir>` commands the harness packages register. A file none of them
// walks is refused even though it exists, because nothing would ever run it.

package feature

import (
	"path"
	"strings"

	leroot "github.com/ze-software/ze/internal/le/le/root"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
	testcli "github.com/ze-software/ze/internal/test/cli"
)

// ciSuffix is the suffix of a file the .ci runners discover.
const ciSuffix = ".ci"

// testTreeDir is the directory every functional runner walks a subdirectory of,
// and the word le files every runner command under.
const testTreeDir = "test"

// testRunner is the command that runs one real-path test file.
type testRunner struct {
	// words follow `le test` and select the runner.
	words []string
	// selector names the file to that runner. It is the last word of the
	// command, and the runner's PASS line for the file ends with it.
	selector string
}

// argv answers the whole command under le, the binary at leBinary.
func (r testRunner) argv(leBinary string) []string {
	argv := make([]string, 0, len(r.words)+3)
	argv = append(argv, leBinary, testTreeDir)
	argv = append(argv, r.words...)
	return append(argv, r.selector)
}

// runnerOf answers the runner that runs the test file at rel, or why none does.
func runnerOf(rel string) (testRunner, string) {
	parts := strings.Split(rel, "/")
	if len(parts) < 3 {
		return testRunner{}, "'" + rel + "' is not under test/<dir>/, where the functional runners look"
	}
	if parts[0] != testTreeDir {
		return testRunner{}, "'" + rel + "' is not under test/<dir>/, where the functional runners look"
	}
	dir := parts[1]
	switch path.Ext(rel) {
	case testcli.EditorTestSuffix:
		return editorRunnerOf(rel, dir)
	case ciSuffix:
		return ciRunnerOf(rel, dir, len(parts))
	}
	return testRunner{}, "'" + rel + "' is neither a Go test (file.go::TestName), a " + ciSuffix +
		" nor a " + testcli.EditorTestSuffix
}

// editorRunnerOf answers the editor runner, which walks test/<EditorSuiteDir>/
// recursively and selects a file by its repository-relative path.
func editorRunnerOf(rel, dir string) (testRunner, string) {
	if dir != testcli.EditorSuiteDir {
		return testRunner{}, "'" + rel + "': the editor runner walks only test/" + testcli.EditorSuiteDir + "/"
	}
	suite, held := testfunctional.SuiteNamed(dir)
	if !held {
		return testRunner{}, "'" + rel + "': no functional suite named '" + dir + "' runs the editor runner"
	}
	return testRunner{words: suiteWords(suite), selector: rel}, ""
}

// ciRunnerOf answers the .ci runner that walks test/<dir>/, asking each .ci
// runner's own declaration in turn. A .ci runner selects a file by its stem
// and walks one directory level only.
func ciRunnerOf(rel, dir string, depth int) (testRunner, string) {
	if depth != 3 {
		return testRunner{}, "'" + rel + "' is not test/<dir>/<name>" + ciSuffix + ", which is where the .ci runners look"
	}
	stem := strings.TrimSuffix(path.Base(rel), ciSuffix)
	if suite, held := testfunctional.SuiteNamed(dir); held {
		return testRunner{words: suiteWords(suite), selector: stem}, ""
	}
	if testcli.BgpRunnerDir(dir) {
		return testRunner{words: []string{"bgp", dir}, selector: stem}, ""
	}
	if leroot.LookupCommand(testTreeDir+" "+dir) != nil {
		return testRunner{words: []string{dir}, selector: stem}, ""
	}
	return testRunner{}, "'" + rel + "': no functional suite, `le test bgp` directory or `le test " + dir +
		"` command runs test/" + dir + "/"
}

// suiteWords answers a functional suite's words without the all-tests flag,
// so the selector picks one file.
func suiteWords(suite testfunctional.Suite) []string {
	words := make([]string, 0, len(suite.Args))
	for _, arg := range suite.Args {
		if arg == testfunctional.AllTests {
			continue
		}
		words = append(words, arg)
	}
	return words
}
