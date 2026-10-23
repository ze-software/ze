---
kind: directive
level: MUST NOT
stage:
---
**An absent requirement of an RFC Ze implements is owed work, never a gap awaiting permission: it MUST be recorded in the RFC summary and implemented, and whether to implement it MUST NOT be put to the owner.** Release scheduling MUST follow the owner's release timing (`ai/META-DECISIONS.md`): a missing RFC feature MAY wait until after the first release. Touching a capability MUST NOT automatically pull every absent feature of its RFC into first-release scope; requirements needed for that capability's correct behavior remain inside the work.
**A question the RFC text answers MUST NOT be put to the owner.** Where the RFC is silent or unclear, the goal is interoperability: Ze MUST work with BIRD and FRR, so read their source, choose what lets Ze interoperate with both, and report the quoted evidence. Ask only when claiming a Linux implementation limit, when proposing a scope exception beyond that release timing, or when the RFC is silent and FRR and BIRD disagree.
**Prior owner decisions MUST remain in force until the owner changes them, and existing unfinished code MUST be preserved.**
