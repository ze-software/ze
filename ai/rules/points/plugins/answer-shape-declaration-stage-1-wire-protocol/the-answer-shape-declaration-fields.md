---
kind: table
level:
stage:
---
| Field | Type | Purpose |
|-------|------|---------|
| `commands[].shape` | string | `doc` for one document, `map` for rows that carry their own keys, `tab` for rows read against column names |
| `commands[].columns` | []string | The answer's keys, in reading order. Needs a shape with rows. Maximum 64, each name 1 to 64 bytes |
| `commands[].address-fields` | []string | The keys whose value holds an address. Needs a shape. Maximum 16, each name 1 to 64 bytes |
