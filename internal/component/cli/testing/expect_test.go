package testing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
)

// TestExpectContextRoot verifies root context assertion.
//
// VALIDATES: expect=context:root matches empty context path.
// PREVENTS: Root context check failing incorrectly.
func TestExpectContextRoot(t *testing.T) {
	exp := Expectation{Type: "context", Values: map[string]string{"root": ""}}

	// Empty context should pass
	err := CheckExpectation(exp, &MockState{contextPath: nil})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{contextPath: []string{}})
	assert.NoError(t, err)

	// Non-empty context should fail
	err = CheckExpectation(exp, &MockState{contextPath: []string{"bgp"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

// TestExpectContextPath verifies context path assertion.
//
// VALIDATES: expect=context:path= matches exact path.
// PREVENTS: Context navigation tests failing incorrectly.
func TestExpectContextPath(t *testing.T) {
	exp := Expectation{Type: "context", Values: map[string]string{"path": "bgp.peer.peer1"}}

	// Matching path should pass
	err := CheckExpectation(exp, &MockState{contextPath: []string{"bgp", "peer", "peer1"}})
	assert.NoError(t, err)

	// Wrong path should fail
	err = CheckExpectation(exp, &MockState{contextPath: []string{"bgp"}})
	assert.Error(t, err)

	// Root should fail
	err = CheckExpectation(exp, &MockState{contextPath: nil})
	assert.Error(t, err)
}

// TestExpectDirty verifies dirty flag assertion.
//
// VALIDATES: expect=dirty:true/false matches dirty state.
// PREVENTS: Modified flag checks failing.
func TestExpectDirty(t *testing.T) {
	expTrue := Expectation{Type: "dirty", Values: map[string]string{"true": ""}}
	expFalse := Expectation{Type: "dirty", Values: map[string]string{"false": ""}}

	// Dirty true
	err := CheckExpectation(expTrue, &MockState{dirty: true})
	assert.NoError(t, err)

	err = CheckExpectation(expTrue, &MockState{dirty: false})
	assert.Error(t, err)

	// Dirty false
	err = CheckExpectation(expFalse, &MockState{dirty: false})
	assert.NoError(t, err)

	err = CheckExpectation(expFalse, &MockState{dirty: true})
	assert.Error(t, err)
}

// TestExpectErrorNone verifies no error assertion.
//
// VALIDATES: expect=error:none passes when no error.
// PREVENTS: Commands incorrectly reported as failed.
func TestExpectErrorNone(t *testing.T) {
	exp := Expectation{Type: "error", Values: map[string]string{"none": ""}}

	err := CheckExpectation(exp, &MockState{err: nil})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{err: assert.AnError})
	assert.Error(t, err)
}

// TestExpectErrorContains verifies error message assertion.
//
// VALIDATES: expect=error:contains= matches error substring.
// PREVENTS: Error message checks failing incorrectly.
func TestExpectErrorContains(t *testing.T) {
	exp := Expectation{Type: "error", Values: map[string]string{"contains": "not found"}}

	err := CheckExpectation(exp, &MockState{err: assert.AnError})
	assert.Error(t, err) // AnError doesn't contain "not found"

	mockErr := &mockError{msg: "block not found"}
	err = CheckExpectation(exp, &MockState{err: mockErr})
	assert.NoError(t, err)
}

// TestExpectCompletionContains verifies completion list contains items.
//
// VALIDATES: expect=completion:contains= checks all items present.
// PREVENTS: Tab completion tests failing incorrectly.
func TestExpectCompletionContains(t *testing.T) {
	exp := Expectation{Type: "completion", Values: map[string]string{"contains": "set,delete,edit"}}

	comps := []cli.Completion{
		{Text: "set"},
		{Text: "delete"},
		{Text: "edit"},
		{Text: "show"},
	}

	err := CheckExpectation(exp, &MockState{completions: comps})
	assert.NoError(t, err)

	// Missing one
	comps = []cli.Completion{
		{Text: "set"},
		{Text: "delete"},
	}
	err = CheckExpectation(exp, &MockState{completions: comps})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "edit")
}

// TestExpectCompletionCount verifies completion count assertion.
//
// VALIDATES: expect=completion:count= checks exact count.
// PREVENTS: Completion count assertions failing.
func TestExpectCompletionCount(t *testing.T) {
	exp := Expectation{Type: "completion", Values: map[string]string{"count": "3"}}

	comps := []cli.Completion{{Text: "a"}, {Text: "b"}, {Text: "c"}}
	err := CheckExpectation(exp, &MockState{completions: comps})
	assert.NoError(t, err)

	comps = []cli.Completion{{Text: "a"}, {Text: "b"}}
	err = CheckExpectation(exp, &MockState{completions: comps})
	assert.Error(t, err)
}

// TestExpectCompletionEmpty verifies empty completion list.
//
// VALIDATES: expect=completion:empty checks no completions.
// PREVENTS: Empty completion checks failing.
func TestExpectCompletionEmpty(t *testing.T) {
	exp := Expectation{Type: "completion", Values: map[string]string{"empty": ""}}

	err := CheckExpectation(exp, &MockState{completions: nil})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{completions: []cli.Completion{}})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{completions: []cli.Completion{{Text: "x"}}})
	assert.Error(t, err)
}

