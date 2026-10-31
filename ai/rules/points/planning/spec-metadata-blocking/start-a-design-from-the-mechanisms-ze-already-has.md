---
kind: directive
level: MUST
stage:
---
**A spec whose Status is `design`, `ready`, `in-progress` or `verification` MUST carry a `## Existing Mechanisms` section, between `## Current Behavior` and `## Data Flow`, whose table holds one row per mechanism Ze already has that does part of the job: the mechanism, where it lives, what it does today, and whether the spec extends it, replaces it, or does not apply it, with the reason.** The rows come from the doors, the architecture pages and `plan/learned/DESIGN-HISTORY.md`, and a design proposed beside an existing mechanism instead of extending it MUST be refused at review (`ai/META-DECISIONS.md`, "How agents work"). `plan/templates/TEMPLATE.md` carries the section. `writeSpecMechanisms` (`internal/le/hookruntime/specmechanisms.go`) refuses a spec edit whose result lacks a data row, and `./le commit create` refuses a commit carrying such a spec it edited (`specMechanismsRefusal`, `internal/le/commit/specmechanisms.go`). A `skeleton`, `blocked`, `deferred` or `done` spec owes no row, and a spec no edit or commit touches is never judged, so a spec written before the rule owes the section the next time its text changes.
