---
kind: directive
level:
stage:
---
**A hypothesis in a shard is not a finding.** If you record one, the next agent will
read it as fact. Before acting on an existing shard's stated cause, verify it against
source (`ai/rules/evidence.md`), and when it turns out to be wrong, say so in
the shard. On 2026-07-23 a shard's "the plugin connection closes before verify is
dispatched" hypothesis was disproved by the first real stress run: the signature
appeared nowhere in the capture, and the true cause was a test-harness race
(archived in `plan/known-failures/RESOLVED.md`, "fixed startup deadlines fail
under CPU oversubscription").
