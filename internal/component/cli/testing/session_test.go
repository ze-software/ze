package testing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
)

// TestETSessionOption verifies that option=session activates a session on the headless model.
//
// VALIDATES: Session option creates session identity, write-through creates draft with metadata.
// PREVENTS: Session features untestable in .et format.
func TestETSessionOption(t *testing.T) {
	et := `
tmpfs=test.conf:terminator=EOF
bgp {
  local-as 65000
  router-id 1.2.3.4
}
EOF
option=file:path=test.conf
option=session:user=thomas:origin=local

input=type:text=set bgp local-as 65001
input=enter
expect=dirty:true
expect=file:path=test.conf.draft:contains=#thomas @local
`
	result := RunETTest(et)
	if !result.Passed {
		t.Fatalf("test failed: %s", result.Error)
	}
}

// TestETMultiSession verifies multi-session support with session= directives.
//
// VALIDATES: Multiple sessions can edit the same config, switch between them.
// PREVENTS: Concurrent editing untestable in .et format.
func TestETMultiSession(t *testing.T) {
	et := `
tmpfs=test.conf:terminator=EOF
bgp {
  local-as 65000
  router-id 1.2.3.4
}
EOF
option=file:path=test.conf

session=alice:user=alice,origin=ssh
input=type:text=set bgp local-as 65001
input=enter
expect=dirty:true

session=bob:user=bob,origin=ssh
input=type:text=set bgp router-id 5.6.7.8
input=enter
expect=dirty:true

expect=file:path=test.conf.draft:contains=#alice @ssh
expect=file:path=test.conf.draft:contains=#bob @ssh
`
	result := RunETTest(et)
	if !result.Passed {
		t.Fatalf("test failed: %s", result.Error)
	}
}

// TestETFileExpectation verifies expect=file checks on-disk content.
//
// VALIDATES: File expectations can check contains, not-contains, and absent.
// PREVENTS: Draft/config file state invisible to .et tests.
func TestETFileExpectation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello world"), 0o600)
	require.NoError(t, err)

	fs := &fileState{tmpDir: tmpDir}

	// Test contains
	exp := Expectation{
		Type:   "file",
		Values: map[string]string{"path": "test.txt", "contains": "hello"},
	}
	err = CheckExpectation(exp, fs)
	assert.NoError(t, err, "file contains 'hello' should pass")

	// Test not-contains
	exp2 := Expectation{
		Type:   "file",
		Values: map[string]string{"path": "test.txt", "not-contains": "goodbye"},
	}
	err = CheckExpectation(exp2, fs)
	assert.NoError(t, err, "file not-contains 'goodbye' should pass")

	// Test absent
	exp3 := Expectation{
		Type:   "file",
		Values: map[string]string{"path": "nonexistent.txt", "absent": ""},
	}
	err = CheckExpectation(exp3, fs)
	assert.NoError(t, err, "absent file should pass")

	// Test absent fails when file exists
	exp4 := Expectation{
		Type:   "file",
		Values: map[string]string{"path": "test.txt", "absent": ""},
	}
	err = CheckExpectation(exp4, fs)
	assert.Error(t, err, "absent should fail when file exists")
}

// TestETSessionSwitch verifies switching back to a previously created session.
//
// VALIDATES: session=name without parameters switches to existing session.
// PREVENTS: Session state lost when switching between sessions.
func TestETSessionSwitch(t *testing.T) {
	et := `
tmpfs=test.conf:terminator=EOF
bgp {
  local-as 65000
  router-id 1.2.3.4
}
EOF
option=file:path=test.conf

session=alice:user=alice,origin=ssh
input=type:text=set bgp local-as 65001
input=enter

session=bob:user=bob,origin=ssh
input=type:text=set bgp router-id 5.6.7.8
input=enter

session=alice
expect=dirty:true
`
	result := RunETTest(et)
	if !result.Passed {
		t.Fatalf("test failed: %s", result.Error)
	}
}

// TestParseSessionErrors verifies that parseSession rejects malformed directives.
//
// VALIDATES: Parser catches empty name, missing params, unknown params, missing user/origin.
// PREVENTS: Invalid session directives silently producing broken SessionActions.
func TestParseSessionErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // substring of expected error
	}{
		{name: "empty name", input: "session=", want: "session requires a name"},
		{name: "missing equals", input: "session=alice:userfoo", want: "key=value"},
		{name: "unknown param", input: "session=alice:user=alice,badkey=val", want: "unknown session parameter"},
		{name: "missing user", input: "session=alice:origin=ssh", want: "requires user"},
		{name: "missing origin", input: "session=alice:user=alice", want: "requires origin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseETFile(tt.input)
			require.Error(t, err, "expected error for input: %s", tt.input)
			assert.Contains(t, err.Error(), tt.want,
				"error for %q should contain %q, got: %v", tt.input, tt.want, err)
		})
	}
}

// TestCheckFilePathTraversal verifies that path traversal is rejected.
//
// VALIDATES: File expectation rejects paths with .. or absolute paths.
// PREVENTS: Test .et files reading files outside tmpDir.
func TestCheckFilePathTraversal(t *testing.T) {
	fs := &fileState{tmpDir: t.TempDir()}

	tests := []struct {
		name string
		path string
	}{
		{name: "parent traversal", path: "../etc/passwd"},
		{name: "deep traversal", path: "../../secret"},
		{name: "absolute path", path: "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := Expectation{
				Type:   "file",
				Values: map[string]string{"path": tt.path, "contains": "root"},
			}
			err := CheckExpectation(exp, fs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "relative without")
		})
	}
}

// fileState is a minimal State implementation for testing file expectations directly.
type fileState struct {
	tmpDir string
}

func (fs *fileState) TmpDir() string                                  { return fs.tmpDir }
func (fs *fileState) ContextPath() []string                           { return nil }
func (fs *fileState) Completions() []cli.Completion                   { return nil }
func (fs *fileState) GhostText() string                               { return "" }
func (fs *fileState) ValidationErrors() []cli.ConfigValidationError   { return nil }
func (fs *fileState) ValidationWarnings() []cli.ConfigValidationError { return nil }
func (fs *fileState) Dirty() bool                                     { return false }
func (fs *fileState) StatusMessage() string                           { return "" }
func (fs *fileState) Error() error                                    { return nil }
func (fs *fileState) IsTemplate() bool                                { return false }
func (fs *fileState) ShowDropdown() bool                              { return false }
func (fs *fileState) WorkingContent() string                          { return "" }
func (fs *fileState) ViewportContent() string                         { return "" }
func (fs *fileState) ConfirmTimerActive() bool                        { return false }
func (fs *fileState) TriggerCompletions()                             {}
func (fs *fileState) Mode() cli.EditorMode                            { return cli.ModeEdit }
