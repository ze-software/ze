---
kind: directive
level: MUST
stage:
---
- You MUST NOT run `make ze-precommit-verify`, `make ze-precommit-verify-changed`, lint, or tests as a
  late commit gate.
- You MUST inspect only enough state to stage exactly the requested files and avoid
  ignored, generated, unrelated, or user-owned paths.
- You MUST use `scripts/dev/commit_helper.py create` with the normal user-run script
  path. The override changes verification requirements only.
- You MUST NOT run `git add`, `git commit`, `git rm`, `git stash`, or prohibited git
  commands from an AI tool.
- You MUST NOT add `--no-verify`, `--no-gpg-sign`, disabled hooks, or any bypass to
  the generated script.
- You MUST report `Verification skipped by Thomas owner override` in the final response
  and, when useful, in the commit body.
- You MUST NOT claim tests, lint, `ze-precommit-verify`, integrations, or behavior were
  verified if they were skipped.
