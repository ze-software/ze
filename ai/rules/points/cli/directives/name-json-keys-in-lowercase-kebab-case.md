---
kind: directive
level: MUST
stage:
---
- **Every JSON key MUST be lowercase kebab-case matching its YANG leaf or config tree key, and MUST NOT be camelCase or snake_case; every exported struct field that reaches JSON output MUST carry a `json:"kebab-name"` tag.** JSON MUST be built with `encoding/json` rather than string concatenation. The envelopes are `{"status":"ok","data":{...}}`, `{"status":"error","error":"msg"}` and `{"error":"description","parsed":false}`; raw hex is uppercase with no `0x`. The key set, the address families and the one exemption are `docs/architecture/api/json-format.md`.
