// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

// EditorTest holds a single editor test case.
type EditorTest struct {
	BaseTest // Embeds Name, Nick, Active, Error
	Path     string

	// Results (filled during execution)
	ErrMsg  string
	TempDir string
}

// EditorTests manages editor test discovery and execution.
type EditorTests struct {
	*TestSet[*EditorTest]
}

// NewEditorTests creates a new editor test manager.
func NewEditorTests() *EditorTests {
	return &EditorTests{
		TestSet: NewTestSet[*EditorTest](),
	}
}

// Add creates and registers a new editor test.
func (et *EditorTests) Add(name, nick, path string) *EditorTest {
	test := &EditorTest{
		BaseTest: BaseTest{
			Name: name,
			Nick: nick,
		},
		Path: path,
	}
	et.TestSet.Add(test)
	return test
}
