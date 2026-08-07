---
kind: directive
level:
stage:
---
- **Table-driven:** `tests := []struct{...}` with `t.Run(tt.name, ...)`
- **Round-trip:** `original → packed → unpacked == original`
- **Fuzz (MANDATORY for wire format):** All external input parsing
- **Non-default params:** Always test with non-default/non-zero values
