| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-16 | - | rule authoring hook | MultiEdit content bypassed RFC 2119 enforcement because input normalization ignored its edit list | fixed: normalized all MultiEdit replacement text before checks run |
| 2026-08-16 | - | rule authoring hook | RFC keywords inside tilde fences and Markdown blockquotes were read as obligations | fixed: excluded fenced and quoted Markdown in both language checks |
