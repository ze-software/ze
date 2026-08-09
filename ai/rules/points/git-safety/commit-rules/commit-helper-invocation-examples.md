---
kind: directive
level:
stage:
---
**Helper format:**
```bash
# Single commit (most common):
scripts/dev/commit_helper.py create \
  --replace \
  --subject "hook: allow tee pipe, per-session log paths" \
  --body "Explanation of why the change was made." \
  --file .claude/hooks/pretool-bash.py \
  --file ai/rules/commands.md \
  --lesson-not-needed "hook fix, no novel pattern"

# Second commit in the same script:
scripts/dev/commit_helper.py create \
  --append \
  --subject "feat: add widget support" \
  --body "Implements widget rendering for the dashboard." \
  --file internal/component/web/widget.go \
  --file internal/component/web/widget_test.go

# Spec closure (remove spec file):
scripts/dev/commit_helper.py create \
  --append \
  --subject "spec: close spec-widget" \
  --remove plan/spec-widget.md

# With a journal row:
scripts/dev/commit_helper.py create \
  --replace \
  --subject "rules: add goroutine lifecycle rule" \
  --file ai/rules/goroutine-lifecycle.md \
  --file plan/journal/<class>.md
```
