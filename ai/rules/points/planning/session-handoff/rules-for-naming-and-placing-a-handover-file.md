---
kind: directive
level: MUST NOT
stage:
---
- `NN` = highest existing number in `plan/handover/` plus one. Check with `ls plan/handover/` first; MUST NOT reuse a number (collisions like two `13-*.md` defeat ordering).
- One handover per file, and only under `plan/handover/`. MUST NOT scatter handover documents elsewhere in `plan/` (the rest of `plan/` is specs, journal classes, deferral shards and known-failure shards).
- The receiving session follows `.claude/rules/session-start.md` "Receiving a Handoff": enumerate every outstanding item before planning.
- Delete the handover file in the commit that completes its last item.
