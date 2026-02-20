// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure

package testing

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config/editor"
)

// State interface defines what can be queried from the editor model for assertions.
type State interface {
	ContextPath() []string
	Completions() []editor.Completion
	GhostText() string
	ValidationErrors() []editor.ConfigValidationError
	Dirty() bool
	StatusMessage() string
	Error() error
	IsTemplate() bool
	ShowDropdown() bool
	WorkingContent() string
	ViewportContent() string
	ConfirmTimerActive() bool
	TriggerCompletions()
}

// validExpectationTypes lists all recognized expectation types.
var validExpectationTypes = map[string]func(Expectation, State) error{
	"context":    checkContext,
	"dirty":      checkDirty,
	"error":      checkError,
	"completion": checkCompletion,
	"ghost":      checkGhost,
	"errors":     checkErrors,
	"warnings":   checkWarnings,
	"content":    checkContent,
	"status":     checkStatus,
	"template":   checkTemplate,
	"dropdown":   checkDropdown,
	"prompt":     checkPrompt,
	"viewport":   checkViewport,
	"timer":      checkTimer,
}

// CheckExpectation verifies a single expectation against the current state.
// Returns nil if expectation passes, error describing the failure otherwise.
func CheckExpectation(exp Expectation, state State) error {
	handler, ok := validExpectationTypes[exp.Type]
	if !ok {
		return fmt.Errorf("unknown expectation type: %s", exp.Type)
	}
	return handler(exp, state)
}

// checkContext verifies context path expectations.
func checkContext(exp Expectation, state State) error {
	if _, hasRoot := exp.Values["root"]; hasRoot {
		if len(state.ContextPath()) > 0 {
			return fmt.Errorf("expected context:root, got path: %v", state.ContextPath())
		}
		return nil
	}

	if expectedPath, hasPath := exp.Values["path"]; hasPath {
		actualPath := strings.Join(state.ContextPath(), ".")
		if actualPath != expectedPath {
			return fmt.Errorf("expected context path %q, got %q", expectedPath, actualPath)
		}
		return nil
	}

	return fmt.Errorf("context expectation requires 'root' or 'path' key")
}

// checkDirty verifies dirty flag expectations.
func checkDirty(exp Expectation, state State) error {
	if _, hasTrue := exp.Values["true"]; hasTrue {
		if !state.Dirty() {
			return fmt.Errorf("expected dirty:true, got false")
		}
		return nil
	}

	if _, hasFalse := exp.Values["false"]; hasFalse {
		if state.Dirty() {
			return fmt.Errorf("expected dirty:false, got true")
		}
		return nil
	}

	return fmt.Errorf("dirty expectation requires 'true' or 'false' key")
}

// checkError verifies command error expectations.
func checkError(exp Expectation, state State) error {
	if _, hasNone := exp.Values["none"]; hasNone {
		if state.Error() != nil {
			return fmt.Errorf("expected error:none, got: %s", state.Error().Error())
		}
		return nil
	}

	if expected, hasContains := exp.Values["contains"]; hasContains {
		if state.Error() == nil {
			return fmt.Errorf("expected error containing %q, got no error", expected)
		}
		if !strings.Contains(state.Error().Error(), expected) {
			return fmt.Errorf("expected error containing %q, got: %s", expected, state.Error().Error())
		}
		return nil
	}

	return fmt.Errorf("error expectation requires 'none' or 'contains' key")
}

