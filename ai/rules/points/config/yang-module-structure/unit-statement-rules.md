---
kind: table
level:
stage:
---
| Rule | Canonical | Anti-pattern |
|------|-----------|--------------|
| One mechanism | `type uint32; units milliseconds;` | unit in the leaf name (`min-tx-us`, `spf-delay-ms`, `teardown-grace-seconds`) |
| Full word, unquoted | `units microseconds;`, `units seconds;`, `units bytes/second;` | `units "seconds";` (quoted), `-us` / `-ms` / `-secs` abbreviations |
| Integer, not string | `type uint32; units seconds;` | `type string` for a duration |
| Protocol-sane default | every dimensioned leaf carries a `default` set to the protocol's standard/recommended value (OSPF `hello-interval` 10s, `dead-interval` 40s, BFD tx/rx per RFC 5880, ...) | no `default`, so omitting the leaf yields 0 or undefined timing |
