---
kind: directive
level: MUST
stage:
---
**You MUST use this native command format:**
```bash
# Single commit (most common):
./le commit create \
  replace \
  subject "hook: allow tee pipe, per-session log paths" \
  body "Explanation of why the change was made." \
  file internal/le/hookruntime/bash.go \
  file ai/rules/points/commands/<section>/<point>.md

# Second commit in the same script:
./le commit create \
  append \
  script tmp/commit-<session>-<tag>-<random>.sh \
  subject "feat: add widget support" \
  body "Implements widget rendering for the dashboard." \
  file internal/component/web/widget.go \
  file internal/component/web/widget_test.go

# Spec closure (remove spec file):
./le commit create \
  append \
  script tmp/commit-<session>-<tag>-<random>.sh \
  subject "spec: close spec-widget" \
  remove plan/spec-widget.md

# With a journal row:
./le commit create \
  replace \
  subject "rules: add goroutine lifecycle rule" \
  file ai/rules/points/<rule>/<section>/<point>.md \
  file plan/journal/<class>.md
```
