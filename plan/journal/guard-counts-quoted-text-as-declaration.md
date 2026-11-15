# Guard counts quoted text as a declaration

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-15 | plugin-fork-environment | `subsystemReadingsInGo` log-key gate | Source-traced, PLAUSIBLE: arbitrary Go literals, including the test's `bgp-no-such-thing`, enter the accepted logger set without a declaration | open: runtime reproduction and producer-aware extraction owed; excluding tests alone misses production literals |
