---
kind: table
level: MUST NOT
stage:
---
| AC text says | Test MUST assert | Test MUST NOT assert |
|-------------|-----------------|---------------------|
| "rejected" / "not installed" | Route is absent from delivery / RIB | No error returned (mechanism) |
| "session torn down" | Connection closed + NOTIFICATION sent | NOTIFICATION struct returned (mechanism) |
| "warning logged" | Log entry exists (or counter incremented) | No teardown (absence of something) |
| "rejected at parse time" | Error returned with specific message | Generic error returned |
