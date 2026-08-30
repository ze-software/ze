---
kind: directive
level: MUST
stage:
---
**Each event below MUST update the spec's Status, Phase and Updated fields as its row says. `writeSpecStatus` in `internal/le/hookruntime/writeedit.go` refuses a source edit while the claimed spec is not `in-progress`, and `docs/contributing/spec-workflow.md` says what each status means.**

| Event | Status change | Phase | Updated | When exactly |
|-------|--------------|-------|---------|--------------|
| Start research | `skeleton` to `design` | - | Yes | When research begins |
| Spec approved | `design` to `ready` | - | Yes | After user approves design |
| Start coding | `ready` to `in-progress` | Set `1/N` | Yes | When coding begins |
| Finish a phase | - | Increment | Yes | After phase tests pass |
| Hand off for review (`Handoff: verify` only) | `in-progress` to `verification` | - | Yes | Before the implementation session commits and stops |
| Blocked | to `blocked` | - | Yes | When blocker identified |
| Deferred | to `deferred` | - | Yes | When user agrees to defer |
