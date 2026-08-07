---
kind: directive
level:
stage:
---
**A main thread whose context passes 600k writes its per-spec state file and hands off rather than continuing.** The file is `tmp/session/session-state-<spec-stem>-<SID>.md`, and `_find_latest_state_for_spec` (`.claude/hooks/lib/state-file.sh`) is what the next session reads it back with. Measured: 49.5% of main-thread context was fed at calls already above 600k, against a 1M ceiling, where every later call pays the whole context again.
