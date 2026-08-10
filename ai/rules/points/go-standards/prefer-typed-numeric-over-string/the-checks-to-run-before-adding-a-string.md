---
kind: directive
level: MUST
stage:
---
**You MUST run these checks before adding a string:**
1. Finite set, compile-time? -> typed enum.
2. Plugin-extensible? -> numeric ID + registry.
3. External contract (YANG/JSON/CLI/log)? -> OK at boundary; convert internally.
4. None of the above? Ask why a string.
5. Does `String()` allocate? -> const literals, or registry + `unsafe.String` on packed store.
6. Consumer parses back to typed? -> emit typed with `MarshalText`; no roundtrip.
7. Map key? -> use numeric type; parse string to numeric at the boundary.
