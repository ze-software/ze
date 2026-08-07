---
kind: table
level:
stage:
---
| Vacuity trap | Why it passes anyway | The tell |
|--------------|----------------------|----------|
| An interop test for a sender-side wire change whose receiver must accept any form (e.g. RFC 7606 Section 5.1: receivers accept any field combination) | a conforming peer accepts the old and new wire equally | reverting the sender change leaves the peer's routing table identical |
| A test asserting the ABSENCE of something (no log line, no allocation, no route) | deleting the mechanism leaves the same absence | ask "what would still be absent if the code were removed?" |
| A test whose fixture is at an extreme (all-fields-set, max value) | an off-by-one or partial break still handles the extreme | boundary the fixture, test one-below and one-above |
| A functional test whose data reaches the peer by a DIFFERENT path than the one changed | the unchanged path still delivers | trace which code path actually produces the asserted bytes |
