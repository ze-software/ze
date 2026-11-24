| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-04 | web-cli-config-one-off-defects | Generated BFD plugin detail page, ce54d4c8ba | A dark-theme screenshot showed pale text on pale metadata cards, including At a glance, Dependencies and Startup ordering. | fixed: CSS uses shared panel/text/muted tokens; `TestPublishedBFDDetailCardContrast` (`internal/le/site/browsertest`, 17.4s, `1440/light`, `1440/dark`, `390/light`, `390/dark`) passed against canonical `../gh-pages`. |
