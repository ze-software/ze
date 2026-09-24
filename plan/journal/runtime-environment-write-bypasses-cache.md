| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-22 | - | Credentialed L2TP evidence driver | `os.Setenv` set the selected daemon path, but `hostDaemon` reads `env.Get`. A diagnostic with the same deployment import observed an empty cached value after the OS value changed. | Use `env.Set`, which updates both stores. The first native timeout is not attributed solely to this defect; the corrected native run is separate. |
