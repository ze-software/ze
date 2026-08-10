---
kind: directive
level: MUST
stage:
---
- **Table-driven:** `tests := []struct{...}` with `t.Run(tt.name, ...)`
- **Round-trip:** `original → packed → unpacked == original`
- **Fuzz (REQUIRED for wire format):** All external input parsing
- **Non-default params:** MUST test with non-default/non-zero values
