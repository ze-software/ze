| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-04-14 | - | protocol | `int16(a - b) < 0` failed at half-space boundary | used unsigned distance `uint16(b - a) <= 0x7FFF` |
