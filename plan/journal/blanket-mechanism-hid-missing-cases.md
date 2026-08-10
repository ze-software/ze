| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-09 | kernel-compose-make-q-assertion-is-vacuous | installer kernel build | an unconditional rebuild was replaced by a precise trigger. Three cases it had silently covered were left with no trigger at all | enumerated what the blanket mechanism covered, then gave each case its own trigger and its own probe |
