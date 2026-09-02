// Design: docs/architecture/core-design.md -- deterministic repository generation

package repository

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/ai"
	"github.com/ze-software/ze/internal/le/archmap"
	"github.com/ze-software/ze/internal/le/discoveryindex"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/featuretags"
	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/rules"
	sitefacts "github.com/ze-software/ze/internal/le/site/facts"
	"github.com/ze-software/ze/internal/le/testhealth"
	"github.com/ze-software/ze/internal/le/vendorweb"
	"github.com/ze-software/ze/internal/le/webassets"
	yangglue "github.com/ze-software/ze/internal/le/yang/glue"
)

// GenerationStep is one native action in the repository generation sequence.
type GenerationStep struct {
	Area string `json:"area"`
	Verb string `json:"verb"`
	Code int    `json:"code"`
}

// generationReport is the complete deterministic generation or checking run.
type generationReport struct {
	Mode  string           `json:"mode"`
	Steps []GenerationStep `json:"steps"`
}

// Text renders the sequence and its result without hiding a failed step.
func (r generationReport) Text() string {
	var tb textbuf.Buffer
	for _, step := range r.Steps {
		tb.Str(step.Area).Byte(' ').Str(step.Verb).Str(": ")
		if step.Code == 0 {
			tb.Str("ok")
		} else {
			tb.Str("exit ").Int(int64(step.Code))
		}
		tb.Byte('\n')
	}
	return tb.String()
}

type generationAction struct {
	area   string
	verb   string
	answer func([]string) (any, int)
}

var generationActions = []generationAction{
	{area: "plugin imports", verb: verbWrite, answer: pluginimports.Answer},
	{area: "yang glue", verb: verbWrite, answer: yangglue.Answer},
	{area: "feature-tags", verb: verbWrite, answer: featuretags.Answer},
	{area: "web-assets", verb: verbWrite, answer: webassets.Answer},
	{area: "vendor-web", verb: "sync", answer: vendorweb.Answer},
	{area: areaRules, verb: "render-update", answer: rules.Answer},
	{area: areaRules, verb: "condensed-update", answer: rules.Answer},
	{area: areaRules, verb: verbIndexUpdate, answer: rules.Answer},
	{area: "rfc", verb: verbIndexUpdate, answer: rfc.Answer},
	{area: "arch-map", verb: verbUpdate, answer: archmap.Answer},
	{area: "discovery-index", verb: verbUpdate, answer: discoveryindex.Answer},
	{area: areaDocsToCode, verb: verbUpdate, answer: docstocode.Answer},
	{area: areaDocsToCode, verb: verbIndexUpdate, answer: docstocode.Answer},
	{area: "test-health", verb: verbUpdate, answer: testhealth.Answer},
	{area: "site facts", verb: verbUpdate, answer: sitefacts.Answer},
	{area: "ai", verb: "skills-sync", answer: ai.Answer},
}

var generationChecks = []generationAction{
	{area: "plugin imports", verb: verbCheck, answer: pluginimports.Answer},
	{area: "yang glue", verb: verbCheck, answer: yangglue.Answer},
	{area: "feature-tags", verb: verbCheck, answer: featuretags.Answer},
	{area: "web-assets", verb: verbCheck, answer: webassets.Answer},
	{area: "vendor-web", verb: verbCheck, answer: vendorweb.Answer},
	{area: areaRules, verb: "lint", answer: rules.Answer},
	{area: areaRules, verb: "render-check", answer: rules.Answer},
	{area: areaRules, verb: "points-roundtrip-check", answer: rules.Answer},
	{area: areaRules, verb: "condensed-check", answer: rules.Answer},
	{area: areaRules, verb: "index-check", answer: rules.Answer},
	{area: "rfc", verb: verbCheck, answer: rfc.Answer},
	{area: "arch-map", verb: verbCheck, answer: archmap.Answer},
	{area: "discovery-index", verb: verbCheck, answer: discoveryindex.Answer},
	{area: areaDocsToCode, verb: verbCheck, answer: docstocode.Answer},
	{area: areaDocsToCode, verb: "index-check", answer: docstocode.Answer},
	{area: "test-health", verb: verbCheck, answer: testhealth.Answer},
	{area: "site facts", verb: verbCheck, answer: sitefacts.Answer},
	{area: "ai", verb: "sync-check", answer: ai.Answer},
}

func runGenerate() (any, int) {
	return runGeneration(verbWrite, generationActions)
}

func runGeneratedCheck() (any, int) {
	return runGeneration(verbCheck, generationChecks)
}

func runGeneration(mode string, actions []generationAction) (any, int) {
	report := generationReport{Mode: mode, Steps: make([]GenerationStep, 0, len(actions))}
	code := 0
	for _, action := range actions {
		_, actionCode := action.answer([]string{action.verb})
		report.Steps = append(report.Steps, GenerationStep{Area: action.area, Verb: action.verb, Code: actionCode})
		if actionCode != 0 {
			code = 1
		}
	}
	return report, code
}

// The generator verbs and the two areas that carry more than one of them.
const (
	verbCheck       = "check"
	verbWrite       = "write"
	verbUpdate      = "update"
	verbIndexUpdate = "index-update"
	areaRules       = "rules"
	areaDocsToCode  = "docs-to-code"
)
