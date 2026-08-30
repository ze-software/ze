---
kind: directive
level: MUST
stage:
---
**Each trap below MUST be checked for by its tell before a test is called evidence:**

| Vacuity trap | Why it passes anyway | The tell |
|--------------|----------------------|----------|
| An interop test for a sender-side wire change whose receiver is obliged to accept any form (RFC 7606 Section 5.1: receivers accept any field combination) | A conforming peer accepts the old and new wire equally | Reverting the sender change leaves the peer's routing table identical |
| A test asserting the ABSENCE of something (no log line, no allocation, no route) | Deleting the mechanism leaves the same absence | Ask "what would still be absent if the code were removed?" |
| A test whose fixture is at an extreme (all-fields-set, max value) | An off-by-one or partial break still handles the extreme | Boundary the fixture: test one below and one above |
| A functional test whose data reaches the peer by a DIFFERENT path than the one changed | The unchanged path still delivers | Trace which code path actually produces the asserted bytes |
