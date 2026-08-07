---
kind: directive
level:
stage:
---
- Error: `{"error":"description","parsed":false}`
- CLI: `{"status":"ok","data":{...}}` or `{"status":"error","error":"msg"}`
- Raw hex: uppercase, no `0x`. `"parsed":false` + `"raw":"DEADBEEF"`
- Numbers: JSON `float64` in Go -> use `formatNumber()` for integer display
