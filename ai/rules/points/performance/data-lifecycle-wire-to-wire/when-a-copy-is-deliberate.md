---
kind: table
level:
stage:
---
| Copy trigger | Why |
|---|---|
| Attribute enters pool for first time | Pool owns the canonical copy; wire buffer will be reused |
| ContextID mismatch on forward | Wire bytes encoded with different capabilities need re-encoding |
| Filter modifies attributes | Modified attributes are written into outgoing buffer |
| JSON serialization for external plugins | External plugins need formatted text, not wire bytes |
