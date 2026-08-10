---
kind: directive
level: MUST
stage:
---
- **A change that adds or changes something future agents need to use, verify, document, or avoid MUST update the discovery path in the same work.**
- **Every feature that adds a new runtime dependency MUST register a `ze doctor` check so agents can verify readiness before starting the daemon.**
- **A generated file MUST NOT be edited. Edit the canonical source, then sync.**
- **Project behavior rules MUST belong in `ai/rules/` and project startup guidance MUST belong in `ai/INSTRUCTIONS.md`, so Claude, Codex, and other agents all discover the same rule through generated tool-specific files.**
- **The hook-to-rule mapping MUST be consulted BEFORE writing code, to comply in advance rather than to fix after rejection. For hook false positives and workarounds, see `plan/learned/HOOK-FRICTION.md`.**
- **A recurring problem pattern, repeated surprise, stale guidance, tooling friction, or wasted effort MUST be reported immediately, and you MUST say whether a new or changed rule would prevent it.**
