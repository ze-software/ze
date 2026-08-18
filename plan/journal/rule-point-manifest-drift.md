| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-17 | spec-rfc4271-med-ibgp-readvertisement | rule point renderer | `make ze-doc-verify` failed because `ai/rules/points/precommit-verify/after-the-commit/what-ze-tracked-build-check-reads.md` existed (then under `git-safety/before-any-commit/`, before pre-commit verification became its own rule), but the git-safety manifest listed `what-ze-repository-tracked-build-check-reads`. The renderer refused the unlisted point. | corrected the manifest slug and regenerated the rendered rule artifacts |
