---
kind: table
level:
stage:
---
| What changed | Required update |
|--------------|-----------------|
| User-facing behavior | Specific file under `docs/`, with source anchors per `ai/rules/writing.md` |
| RFC support status (protocol behavior implemented, changed, or newly proven) | The matching `docs/features/rfc-status.md` row (Status, Implemented coverage, Remaining) with a source anchor to the producing `file:line`; reconcile `docs/comparison.md` and `docs/features.md` when the support level changes |
| Agent-facing command or contract | `docs/features/ai-first.md`, `docs/guide/mcp/overview.md` if MCP-visible, and `ai/rules/cli.md` if workflow changes |
| CLI command grammar or command availability | `ai/rules/cli.md` or `ai/rules/cli.md`, plus command validation docs if needed |
| New tool or make target | `ai/INDEX.md` Dev Tools or keyword map, plus the owning `docs/contributing/` or `docs/architecture/testing/` page |
| New verification gate or hook | The "Hook-to-Rule Mapping" section below, the rule enforced by the hook, and the relevant make-target documentation |
| New doc or inventory checker | `docs/contributing/documentation-testing.md`, `mk/inventory.mk` quick reference, and `ai/rules/writing.md` if policy changed |
| New test runner or format | `ai/rules/testing.md`, `ai/patterns/functional-test.md` if `.ci`, and the relevant `docs/architecture/testing/` page |
| New runtime dependency | The "Doctor Checks" section below, diagnostic code registration, and a `ze doctor` unit plus functional test |
| New registration or generated inventory | `ai/rules/evidence.md`, `ai/patterns/registration.md`, and registry-backed inventory checks |
| Recurring trap | `plan/journal/<class>.md` -- one row per occurrence; recurrence is the row count |
| New task category or search keyword | `ai/INDEX.md` (task navigation + keyword map) |
