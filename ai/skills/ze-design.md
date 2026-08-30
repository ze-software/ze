---
name: ze-design
description: Design Collaboration
---

# Design Collaboration

Stress-test and refine a plan or design through structured, depth-first decision-making.

See also: `/ze-spec` (write a spec), `/ze-explore` (research a topic)

## Delegation

`ai/rules/planning.md` exempts this skill: it runs in the MAIN THREAD.
Its gates require `AskUserQuestion`, and a subagent can neither hold a dialogue
with the user nor use LSP.

Delegate the work around the gates, not the gates themselves: send the research
and the file reading out to agents (`/ze-explore`, `/ze-audit`), then carry every
decision point here yourself. Verify what those agents report against source
before putting it in front of the user (`ai/rules/evidence.md`).

## Instructions

### Ground Rules

- Before answering ANY design question, review `.claude/rules/` for applicable constraints.
  Every recommendation must be checked against project rules.
  "I probably remember" is not acceptable.
- Recommendations must come from THIS codebase's patterns, not industry defaults.
  Cite the existing code or rule that informed them.
  "Industry best practice" is not a citation.
- One decision per question. State options explicitly with your recommendation,
  so "yes" always means "go with that." Never compound questions.
- Prefer codebase exploration over asking questions you can answer yourself.
- Challenge weak reasoning in both directions. Propose alternatives, don't just poke holes.

### Process

#### Step 1: Load Context

1. Read `.claude/rules/` systematically for constraints relevant to the topic
2. Read the plan/design being discussed
3. Read the pages that already cover the topic, BEFORE any exploration or agent (`ai/rules/documentation.md`). `ai/INDEX.md` names the page. `ai/DOCS-TO-CODE.md` names the files behind it. `ai/CODE-TO-DOCS.md` names a file's pages. A design that contradicts a page it never read is a design nobody can review.
4. Explore relevant codebase context through the lens of those rules

#### Step 2: Map the Decision Space

Present a table of what's already decided vs. what needs deciding.
Each "needs deciding" item becomes a branch to work through.

#### Step 3: Resolve Decisions (depth-first)

For each open decision:

1. State the question and why it matters
2. Review which project rules apply to this decision
3. Recommend an answer grounded in existing code and rules, with citation
4. Surface trade-offs and codebase constraints
5. Wait for input before moving on

After each decision resolves, update the running decision log:

```
## Decision Log

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | ... | A over B | rule: X, existing pattern in Y.go |
| 2 | ... | deferred | needs Z resolved first |
```

#### Step 4: Summarize

When all branches are resolved or explicitly deferred, output the final decision log
in a format ready for spec or implementation.

## Rules

- Never ask a question where "yes" is ambiguous. If there are two options,
  name them and recommend one.
- Never ask compound questions. One decision at a time. If a question
  can be split into two, split it.
- Never recommend something that contradicts a project rule without
  naming the rule and explaining why it should be reconsidered.
- When you catch yourself reaching for a generic pattern, stop.
  Search the codebase for how this project handles the same situation.