// TestExpectGhostText verifies ghost text assertion.
//
// VALIDATES: expect=ghost:text= matches ghost suffix.
// PREVENTS: Ghost text assertions failing.
func TestExpectGhostText(t *testing.T) {
	exp := Expectation{Type: "ghost", Values: map[string]string{"text": "-as"}}

	err := CheckExpectation(exp, &MockState{ghostText: "-as"})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{ghostText: "-id"})
	assert.Error(t, err)
}

// TestExpectGhostEmpty verifies empty ghost text.
//
// VALIDATES: expect=ghost:empty checks no ghost text.
// PREVENTS: Ghost empty checks failing.
func TestExpectGhostEmpty(t *testing.T) {
	exp := Expectation{Type: "ghost", Values: map[string]string{"empty": ""}}

	err := CheckExpectation(exp, &MockState{ghostText: ""})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{ghostText: "-as"})
	assert.Error(t, err)
}

// TestExpectErrorsCount verifies error count assertion.
//
// VALIDATES: expect=errors:count= checks validation error count.
// PREVENTS: Validation error count assertions failing.
func TestExpectErrorsCount(t *testing.T) {
	exp := Expectation{Type: "errors", Values: map[string]string{"count": "0"}}

	err := CheckExpectation(exp, &MockState{validationErrors: nil})
	assert.NoError(t, err)

	errs := []cli.ConfigValidationError{{Message: "err1"}}
	err = CheckExpectation(exp, &MockState{validationErrors: errs})
	assert.Error(t, err)

	exp = Expectation{Type: "errors", Values: map[string]string{"count": "2"}}
	errs = []cli.ConfigValidationError{{Message: "err1"}, {Message: "err2"}}
	err = CheckExpectation(exp, &MockState{validationErrors: errs})
	assert.NoError(t, err)
}

// TestExpectContentContains verifies content contains assertion.
//
// VALIDATES: expect=content:contains= checks text in content.
// PREVENTS: Content checks failing incorrectly.
func TestExpectContentContains(t *testing.T) {
	exp := Expectation{Type: "content", Values: map[string]string{"contains": "as 65001"}}

	content := "bgp {\n  peer peer1 {\n    remote {\n      ip 1.1.1.1\n      as 65001\n    }\n  }\n}"
	err := CheckExpectation(exp, &MockState{workingContent: content})
	assert.NoError(t, err)

	content = "bgp {\n  peer peer1 {\n    remote {\n      ip 1.1.1.1\n      as 65002\n    }\n  }\n}"
	err = CheckExpectation(exp, &MockState{workingContent: content})
	assert.Error(t, err)
}

// TestExpectContentNotContains verifies content not-contains assertion.
//
// VALIDATES: expect=content:not-contains= checks text absent.
// PREVENTS: Negative content checks failing.
func TestExpectContentNotContains(t *testing.T) {
	exp := Expectation{Type: "content", Values: map[string]string{"not-contains": "error"}}

	err := CheckExpectation(exp, &MockState{workingContent: "bgp { local { as 65000; } }"})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{workingContent: "error: something wrong"})
	assert.Error(t, err)
}

// TestExpectStatusContains verifies status message assertion.
//
// VALIDATES: expect=status:contains= checks status message.
// PREVENTS: Status message assertions failing.
func TestExpectStatusContains(t *testing.T) {
	exp := Expectation{Type: "status", Values: map[string]string{"contains": "committed"}}

	err := CheckExpectation(exp, &MockState{statusMessage: "Configuration committed"})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{statusMessage: "Changes discarded"})
	assert.Error(t, err)
}

