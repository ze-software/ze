---
kind: directive
level: MUST
stage:
---
1. Every repair MUST carry a stable `id` (lower-kebab) and `summary`.
2. Safety labels: `format-only`, `section-local`, `behavior-preserving`, `api-changing`, `target-changing`, `requires-human-review`.
3. If Ze cannot prove a repair is safe, MUST use `requires-human-review` with id `manual-review`.
