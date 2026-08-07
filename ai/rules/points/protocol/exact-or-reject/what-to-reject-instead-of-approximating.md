---
kind: table
level:
stage:
---
| Situation | Wrong (silent) | Right (reject at verify) |
|-----------|----------------|--------------------------|
| Qdisc backend cannot reproduce | Map to closest supported | `qdisc <type>: not supported by backend <name>` |
| Filter type backend cannot match | Skip that filter | `filter <type>: not supported by backend <name>` |
| Backend has fewer slots than classes configured | Discard extras | Error naming capacity + actual count |
| Backend maps N inputs to same output (name truncation) | Second overwrites first | `<name> exceeds <limit>-char limit; shorten or rename` |
| Numeric overflow at backend's wire format | Truncate/wrap | `<value> out of range <lo..hi>` |
| Rate/burst/DSCP outside representable range | Silently clamp | Reject with valid range in message |