// TestExpectStatusEmpty verifies empty status assertion.
//
// VALIDATES: expect=status:empty checks no status message.
// PREVENTS: Status empty checks failing.
func TestExpectStatusEmpty(t *testing.T) {
	exp := Expectation{Type: "status", Values: map[string]string{"empty": ""}}

	err := CheckExpectation(exp, &MockState{statusMessage: ""})
	assert.NoError(t, err)

	err = CheckExpectation(exp, &MockState{statusMessage: "some message"})
	assert.Error(t, err)
}

// TestExpectTemplate verifies template mode assertion.
//
// VALIDATES: expect=template:true/false checks template mode.
// PREVENTS: Template mode assertions failing.
func TestExpectTemplate(t *testing.T) {
	expTrue := Expectation{Type: "template", Values: map[string]string{"true": ""}}
	expFalse := Expectation{Type: "template", Values: map[string]string{"false": ""}}

	err := CheckExpectation(expTrue, &MockState{isTemplate: true})
	assert.NoError(t, err)

	err = CheckExpectation(expTrue, &MockState{isTemplate: false})
	assert.Error(t, err)

	err = CheckExpectation(expFalse, &MockState{isTemplate: false})
	assert.NoError(t, err)
}

// TestExpectDropdown verifies dropdown visibility assertion.
//
// VALIDATES: expect=dropdown:visible/hidden checks dropdown state.
// PREVENTS: Dropdown assertions failing.
func TestExpectDropdown(t *testing.T) {
	expVisible := Expectation{Type: "dropdown", Values: map[string]string{"visible": ""}}
	expHidden := Expectation{Type: "dropdown", Values: map[string]string{"hidden": ""}}

	err := CheckExpectation(expVisible, &MockState{showDropdown: true})
	assert.NoError(t, err)

	err = CheckExpectation(expVisible, &MockState{showDropdown: false})
	assert.Error(t, err)

	err = CheckExpectation(expHidden, &MockState{showDropdown: false})
	assert.NoError(t, err)
}

func TestExpectMode(t *testing.T) {
	expEdit := Expectation{Type: "mode", Values: map[string]string{"is": "edit"}}
	expCommand := Expectation{Type: "mode", Values: map[string]string{"is": "command"}}

	err := CheckExpectation(expEdit, &MockState{mode: cli.ModeEdit})
	assert.NoError(t, err)

	err = CheckExpectation(expEdit, &MockState{mode: cli.ModeCommand})
	assert.Error(t, err)

	err = CheckExpectation(expCommand, &MockState{mode: cli.ModeCommand})
	assert.NoError(t, err)

	err = CheckExpectation(expCommand, &MockState{mode: cli.ModeEdit})
	assert.Error(t, err)
}

// TestExpectUnknownType verifies error on unknown expectation.
//
// VALIDATES: Unknown expectation types produce error.
// PREVENTS: Silent failure on typos.
func TestExpectUnknownType(t *testing.T) {
	exp := Expectation{Type: "unknown", Values: map[string]string{"foo": "bar"}}

	err := CheckExpectation(exp, &MockState{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

// --- Mock types for testing ---

// MockState implements the state interface for testing expectations.
type MockState struct {
	contextPath        []string
	completions        []cli.Completion
	ghostText          string
	validationErrors   []cli.ConfigValidationError
	validationWarnings []cli.ConfigValidationError
	dirty              bool
	statusMessage      string
	err                error
	isTemplate         bool
	showDropdown       bool
	workingContent     string
	viewportContent    string
	confirmTimerActive bool
	mode               cli.EditorMode
}

func (m *MockState) ContextPath() []string                           { return m.contextPath }
func (m *MockState) Completions() []cli.Completion                   { return m.completions }
func (m *MockState) GhostText() string                               { return m.ghostText }
func (m *MockState) ValidationErrors() []cli.ConfigValidationError   { return m.validationErrors }
func (m *MockState) ValidationWarnings() []cli.ConfigValidationError { return m.validationWarnings }
func (m *MockState) Dirty() bool                                     { return m.dirty }
func (m *MockState) StatusMessage() string                           { return m.statusMessage }
func (m *MockState) Error() error                                    { return m.err }
func (m *MockState) IsTemplate() bool                                { return m.isTemplate }
func (m *MockState) ShowDropdown() bool                              { return m.showDropdown }
func (m *MockState) WorkingContent() string                          { return m.workingContent }
func (m *MockState) ViewportContent() string                         { return m.viewportContent }
func (m *MockState) ConfirmTimerActive() bool                        { return m.confirmTimerActive }
func (m *MockState) TriggerCompletions()                             {}
func (m *MockState) Mode() cli.EditorMode                            { return m.mode }
func (m *MockState) TmpDir() string                                  { return "" }

// mockError is a simple error type for testing.
type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }
