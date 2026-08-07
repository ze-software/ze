---
kind: table
level:
stage:
---
| Rule | Why |
|------|-----|
| Lowercase start, no trailing punctuation, single line | Go convention; errors get wrapped, joined, and grepped |
| One **stable leading phrase** per failure kind (e.g. `reject=syslog pattern found:`) | Agents and log scanners match on it; do not reword per call site |
| Wrap the cause and add context: `fmt.Errorf("parse %s: %w", path, err)` | Preserves `errors.Is/errors.As` chains; each layer adds what it knows |
| Name the subject and the value, not just the type | "invalid value" with no value is unactionable |
| Truncate large blobs (bodies, dumps, hex) before embedding | A 10 MB error is unreadable for both humans and agents |
| No `fmt.Sprintf`/`fmt.Errorf` on hot paths -- see `ai/rules/performance.md` | Boundary and one-shot errors may use `fmt.Errorf`; hot paths use append builders |
