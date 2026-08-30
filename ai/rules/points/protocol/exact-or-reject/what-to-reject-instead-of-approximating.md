---
kind: directive
level: MUST
stage:
rationale: ai/rationale/exact-or-reject.md
---
**Each situation below MUST be rejected at verify time, with the error its row names:**

| Situation | Wrong (silent) | Right (reject at verify) |
|-----------|----------------|--------------------------|
| Qdisc backend cannot reproduce | Map to closest supported | `qdisc <type>: not supported by backend <name>` |
| Filter type backend cannot match | Skip that filter | `filter <type>: not supported by backend <name>` |
| Backend has fewer slots than classes configured | Discard extras | Error naming capacity and actual count |
| Backend maps N inputs to the same output (name truncation) | Second overwrites first | `<name> exceeds <limit>-char limit; shorten or rename` |
| Numeric overflow at the backend's wire format | Truncate or wrap | `<value> out of range <lo..hi>` |
| Rate, burst, or DSCP outside the representable range | Silently clamp | Reject with the valid range in the message |
