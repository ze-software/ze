---
kind: table
level:
stage:
---
| Operation | `map[string]V` | `map[uint16]V` |
|-----------|----------------|-----------------|
| Hash | hash the string bytes (length-dependent) | hash the integer (constant) |
| Compare | byte-by-byte comparison | single integer comparison |
| Key storage | allocates string header + backing bytes per key | inline in map bucket |
| GC scan | GC must scan string pointers | no pointers, no GC scan |
