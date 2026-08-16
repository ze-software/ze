---
kind: directive
level: MUST
stage:
---
**Home a deferral by making the DESTINATION record it. What the destination records, the row MUST NOT repeat.**
When homing writes a NEW spec for the item, that spec carries the date, the source, the item and the reason, so the row is duplicate bookkeeping and MUST be deleted in the same commit that adds the spec.
When homing points at an EXISTING spec, one of two things MUST happen: add the item to that spec and delete the row, or keep the row, because it is then the only link between the work and its home.
Rationale: the gate reports a row with NO destination, so a homed row is already silent and deleting it removes no signal.
The link stays greppable from the source side, because the destination spec names the source it came from.
A row deleted this way is not work dropped: `ai/rules/completion.md` still governs, and `plan/future/README.md` still refuses a defect.
