# One failure skips independent startup work

A startup sequence stops at the first failing step, and the steps after it
never run although none of them depends on the step that failed. When the
process then keeps running, it runs with part of its startup undone and
nothing an operator reads says which part.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-10 | spec-ddos-blind-detector-diagnostic (review round 2) | plugin startup (`Server.runPluginStartup` phase 2) | Read, not run: a failed explicit-plugin phase logs ERROR and returns, so auto-load phases 3 to 5 never run; unlike phase 1 it sets no `startupErr`, so ze runs half-started | open: `spec-doctor-reports-failed-plugin-startup.md`; the fail-closed owner ruling keeps the skip; stop ze as phase 1 does, or doctor names the phases not run |
