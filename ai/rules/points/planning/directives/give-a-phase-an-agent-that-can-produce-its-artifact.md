---
kind: directive
level: MUST
stage:
---
**A phase MUST be given an agent whose tool set can produce that phase's artifact, and a phase that writes a file MUST NOT be given a read-only agent.** `ze-read` carries neither Write nor Edit, so an agent asked to author a spec returns the spec as prose in its report and the main thread pays a second agent to land the file; `ze-work` is the editing phase agent. Measured 2026-09-04: two spec phases in one session came back as text because their briefs named `ze-read`, and a 593-line spec crossed three contexts to reach `plan/`. The ARTIFACT decides the agent, never the phase's name: research that ends in a journal row writes, and a review that ends in a report does not.
