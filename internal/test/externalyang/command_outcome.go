// Design: docs/architecture/testing/runner-architecture.md -- shared fixture schema publication.
// Related: ../fixture/command_outcome_fixture.go -- the actual Stage-1 provider.

package externalyang

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CommandOutcomePlugin is the declaring process used by the outcome fixtures.
const CommandOutcomePlugin = "answer-outcome-provider"

func init() {
	mustRegisterConfigDeclaration(CommandOutcomeWeb())
}

// OutcomeQuestion is literal request and result data for an observer. Zero
// fields remain unset expectations; observers invent no defaults for them.
type OutcomeQuestion struct {
	Command string
	Outcome string
	Count   int
}

// OutcomeCommand names the command the provider and every observer share.
func OutcomeCommand(outcome, size string) string {
	return "show answer-outcome " + outcome + " " + size
}

// OutcomeQuestions pairs a healthy control and the requested outcome, below
// and above the actual record writer's threshold, in their wire request order.
func OutcomeQuestions(outcome string) []OutcomeQuestion {
	questions := make([]OutcomeQuestion, 0, 4)
	for _, size := range []struct {
		name  string
		count int
	}{{name: "short", count: 2}, {name: "long", count: rpc.AnswerBufferThreshold + 3}} {
		for _, current := range []string{"healthy", outcome} {
			questions = append(questions, OutcomeQuestion{
				Command: OutcomeCommand(current, size.name),
				Outcome: current,
				Count:   size.count,
			})
		}
	}
	return questions
}

// OutcomeToolID is the related-tool identifier shared by the authored module
// and the browser request that selects one of its descriptors.
func OutcomeToolID(question OutcomeQuestion) string {
	return strings.ReplaceAll(strings.TrimPrefix(question.Command, "show answer-outcome "), " ", "-")
}

// CommandOutcomeWeb declares the same module the real provider sends at
// Stage 1. The corpus reads this value without running the provider.
func CommandOutcomeWeb() ConfigDeclaration {
	var module strings.Builder
	module.WriteString(`module ze-answer-outcome-web-conf {
    namespace "urn:ze:test:answer-outcome-web:conf";
    prefix outcome;
    import ze-extensions { prefix ze; }
    description "External command outcome fixture metadata.";
    revision 2026-11-18 { description "Declare related outcome tools."; }
    container answer-outcome-web {
        presence "Enable the command outcome fixture";
        ze:help "External command outcome fixture.";
        description "Related tools dispatch commands declared by the external fixture.";
`)
	declared := make(map[string]bool)
	for _, outcome := range []string{"partial", "failed", "warning"} {
		for _, question := range OutcomeQuestions(outcome) {
			id := OutcomeToolID(question)
			if declared[id] {
				continue
			}
			declared[id] = true
			fmt.Fprintf(&module, "        ze:related 'id=%s; label=\"Outcome %s\"; command=\"%s | raw\"; placement=detail; presentation=modal; class=inspect';\n", id, id, question.Command)
		}
	}
	module.WriteString("    }\n}\n")
	return ConfigDeclaration{
		Plugin: CommandOutcomePlugin,
		Module: Module{
			Name: "ze-answer-outcome-web-conf", Namespace: "urn:ze:test:answer-outcome-web:conf", Text: module.String(),
		},
	}
}
