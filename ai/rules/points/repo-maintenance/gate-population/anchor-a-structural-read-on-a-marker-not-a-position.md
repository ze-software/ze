---
kind: directive
level: MUST
stage:
---
**A check that reads another artifact's STRUCTURE MUST anchor on a marker that
artifact guarantees, never on a position inside it, and MUST resolve the
indirection that artifact's own format permits. A positional window stops seeing
data the moment the data moves past it. A reader blind to indirection reports
"not wired" for a subject that is wired. Both failures present as a verdict
about the subject when they are a verdict about the reader, which is why neither
is caught by re-running the check.**

**The agreement MUST be pinned by feeding a real, canonical instance of that
artifact through the reader in a test.** A reader and the artifact it parses
drift apart silently whenever nothing exercises the two together, and the drift
surfaces as a wrong answer rather than as an error. Where the read can fail
outright, the failure MUST stay distinguishable from a value that is legitimately
absent (`ai/rules/evidence.md`, "Fail-Closed Guards").
