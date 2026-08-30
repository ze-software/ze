---
kind: directive
level: MUST
stage:
---
**A handler MUST return structured data, never pre-formatted text. The display layer owns rendering:**

| Do | Don't |
|----|-------|
| Return a typed struct (`*CPUInfo`, `[]NICInfo`) | Return `"CPU: Intel N100, 4 cores"` |
| Numeric fields (`*-bytes`, `*-mhz`) | A human string (`"8.0 GiB"`) |
| Kebab-case JSON with typed fields | YAML-ish text blocks |
| Let `\| table` and the web UI render | Render text in the handler |
