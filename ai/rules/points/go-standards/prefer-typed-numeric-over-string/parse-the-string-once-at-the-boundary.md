---
kind: directive
level:
stage:
---
**Pattern: Registry Maps Name to ID at Init, All Lookups Use ID.** Parse the string once at the boundary (config load, CLI parse, JSON unmarshal), convert to the numeric type, and pass the numeric value everywhere internally. The string exists only at the boundary for human readability.