// checkCompletion verifies completion list expectations.
func checkCompletion(exp Expectation, state State) error {
	comps := state.Completions()

	if _, hasEmpty := exp.Values["empty"]; hasEmpty {
		if len(comps) > 0 {
			return fmt.Errorf("expected completion:empty, got %d completions", len(comps))
		}
		return nil
	}

	if expected, hasCount := exp.Values["count"]; hasCount {
		expectedCount, err := strconv.Atoi(expected)
		if err != nil {
			return fmt.Errorf("invalid count value: %s", expected)
		}
		if len(comps) != expectedCount {
			return fmt.Errorf("expected %d completions, got %d", expectedCount, len(comps))
		}
		return nil
	}

	if expected, hasContains := exp.Values["contains"]; hasContains {
		expectedItems := strings.Split(expected, ",")
		compTexts := make(map[string]bool)
		for _, c := range comps {
			compTexts[c.Text] = true
		}

		var missing []string
		for _, item := range expectedItems {
			if !compTexts[item] {
				missing = append(missing, item)
			}
		}

		if len(missing) > 0 {
			return fmt.Errorf("completion missing items: %v (have: %v)", missing, completionTexts(comps))
		}
		return nil
	}

	if expected, hasExact := exp.Values["exact"]; hasExact {
		expectedItems := strings.Split(expected, ",")
		if len(comps) != len(expectedItems) {
			return fmt.Errorf("expected exactly %d completions, got %d", len(expectedItems), len(comps))
		}

		compTexts := make(map[string]bool)
		for _, c := range comps {
			compTexts[c.Text] = true
		}

		for _, item := range expectedItems {
			if !compTexts[item] {
				return fmt.Errorf("completion missing item: %s", item)
			}
		}
		return nil
	}

	return fmt.Errorf("completion expectation requires 'empty', 'count', 'contains', or 'exact' key")
}

// completionTexts extracts text values from completions for error messages.
func completionTexts(comps []editor.Completion) []string {
	texts := make([]string, len(comps))
	for i, c := range comps {
		texts[i] = c.Text
	}
	return texts
}

// checkGhost verifies ghost text expectations.
func checkGhost(exp Expectation, state State) error {
	if _, hasEmpty := exp.Values["empty"]; hasEmpty {
		if state.GhostText() != "" {
			return fmt.Errorf("expected ghost:empty, got %q", state.GhostText())
		}
		return nil
	}

	if expected, hasText := exp.Values["text"]; hasText {
		if state.GhostText() != expected {
			return fmt.Errorf("expected ghost text %q, got %q", expected, state.GhostText())
		}
		return nil
	}

	return fmt.Errorf("ghost expectation requires 'empty' or 'text' key")
}

// checkErrors verifies validation error expectations.
func checkErrors(exp Expectation, state State) error {
	errs := state.ValidationErrors()

	if expected, hasCount := exp.Values["count"]; hasCount {
		expectedCount, err := strconv.Atoi(expected)
		if err != nil {
			return fmt.Errorf("invalid count value: %s", expected)
		}
		if len(errs) != expectedCount {
			return fmt.Errorf("expected %d validation errors, got %d", expectedCount, len(errs))
		}
		return nil
	}

	if expected, hasContains := exp.Values["contains"]; hasContains {
		for _, e := range errs {
			if strings.Contains(e.Message, expected) {
				return nil
			}
		}
		return fmt.Errorf("no validation error contains %q", expected)
	}

	return fmt.Errorf("errors expectation requires 'count' or 'contains' key")
}

// checkWarnings verifies validation warning expectations.
func checkWarnings(_ Expectation, _ State) error {
	// Note: State interface would need ValidationWarnings() method
	// For now, this is a placeholder
	return fmt.Errorf("warnings expectation not yet implemented")
}

// checkContent verifies content expectations.
// Checks both working content (config) and viewport content (command output).
func checkContent(exp Expectation, state State) error {
	workingContent := state.WorkingContent()
	viewportContent := state.ViewportContent()

	if expected, hasContains := exp.Values["contains"]; hasContains {
		// Check both working content and viewport content
		if strings.Contains(workingContent, expected) || strings.Contains(viewportContent, expected) {
			return nil
		}
		return fmt.Errorf("content does not contain %q", expected)
	}

	if expected, hasNotContains := exp.Values["not-contains"]; hasNotContains {
		// For not-contains, check both don't contain it
		if strings.Contains(workingContent, expected) || strings.Contains(viewportContent, expected) {
			return fmt.Errorf("content should not contain %q", expected)
		}
		return nil
	}

	if expected, hasLines := exp.Values["lines"]; hasLines {
		expectedLines, err := strconv.Atoi(expected)
		if err != nil {
			return fmt.Errorf("invalid lines value: %s", expected)
		}
		// Use viewport content for line count (matches displayed output)
		content := viewportContent
		if content == "" {
			content = workingContent
		}
		actualLines := len(strings.Split(content, "\n"))
		if actualLines != expectedLines {
			return fmt.Errorf("expected %d lines, got %d", expectedLines, actualLines)
		}
		return nil
	}

	return fmt.Errorf("content expectation requires 'contains', 'not-contains', or 'lines' key")
}

