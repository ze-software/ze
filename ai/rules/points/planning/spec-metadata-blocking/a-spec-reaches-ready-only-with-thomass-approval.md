---
kind: directive
level: MUST
stage:
---
**Unless Thomas is recorded away, a spec MUST reach `ready` only after he approved its design from his own terminal, and the main session MUST present that design, never a subagent.** Before he sees it, the spec carries `## Design documents`, a verdict on every design page its files link and every decision row on them, and `## Design presentation`; `./le spec design check` passes, and an agent that did not write the spec has recorded its verdict through `./le spec design judge` (`ai/skills/ze-design-judge.md`). The main session presents the design whole and hands him `./le spec design approve spec <path> words "<what he said>"`, which it MUST NOT run itself. `writeSpecReady` (`internal/le/hookruntime/specready.go`) refuses the edit that makes a spec `ready` from a subagent, and from the main session without a matching approval; `Claim` (`internal/le/spec/session.go`) refuses to start a `ready` spec whose design text has no approval. While he is away, only a judged fix spec with no `departs` row starts without him. The formats, the records and the commands are `docs/contributing/spec-workflow.md`, "Design approval".
