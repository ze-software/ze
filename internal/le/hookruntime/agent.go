// Design: docs/architecture/core-design.md -- native delegation hook policy
package hookruntime

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/le/spec/session"
)

type skillTrigger struct {
	skill   string
	pattern *regexp.Regexp
}

var skillTriggers = []skillTrigger{
	{"/ze-review", regexp.MustCompile(`(?i)\b(review|critique|audit)\b.{0,60}\b(diff|change|commit|branch|implementation|code|pr)\b|\bfind (bugs|issues|defects|problems)\b|\b(blocker|adversarial(ly)? (review|verify))\b`)},
	{"/ze-review-spec", regexp.MustCompile(`(?i)\breview\b.{0,40}\bagainst\b.{0,20}\bspec\b|\bacceptance criteri\w+\b.{0,40}\b(met|verified|implemented)\b`)},
	{"/ze-debug", regexp.MustCompile(`(?i)\b(debug|diagnose|root[- ]cause)\b.{0,60}\b(failure|failing|red|test|gate|hang|crash|flake)\b`)},
	{"/ze-explore", regexp.MustCompile(`(?i)\b(survey|explore|investigate|research|map out|find (out |every |all )?(where|which|how))\b`)},
	{"/ze-audit", regexp.MustCompile(`(?i)\baudit\b.{0,60}\b(spec|requirement|implementation|coverage)\b`)},
	{"/ze-implement", regexp.MustCompile(`(?i)\bimplement\b.{0,60}\b(spec|feature|ac-\d)\b`)},
	{"/ze-hunt", regexp.MustCompile(`(?i)\bhunt\b.{0,40}\b(bug|class|pattern)\b`)},
}

var (
	skillReference     = regexp.MustCompile(`/ze-[a-z0-9-]+`)
	implementationVerb = regexp.MustCompile(`(?i)\b(apply|fix|implement|update|edit|rewrite|refactor|rename|migrate|add|write|remove|delete|port|wire)\b`)
	writesGo           = regexp.MustCompile(`(?i)\bgo\b|\.go\b|./le changed scope|gofmt|gopls`)
	briefWork          = regexp.MustCompile(`(?i)\b(fix|implement|write|add|refactor|migrate|wire|rewrite)\b`)
	styleGuide         = regexp.MustCompile(`(?i)docs/contributing/ze-(?:go-)?style\.md|\bze-(?:go-)?style\b`)
)

// ze point: planning/work-phases/run-every-review-on-opus-5
// agentReviewModel refuses a review agent that does not run on Opus 5.
func agentReviewModel(ctx context) *verdict {
	prompt := stringInput(ctx.input, "prompt")
	refusal, note := reviewModelRefusal(ctx, prompt)
	if refusal != "" {
		return &verdict{2, refusal}
	}
	if note != "" {
		return &verdict{0, note}
	}
	return nil
}

// ze point: cli/agent-tooling-contract/use-the-skill-instead-of-a-raw-agent
// agentSkill refuses a hand-written prompt that a ze-* skill already covers.
func agentSkill(ctx context) *verdict {
	prompt := stringInput(ctx.input, "prompt")
	skill, matched := coveredSkill(ctx.root, prompt)
	if skill == "" {
		return nil
	}
	return &verdict{2, "❌ Blocked: a ze-* skill covers this agent's task (ai/rules/cli.md).\n" +
		"  The prompt asks for: \"" + matched + "\"\n" +
		"  Use " + skill + " instead of a hand-written prompt. The skill carries the\n" +
		"  workflow, the gates and the report format that a raw agent improvises\n" +
		"  and usually drops.\n" +
		"  Invoke it with the Skill tool, or name " + skill + " in the agent prompt so\n" +
		"  the agent follows it. Delegation itself needs no permission."}
}

// ze point: go-standards/directives/read-the-ze-style-guide-before-go-design-or-review
// agentStyleGuide warns when a brief will produce Go and names no style guide.
func agentStyleGuide(ctx context) *verdict {
	prompt := stringInput(ctx.input, "prompt")
	if !writesGo.MatchString(prompt) || !briefWork.MatchString(first(prompt, 400)) || styleGuide.MatchString(prompt) {
		return nil
	}
	return &verdict{1, "WARN: this brief will produce Go and never names docs/contributing/ze-go-style.md\n" +
		"  The guide is read in full BEFORE any code, every session, and a\n" +
		"  subagent gets it from your brief or not at all.\n" +
		"  → Name it as a PRECONDITION in the brief's opening, never under a\n" +
		"    'before you finish' heading.\n" +
		"  See ai/rules/planning.md, briefing a subagent."}
}

func first(text string, length int) string {
	if len(text) <= length {
		return text
	}
	return text[:length]
}

func knownSkills(root string) map[string]bool {
	known := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(root, "ai", "skills"))
	if err != nil {
		return known
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "ze-") && strings.HasSuffix(name, ".md") {
			known["/"+strings.TrimSuffix(name, ".md")] = true
		}
	}
	return known
}

func namesKnownSkill(root, prompt string) bool {
	known := knownSkills(root)
	for _, reference := range skillReference.FindAllString(prompt, -1) {
		if known[reference] {
			return true
		}
	}
	return false
}

func coveredSkill(root, prompt string) (string, string) {
	if prompt == "" || namesKnownSkill(root, prompt) {
		return "", ""
	}
	for _, trigger := range skillTriggers {
		location := trigger.pattern.FindStringIndex(prompt)
		if location != nil {
			return trigger.skill, strings.TrimSpace(prompt[location[0]:location[1]])
		}
	}
	return "", ""
}

func isReviewWork(root, prompt string) bool {
	if implementationVerb.MatchString(first(prompt, 160)) {
		return false
	}
	reviewSkills := map[string]bool{"/ze-review": true, "/ze-review-spec": true, "/ze-review-deep": true, "/ze-audit": true, "/ze-close": true}
	for _, reference := range skillReference.FindAllString(prompt, -1) {
		if reviewSkills[strings.ToLower(reference)] {
			return true
		}
	}
	skill, _ := coveredSkill(root, prompt)
	return reviewSkills[skill]
}

func reviewAck(ctx context) bool {
	id, present := payloadSessionID(ctx.payload)
	if !present {
		id = resolvedSessionID(ctx)
	}
	if id == "" {
		return false
	}
	body, err := os.ReadFile(filepath.Join(ctx.root, "tmp", "session", ".model-ack-"+id)) //nolint:gosec // a session marker under the checkout tmp directory
	return err == nil && len(strings.TrimSpace(string(body))) >= 10
}

func reviewModelRefusal(ctx context, prompt string) (string, string) {
	if !isReviewWork(ctx.root, prompt) || reviewAck(ctx) {
		return "", ""
	}
	model := specsession.RunningModel(ctx.transcript)
	if ctx.transcript == "" {
		model = specsession.CurrentModel(ctx.root)
	}
	if model == "" {
		return "", "note: could not determine the running model, so the review-model boundary is UNCHECKED here (ai/rules/planning.md)"
	}
	if specsession.IsReviewTier(model) {
		return "", ""
	}
	return "❌ Blocked: review runs on Opus 5, and this session is on " + model + "\n" +
		"  (ai/rules/planning.md).\n" +
		"  A subagent inherits the PHASE, not the task shape, so spawning a\n" +
		"  reviewer from an implementation session still reviews on the wrong\n" +
		"  model. Say so and stop, so the operator can switch or start a review session.\n" +
		"  If the operator decides otherwise, their reason goes in\n" +
		"  tmp/session/.model-ack-<sid>.", ""
}
