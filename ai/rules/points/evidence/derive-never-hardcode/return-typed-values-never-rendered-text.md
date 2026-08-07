---
kind: table
level:
stage:
---
| Do | Don't |
|----|-------|
| Return typed struct (`*CPUInfo`, `[]NICInfo`) | Return `"CPU: Intel N100, 4 cores"` |
| Numeric fields (`*-bytes`, `*-mhz`) | Human string (`"8.0 GiB"`) |
| Kebab-case JSON with typed fields | YAML-ish text blocks |
| Let `\| table` / web UI render | Render text in handler |
