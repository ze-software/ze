---
kind: directive
level: MUST
stage:
---
- **A change that adds or changes something future agents need to use, verify, document, or avoid MUST update the discovery path in the same work.**
- **Every feature that adds a new runtime dependency must register a `ze doctor` check so agents can verify readiness before starting the daemon.**
- **Never edit a generated file. Edit the canonical source, then sync.**
- **Project behavior rules belong in `ai/rules/` and project startup guidance belongs in `ai/INSTRUCTIONS.md`, so Claude, Codex, and other agents all discover the same rule through generated tool-specific files.**
- **Consult the hook-to-rule mapping BEFORE writing code to comply in advance, rather than to fix after rejection. For hook false positives and workarounds, see `plan/learned/HOOK-FRICTION.md`.**
- **Report a recurring problem pattern, repeated surprise, stale guidance, tooling friction, or wasted effort immediately, and say whether a new or changed rule would prevent it.**
