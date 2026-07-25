// Design: docs/features/ai-first.md — agent diagnostic contract types

// Package diagnostic provides stable diagnostic records, codes, and
// explanations for Ze's agent-facing tooling surface.
package diagnostic

import "github.com/ze-software/ze/internal/component/plugin"

// SchemaVersion is the current diagnostic JSON contract version.
const SchemaVersion = 1

// Severity indicates how serious a diagnostic is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
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

	Help      string    `json:"help,omitempty"`
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

// ValidateResult is the top-level JSON envelope for config validation.
type ValidateResult struct {
	SchemaVersion int          `json:"schema-version"`
	Valid         bool         `json:"valid"`
	Path          string       `json:"path"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Config        any          `json:"config,omitempty"`
}

// ExplainResult is the JSON envelope for ze explain --json.
type ExplainResult struct {
	Code         string   `json:"code"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Examples     []string `json:"examples,omitempty"`
	RelatedCodes []string `json:"related-codes,omitempty"`
}

// FixPlanResult is the JSON envelope for ze config fix --plan --json.
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

// SkillEntry describes a bundled skill for ze skills list.
type SkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
