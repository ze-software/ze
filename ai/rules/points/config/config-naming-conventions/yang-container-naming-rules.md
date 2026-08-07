---
kind: table
level:
stage:
---
| Rule | Example | Anti-pattern |
|------|---------|-------------|
| Singular noun for the subsystem | `reactor` | `reactor-settings`, `reactor-config` |
| No `-config` or `-settings` suffix | `session` | `session-config` |
| Group related leaves, not one-per-container | `reactor { cache-ttl; cache-max; forward-queue-size; }` | `reactor-cache { ttl; max; }` + `reactor-forward { queue-size; }` |
