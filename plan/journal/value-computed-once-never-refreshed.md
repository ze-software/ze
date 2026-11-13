# A value computed once is never refreshed while what it describes changes

A grade, a classification or a derived figure is computed at one moment, from
the inputs of that moment, and every later decision reads that first answer
while the inputs keep moving.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-13 | single-owner-lifecycle-5-session-end-locals (walked into) | ddos-detect severity (`onAttackStart`, `GradeSeverity`) | Severity is graded once, from the peak and threshold at trigger, and never revised: `AttackOngoing` carries no severity, and ddos-flowspec `onOngoing` only feeds `probeTick`. An attack triggering at 2x and growing to 100x never turns critical, so `blackhole-fallback` never fires: a 16,000 pps flood stayed `high` for 50s, graded on a first 4000-packet burst | open: re-grade on the ongoing tick and emit an escalation, or let blackhole-fallback read the live peak |
