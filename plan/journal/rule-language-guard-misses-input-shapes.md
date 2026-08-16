| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-16 | - | rule authoring hook | MultiEdit content bypassed RFC 2119 enforcement because input normalization ignored its edit list | normalized all MultiEdit replacement text before checks run |
| 2026-08-16 | - | rule authoring hook | RFC keywords inside tilde fences and Markdown blockquotes were read as obligations | excluded fenced and quoted Markdown in both language checks |
