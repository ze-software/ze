---
kind: table
level:
stage:
---
| Pipe | Purpose | Operates on |
|------|---------|-------------|
| `\| json` | Raw JSON output | JSON string |
| `\| ndjson` | Newline-delimited JSON | JSON string |
| `\| table` | Tabular display (default) | JSON string |
| `\| text` | Plain text | JSON string |
| `\| yaml` | YAML output | JSON string |
| `\| match <pat>` | Grep output lines | formatted string |
| `\| count` | Count results | JSON string |
| `\| resolve` | Add reverse DNS for IPs | JSON (walks values) |
| `\| origin` | Add ASN/network for IPs | JSON (walks values) |
| `\| log` | Streaming log mode | display mode flag |
| `\| no-more` | Paging | display mode flag |
