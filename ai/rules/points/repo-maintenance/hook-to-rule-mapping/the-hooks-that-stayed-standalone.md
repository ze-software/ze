---
kind: note
level:
stage:
---
Still standalone (single-purpose or deliberately not folded): `block-until-lsp.sh`, `validate-spec.sh` (see note below), `mark-lsp-invoked.sh`, `mark-source-read.sh`, and the session-lifecycle hooks. The Stop hook also shells out to `scripts/dev/spec-closure-check.py` (the spec-closure detector; also usable directly as `--list`).
