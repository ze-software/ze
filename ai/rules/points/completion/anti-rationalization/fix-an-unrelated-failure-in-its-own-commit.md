---
kind: directive
level: MUST
stage:
---
1. **You MUST spec it, close the work in hand, then ask ("A problem you FIND while working on something else gets a SPEC", above). You MUST NOT block current work on a failure you did not cause, and you MUST NOT fix it in this session either: the fix runs when Thomas answers, as its own spec and its own commit, never mixed with the feature work you were closing.**
2. **A shard MAY be used for ONE case only: a failure whose MECHANISM you could not
   determine.** Deterministic reds, structural gates, anything with a reproduction
   command, and anything host load explains MUST be fixed, never sharded. When the exception
   does apply, you MUST add
   `plan/known-failures/<native-action>-<test-name>.md` with: failure output, the
   reproduction attempt and its result, evidence gathered, and the next step. You MUST label a
   root cause you have not verified against source a HYPOTHESIS, so the next agent does
   not inherit it as fact.
3. **Mechanical check before session end:** every failure your session encountered MUST be fixed, or
   MUST carry a spec that was put to Thomas, or MUST be a non-reproducible one whose shard names the next
   step. A failure that is none of the three is a violation regardless of what was written down.
