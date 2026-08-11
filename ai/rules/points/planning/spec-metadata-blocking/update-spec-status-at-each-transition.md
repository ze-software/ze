---
kind: table
level:
stage:
---
| Event | Status change | Phase | Updated | When exactly |
|-------|--------------|-------|---------|--------------|
| Start research | `skeleton` to `design` | - | Yes | When research begins |
| Spec approved | `design` to `ready` | - | Yes | After user approves design |
| Start coding | `ready` to `in-progress` | Set `1/N` | Yes | When coding begins |
| Finish a phase | - | Increment | Yes | After phase tests pass |
| Hand off for review (`Handoff: verify` only) | `in-progress` to `verification` | - | Yes | Before the implementation session commits and stops |
| Blocked | to `blocked` | - | Yes | When blocker identified |
| Deferred | to `deferred` | - | Yes | When user agrees to defer |
