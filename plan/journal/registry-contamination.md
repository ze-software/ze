| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | tests | test A called `Reset()` on package-level registry, test B failed | added Snapshot/Restore pair with t.Cleanup |
| 2026-04-06 | - | tests | Snapshot/Restore/Reset in registry missed new global | included every new global in snapshot |
| 2026-08-09 | kernel-profile-fixtures-leak-into-registry | tests | two `.ci` wrote kernel profile fragments into the tracked directory the appliance profile registry scans, cleaned only by an EXIT trap, so a killed run left a buildable profile behind | tests build a scratch repository root and write there; a unit test refuses any on-disk profile outside the shipped set |
