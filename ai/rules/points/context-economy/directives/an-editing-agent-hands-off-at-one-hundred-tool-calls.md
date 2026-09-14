---
kind: directive
level: MUST
stage:
rationale: ai/rationale/context-economy.md
---
- **A work package MUST be cut at decomposition to about 60 tool calls, and an editing agent that reaches 100 tool calls MUST stop, append its handoff to the per-spec state file, and report that the package needs a continuation.** Every API call re-feeds the whole context, a subagent starts at a 45k floor and grows by about 1.9k tokens a call, and past 250k each call reads more for one tool result than a successor's whole start costs. The continuation carries the SAME package into a fresh context: it is never a reduced scope, a stub, or a parked item (`ai/rules/completion.md`), and the handoff's four parts are in `/ze-implement`, "Phase handoff". The pretool hook counts a `ze-work` agent's Bash and Write/Edit calls and refuses the 101st, letting the calls that write the state file through (`bashCallBudget`, `internal/le/hookruntime/budget.go`).
- **The main thread MUST spawn the continuation from that handoff, and the successor MUST NOT re-derive what the handoff digests.** A digest says where to look, never what the tree holds now: the successor judges the current tree, using the digest to make that cheap.
