# Long run reports no progress

A command that runs for tens of minutes prints nothing until it ends. The
operator cannot tell a run that is nearly done from one that has barely started
or one that is stuck, so the only choices are to wait blind or to cancel and lose
the work done so far.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-23 | lab-docker-cleanup-on-macos | `./le verify current fast` | The last verify had exited 130, so the scope widened to every package and feature, said once in one line, then silence. After 45 minutes there was no stage, count or ETA, and the owner cancelled it. | open: print each stage as it starts and ends, with done/total and elapsed time. |
