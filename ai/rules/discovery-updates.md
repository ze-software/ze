# Discovery Updates

**When:** adding or changing anything future agents must discover: a feature, command, tool, gate, test type, or runtime dependency
**Severity:** blocking

## Directives

A change that adds or changes something future agents need to use,
verify, document, or avoid MUST update the discovery path in the same work.

## Trigger

Apply this rule when adding or changing any of these:

| Change | Why agents need it |
|--------|--------------------|
| User-facing feature | Agents must know the feature exists and where users configure or invoke it |
| CLI command, RPC, MCP tool, YANG command, or API contract | Agents must discover the command shape, JSON contract, and wiring |
| Developer tool, script, make target, generator, or inventory command | Agents must know the tool exists before reimplementing it |
| Self-check, verification gate, hook, lint, or doc validator | Agents must run the right check and understand failures |
| Test runner, test format, fixture pattern, or required test category | Agents must place tests in the right suite and run the right target |
| Runtime dependency or readiness condition | Agents must verify the host with `ze doctor` before starting Ze |
| Structural decision, repeated gotcha, or workflow change | Agents must find it through the learned index or a rule before repeating the mistake |
| New BGP family, SAFI, or capability | Agents must update migration schema, route converter, bridge, and compat tests (`ai/patterns/bgp-family.md`) |
| RFC-level protocol behavior added, changed, or newly proven | The standards ledger drives user and design decisions; a stale RFC status misleads both |

Private refactors with no new surface still trigger this rule when they change a
pattern future work must follow.

## Required Discovery Artifacts

Update every row that applies:

| What changed | Required update |
|--------------|-----------------|
| User-facing behavior | Specific file under `docs/`, with source anchors per `ai/rules/documentation.md` |
| RFC support status (protocol behavior implemented, changed, or newly proven) | The matching `docs/features/rfc-status.md` row (Status, Implemented coverage, Remaining) with a source anchor to the producing `file:line`; reconcile `docs/comparison.md` and `docs/features.md` when the support level changes |
| Agent-facing command or contract | `docs/features/ai-first.md`, `docs/guide/mcp/overview.md` if MCP-visible, and `ai/rules/agent-tooling.md` if workflow changes |
| CLI command grammar or command availability | `ai/rules/cli-grammar.md` or `ai/rules/pipe-completeness.md`, plus command validation docs if needed |
| New tool or make target | `ai/INDEX.md` Dev Tools or keyword map, plus the owning `docs/contributing/` or `docs/architecture/testing/` page |
| New verification gate or hook | `ai/rules/hook-mapping.md`, the rule enforced by the hook, and the relevant make-target documentation |
| New doc or inventory checker | `docs/contributing/documentation-testing.md`, `mk/inventory.mk` quick reference, and `ai/rules/documentation.md` if policy changed |
| New test runner or format | `ai/rules/testing.md`, `ai/patterns/functional-test.md` if `.ci`, and the relevant `docs/architecture/testing/` page |
| New runtime dependency | `ai/rules/doctor-checks.md`, diagnostic code registration, and a `ze doctor` unit plus functional test |
| New registration or generated inventory | `ai/rules/derive-not-hardcode.md`, `ai/patterns/registration.md`, and registry-backed inventory checks |
| Structural decision or recurring trap | `plan/learned/NNN-*.md`; add `ai/LEARNED-INDEX.md` when the lesson is structural, not task-only |
| New task category or search keyword | `ai/INDEX.md` (task navigation + keyword map) |

Do not create an isolated rule or doc page that no existing navigation path links
to. A rule that agents cannot discover is not a rule.

## Mechanical Checklist

Before implementation is complete, answer these in the spec, review notes, or
handoff:

1. **Where would an agent look first?** Add or update the `ai/INDEX.md` keyword
   row, `ai/INDEX.md` task row, or both.
2. **What rule prevents regression?** Update the narrowest existing rule. Create a
   new `ai/rules/*.md` only when no existing rule owns the behavior.
3. **What source of truth prevents drift?** Use a registry, generated inventory,
   YANG schema, or live binary output. Do not copy static lists.
4. **What verification proves it?** Name the make target, unit test, functional
   test, hook, or doc validator that catches drift.
5. **What docs explain usage?** Name the exact file and section. Add source
   anchors for factual `docs/` claims.
6. **What learned record preserves the decision?** Update `ai/LEARNED-INDEX.md`
   if the learned summary changes future design choices.

## Current Discovery Surfaces

Use these before inventing a new mechanism:

| Need | Existing surface |
|------|------------------|
| Changed-file-aware wiring, doc, command, and inventory gate | `make ze-verify-wiring-docs` |
| Documentation drift and YANG command contracts | `make ze-doc-test` |
| Source-to-document reverse index | `make ze-doc-index`; read `ai/CODE-TO-DOCS.md` |
| RFC MUST requirement to enforcing-test coverage (which tests prove each requirement, plus the backlog) | `make ze-rfc-index`; read `ai/RFC-REQUIREMENTS.md` (the generated two-way ledger; coverage gated by `make ze-rfc-check`, staleness by `make ze-doc-test`) |
| What each package does ("what does what") | `make ze-discovery-index`; read `ai/PACKAGE-MAP.md` |
| Which `.go` files implement a design doc | read `ai/DOCS-TO-CODE.md` (inverse of `// Design:`) |
| Which tests enforce an RFC MUST (and the un-enrolled backlog) | read `ai/RFC-REQUIREMENTS.md` (generated by `make ze-rfc-index`; freshness gated by `make ze-rfc-check` and `make ze-doc-test`) |
| Every learned summary by number (complete) | read `ai/LEARNED-FULL-INDEX.md`; curated by topic: `ai/LEARNED-INDEX.md` |
| How data flows through a subsystem | read `ai/digests/<subsystem>.md` (living, hand-maintained flow digests; `ai/digests/README.md` lists them); anchors validated by `make ze-digest-check` |
| Plugin, command, YANG, and test inventory | `make ze-inventory`, `make ze-inventory-json` |
| Command inventory | `make ze-command-list`, `make ze-command-list-json` |
| Spec progress | `make ze-spec-status`, `make ze-spec-status-json` |
| Generated plugin imports | `make ze-plugin-imports-check` |
| Runtime readiness | `ze doctor --json` and `ze explain <diagnostic-code>` |

If a new feature cannot be found from one of those surfaces or from
`ai/INDEX.md`, add the missing discovery link before claiming completion.
