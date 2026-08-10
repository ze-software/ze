---
kind: directive
level: MUST
stage:
---
**On any diff, the FIRST review question is "is this change bigger than the problem?" MUST ask it before you audit one detail.**
**A round that audits the details of an over-engineered change ratifies it. Every finding is about machinery that SHOULD NOT exist, and every fix drives another pass over more of it. MUST report `this should be N lines` as a BLOCKER, and restart from the smaller change (`ai/rules/simplicity.md`).**
**This happened on 2026-08-08. A two-line regex change took three review rounds and two implementation agents, because the first implementation was over-engineered and the rounds policed it instead of questioning its size.**
