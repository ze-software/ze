// Design: docs/features/ai-first.md — agent diagnostic contract types

// Package diagnostic provides stable diagnostic records, codes, and
// explanations for Ze's agent-facing tooling surface.
package diagnostic

import (
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
)

// SchemaVersion is the current diagnostic JSON contract version.
const SchemaVersion = 1

// Severity indicates how serious a diagnostic is.
type Severity string

const (
	// SeverityError is a problem that makes the box not ready.
	SeverityError Severity = "error"
	// SeverityWarning is a problem the box keeps running with.
	SeverityWarning Severity = "warning"
	// SeverityInfo is a fact that is not a problem by itself: it never makes the
	// box not ready and is never counted as a warning. A check that reports one
	// leaves the impact to the checks that can judge it.
	SeverityInfo Severity = "info"
)

// FixSafety indicates how safe a proposed repair is.
type FixSafety string

const (
	SafetyFormatOnly          FixSafety = "format-only"
	SafetySectionLocal        FixSafety = "section-local"
	SafetyBehaviorPreserving  FixSafety = "behavior-preserving"
	SafetyAPIChanging         FixSafety = "api-changing"
	SafetyTargetChanging      FixSafety = "target-changing"
	SafetyRequiresHumanReview FixSafety = "requires-human-review"
)

// Diagnostic is a single structured diagnostic record.
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`

	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Length int    `json:"length,omitempty"`

	Expected any `json:"expected,omitempty"`
	Actual   any `json:"actual,omitempty"`

	Help string `json:"help,omitempty"`

	// Fix, FixSafety and Repair answer three different questions.
	//
	// Fix is the command or config statement the operator runs to clear the
	// problem, alone and with no label, as an internal/core/opererr fix holds
	// it ("ze start", "set system name-server 192.0.2.53"). Every surface draws
	// it on a line of its own after the message, and a log line carries it
	// under the "fix" key beside the "code" key. Fix never holds a repair id.
	//
	// Repair names a candidate automated repair an agent may apply (an id and
	// a summary), and FixSafety grades how safe that automated repair is. A
	// diagnostic may carry Fix with neither of them.
	Fix       string    `json:"fix,omitempty"`
	FixSafety FixSafety `json:"fix-safety,omitempty"`
	Repair    *Repair   `json:"repair,omitempty"`
	Related   []Related `json:"related,omitempty"`
}

// Repair describes a candidate repair for an agent.
type Repair struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

// Related is an additional span or fact for a multi-point diagnostic.
type Related struct {
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Length  int    `json:"length,omitempty"`
	Message string `json:"message"`
}

// ValidateResult is the record `show config validation` answers with: the verdict,
// and every diagnostic behind it.
type ValidateResult struct {
	SchemaVersion int          `json:"schema-version"`
	Valid         bool         `json:"valid"`
	Path          string       `json:"path"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Config        any          `json:"config,omitempty"`
}

// ExplainResult is the JSON envelope for ze show diagnostic --json.
type ExplainResult struct {
	Code         string   `json:"code"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Examples     []string `json:"examples,omitempty"`
	RelatedCodes []string `json:"related-codes,omitempty"`
}

// FixPlanResult is the record `ze config fix --plan` answers with.
type FixPlanResult struct {
	SchemaVersion int          `json:"schema-version"`
	Path          string       `json:"path"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}

// NewFixPlan creates a FixPlanResult with the current contract version.
func NewFixPlan(path string, diags []Diagnostic) FixPlanResult {
	if diags == nil {
		diags = []Diagnostic{}
	}
	return FixPlanResult{
		SchemaVersion: SchemaVersion,
		Path:          path,
		Diagnostics:   diags,
	}
}

// NewValidateResult creates a ValidateResult with the current contract version.
func NewValidateResult(path string, valid bool, diags []Diagnostic, cfg any) ValidateResult {
	if diags == nil {
		diags = []Diagnostic{}
	}
	return ValidateResult{
		SchemaVersion: SchemaVersion,
		Valid:         valid,
		Path:          path,
		Diagnostics:   diags,
		Config:        cfg,
	}
}

// DecodeValidateResult reads body as the record `show config validation`
// answers with, for a caller that draws a verdict another process gave: the
// local `ze config validate` reading the running daemon's answer. A body that
// is not JSON of that shape, or that carries another schema version than this
// binary reads, is refused, and the error says what the body was: drawing it
// would print a verdict nobody gave. A nil diagnostics list is answered empty.
func DecodeValidateResult(body []byte) (ValidateResult, error) {
	var result ValidateResult
	if err := json.Unmarshal(body, &result); err != nil {
		return ValidateResult{}, fmt.Errorf("something that is not a validation result (%w)", err)
	}
	if result.SchemaVersion != SchemaVersion {
		return ValidateResult{}, fmt.Errorf("a validation result of schema version %d, and this ze reads version %d",
			result.SchemaVersion, SchemaVersion)
	}
	if result.Diagnostics == nil {
		result.Diagnostics = []Diagnostic{}
	}
	return result, nil
}

// DoctorResult is the JSON envelope for ze doctor --json.
type DoctorResult struct {
	plugin.DataMarker
	SchemaVersion int          `json:"schema-version"`
	Ready         bool         `json:"ready"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}

// NewDoctorResult creates a DoctorResult with the current contract version.
func NewDoctorResult(ready bool, diags []Diagnostic) DoctorResult {
	if diags == nil {
		diags = []Diagnostic{}
	}
	return DoctorResult{
		SchemaVersion: SchemaVersion,
		Ready:         ready,
		Diagnostics:   diags,
	}
}

// SkillEntry describes a bundled skill for ze show skills.
type SkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
