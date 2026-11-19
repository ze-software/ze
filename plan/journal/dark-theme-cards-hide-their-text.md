| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-04 | web-cli-config-one-off-defects | Generated BFD plugin detail page, ce54d4c8ba | A dark-theme screenshot showed pale text on pale metadata cards, including At a glance, Dependencies and Startup ordering. | open: AC-2 current CSS uses shared panel, text and muted tokens. Canonical-artifact browser proof now checks every metadata-card text node in both themes at 1440 and 390 pixels. Native build and browser acceptance remain owed; no new palette or CSS fix is claimed. |
