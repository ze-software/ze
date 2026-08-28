// Design: docs/architecture/core-design.md -- deterministic repository generation

package repository

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/aisync"
	"github.com/ze-software/ze/internal/le/archmap"
	"github.com/ze-software/ze/internal/le/discoveryindex"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/pluginimports"
	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/rules"
	"github.com/ze-software/ze/internal/le/sitefacts"
	"github.com/ze-software/ze/internal/le/testhealth"
	"github.com/ze-software/ze/internal/le/vendorweb"
	"github.com/ze-software/ze/internal/le/webassets"
	"github.com/ze-software/ze/internal/le/yangglue"
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
	{area: "plugin-imports", verb: "write", answer: pluginimports.Answer},
	{area: "yang-glue", verb: "write", answer: yangglue.Answer},
	{area: "feature-tags", verb: "write", answer: featuretags.Answer},
	{area: "web-assets", verb: "write", answer: webassets.Answer},
	{area: "vendor-web", verb: "sync", answer: vendorweb.Answer},
	{area: "rules", verb: "render-update", answer: rules.Answer},
	{area: "rules", verb: "condensed-update", answer: rules.Answer},
	{area: "rules", verb: "index-update", answer: rules.Answer},
	{area: "rfc", verb: "index-update", answer: rfc.Answer},
	{area: "arch-map", verb: "update", answer: archmap.Answer},
	{area: "discovery-index", verb: "update", answer: discoveryindex.Answer},
	{area: "docs-to-code", verb: "update", answer: docstocode.Answer},
	{area: "docs-to-code", verb: "index-update", answer: docstocode.Answer},
	{area: "test-health", verb: "update", answer: testhealth.Answer},
	{area: "site-facts", verb: "update", answer: sitefacts.Answer},
	{area: "ai", verb: "skills-sync", answer: aisync.Answer},
}

var generationChecks = []generationAction{
	{area: "plugin-imports", verb: "check", answer: pluginimports.Answer},
	{area: "yang-glue", verb: "check", answer: yangglue.Answer},
	{area: "feature-tags", verb: "check", answer: featuretags.Answer},
	{area: "web-assets", verb: "check", answer: webassets.Answer},
	{area: "vendor-web", verb: "check", answer: vendorweb.Answer},
	{area: "rules", verb: "lint", answer: rules.Answer},
	{area: "rules", verb: "render-check", answer: rules.Answer},
	{area: "rules", verb: "points-roundtrip-check", answer: rules.Answer},
	{area: "rules", verb: "condensed-check", answer: rules.Answer},
	{area: "rules", verb: "index-check", answer: rules.Answer},
	{area: "rfc", verb: "check", answer: rfc.Answer},
	{area: "arch-map", verb: "check", answer: archmap.Answer},
	{area: "discovery-index", verb: "check", answer: discoveryindex.Answer},
	{area: "docs-to-code", verb: "check", answer: docstocode.Answer},
	{area: "docs-to-code", verb: "index-check", answer: docstocode.Answer},
	{area: "test-health", verb: "check", answer: testhealth.Answer},
	{area: "site-facts", verb: "check", answer: sitefacts.Answer},
	{area: "ai", verb: "sync-check", answer: aisync.Answer},
}

func runGenerate() (any, int) {
	return runGeneration("write", generationActions)
}

func runGeneratedCheck() (any, int) {
	return runGeneration("check", generationChecks)
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