// checkStatus verifies status message expectations.
func checkStatus(exp Expectation, state State) error {
	status := state.StatusMessage()

	if _, hasEmpty := exp.Values["empty"]; hasEmpty {
		if status != "" {
			return fmt.Errorf("expected status:empty, got %q", status)
		}
		return nil
	}

	if expected, hasContains := exp.Values["contains"]; hasContains {
		if !strings.Contains(status, expected) {
			return fmt.Errorf("status message does not contain %q, got %q", expected, status)
		}
		return nil
	}

	return fmt.Errorf("status expectation requires 'empty' or 'contains' key")
}

// checkTemplate verifies template mode expectations.
func checkTemplate(exp Expectation, state State) error {
	if _, hasTrue := exp.Values["true"]; hasTrue {
		if !state.IsTemplate() {
			return fmt.Errorf("expected template:true, got false")
		}
		return nil
	}

	if _, hasFalse := exp.Values["false"]; hasFalse {
		if state.IsTemplate() {
			return fmt.Errorf("expected template:false, got true")
		}
		return nil
	}

	return fmt.Errorf("template expectation requires 'true' or 'false' key")
}

// checkDropdown verifies dropdown visibility expectations.
func checkDropdown(exp Expectation, state State) error {
	if _, hasVisible := exp.Values["visible"]; hasVisible {
		if !state.ShowDropdown() {
			return fmt.Errorf("expected dropdown:visible, got hidden")
		}
		return nil
	}

	if _, hasHidden := exp.Values["hidden"]; hasHidden {
		if state.ShowDropdown() {
			return fmt.Errorf("expected dropdown:hidden, got visible")
		}
		return nil
	}

	return fmt.Errorf("dropdown expectation requires 'visible' or 'hidden' key")
}

// checkPrompt verifies prompt text expectations.
func checkPrompt(exp Expectation, state State) error {
	// Prompt is derived from context path - build expected prompt
	// For now, just check context-based prompt
	if expected, hasContains := exp.Values["contains"]; hasContains {
		// Build prompt from context
		path := state.ContextPath()
		var prompt string
		if len(path) == 0 {
			prompt = "ze#"
		} else {
			prompt = fmt.Sprintf("ze[%s]#", strings.Join(path, " "))
		}

		if !strings.Contains(prompt, expected) {
			return fmt.Errorf("prompt does not contain %q, got %q", expected, prompt)
		}
		return nil
	}

	return fmt.Errorf("prompt expectation requires 'contains' key")
}

// checkViewport verifies viewport content expectations.
func checkViewport(exp Expectation, state State) error {
	// Viewport content is what's displayed (may be command output or filtered config)
	content := state.ViewportContent()

	if expected, hasContains := exp.Values["contains"]; hasContains {
		if !strings.Contains(content, expected) {
			return fmt.Errorf("viewport does not contain %q", expected)
		}
		return nil
	}

	if expected, hasNotContains := exp.Values["not-contains"]; hasNotContains {
		if strings.Contains(content, expected) {
			return fmt.Errorf("viewport should not contain %q", expected)
		}
		return nil
	}

	return fmt.Errorf("viewport expectation requires 'contains' or 'not-contains' key")
}

// checkTimer verifies commit confirm timer state.
func checkTimer(exp Expectation, state State) error {
	if _, hasActive := exp.Values["active"]; hasActive {
		if !state.ConfirmTimerActive() {
			return fmt.Errorf("expected timer:active, got inactive")
		}
		return nil
	}

	if _, hasInactive := exp.Values["inactive"]; hasInactive {
		if state.ConfirmTimerActive() {
			return fmt.Errorf("expected timer:inactive, got active")
		}
		return nil
	}

	return fmt.Errorf("timer expectation requires 'active' or 'inactive' key")
}

// CheckExpectations verifies multiple expectations against state.
// Returns the first error encountered, or nil if all pass.
func CheckExpectations(expectations []Expectation, state State) error {
	for i, exp := range expectations {
		if err := CheckExpectation(exp, state); err != nil {
			return fmt.Errorf("expectation %d (%s): %w", i+1, exp.Type, err)
		}
	}
	return nil
}
