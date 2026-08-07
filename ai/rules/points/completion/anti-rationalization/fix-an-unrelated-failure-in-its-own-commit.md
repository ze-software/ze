---
kind: directive
level:
stage:
---
1. **Fix it** as a separate commit (not mixed with feature work). Do not block current work on a
   failure you didn't cause, but DO fix it in the same session after completing the primary task.
2. **A shard is allowed for ONE case only: a failure whose MECHANISM you could not
   determine.** Deterministic reds, structural gates, anything with a reproduction
   command, and anything host load explains are fixed, never sharded. When the exception
   does apply, add
   `plan/known-failures/<make-target>-<test-name>.md` with: failure output, the
   reproduction attempt and its result, evidence gathered, and the next step. Label a
   root cause you have not verified against source a HYPOTHESIS, so the next agent does
   not inherit it as fact.
3. **Mechanical check before session end:** every failure your session encountered is
   fixed, or is a non-reproducible one whose shard names the next step. An unfixed
   deterministic failure is a violation regardless of what was written down.
