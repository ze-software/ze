---
kind: directive
level:
stage:
---
- Do not run `make ze-verify`, `make ze-verify-changed`, lint, or tests as a
  late commit gate.
- Do inspect only enough state to stage exactly the requested files and avoid
  ignored, generated, unrelated, or user-owned paths.
- Do use `scripts/dev/commit_helper.py create` with the normal user-run script
  path. The override changes verification requirements only.
- Do not run `git add`, `git commit`, `git rm`, `git stash`, or prohibited git
  commands from an AI tool.
- Do not add `--no-verify`, `--no-gpg-sign`, disabled hooks, or any bypass to
  the generated script.
- Report `Verification skipped by Thomas owner override` in the final response
  and, when useful, in the commit body.
- Do not claim tests, lint, `ze-verify`, integrations, or behavior were
  verified if they were skipped.
