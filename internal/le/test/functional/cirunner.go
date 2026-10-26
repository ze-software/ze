// Design: docs/architecture/testing/ci-format.md -- the runner that runs one .ci of test/<dir>/
// Overview: suites.go -- the functional suite table, the first declaration asked
//
// cirunner.go answers which runner walks a test/<dir>/ directory of .ci files,
// and the words that run ONE of them. The answer comes from each runner's own
// declaration, never from a list kept here: the functional suite table, the
// `le test bgp` runner's directories, and the `le test <dir>` commands the
// harness packages register. A suite name is not always its directory (suite
// `installer` runs test/install/ through `le test install`), and a directory can
// have a runner and no suite row (test/pppoe/), so walking the suite table alone
// misses both.

package testfunctional

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
	testcli "github.com/ze-software/ze/internal/test/cli"
)

// testWord is the le command every functional runner is filed under, and the
// directory of the checkout every .ci runner walks a subdirectory of.
const testWord = "test"

// CIRunnerWords answers the words after `le test` that run one .ci of
// test/<dir>/, before its stem, and whether any runner walks that directory.
// A .ci runner selects a file by its stem and walks one directory level only.
//
// The registered `le test <dir>` commands are asked last, so the answer needs
// the process to hold le's registrations: inside le it does, and a test that
// asks about such a directory blank-imports the harness package.
func CIRunnerWords(dir string) ([]string, bool) {
	if suite, held := SuiteNamed(dir); held {
		return suite.OneTestWords(), true
	}
	if testcli.BgpRunnerDir(dir) {
		return []string{"bgp", dir}, true
	}
	if leroot.LookupCommand(testWord+" "+dir) != nil {
		return []string{dir}, true
	}
	return nil, false
}

// OneTestWords answers the suite's words after `le test` that run one test,
// before its selector. They are the words before the select-every-test flag:
// the words after it tune how the runner runs the whole suite (`-p 8`), and one
// test does not need them.
func (s Suite) OneTestWords() []string {
	words := make([]string, 0, len(s.Args))
	for _, arg := range s.Args {
		if arg == AllTests {
			break
		}
		words = append(words, arg)
	}
	return words
}
