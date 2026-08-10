| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-09 | kernel-compose-make-q-assertion-is-vacuous | installer kernel build | a stamp file listed as an ordinary make prerequisite read as not-newer and lost a profile switch. GNU Make 3.81 resolves mtimes to one second, and the same recipe wrote both files | compared the stamp's CONTENT at parse time, so no mtime decides the answer |
