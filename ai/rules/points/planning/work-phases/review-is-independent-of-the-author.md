---
kind: directive
level: MUST
stage:
rationale: ai/rationale/planning.md
---
**Independence is a property of the context, not of the model, the agent count or the intention: a fresh session, a phase agent spawned after the implementing phase ended, or reviewer subagents each satisfy it, and the context that produced the work MUST NOT sit in judgment on it.** Your own inline reasoning about code you just wrote is authoring, not reviewing. This is the one phase boundary that MUST NOT be crossed by continuing an agent, and it holds for a small change and a mechanical one alike.
**Any one of the three satisfies the guarantee, so a context that already meets it MUST NOT spawn readers of its own: `/ze-close` MUST run every lens itself.** A pass carries at least two distinct lenses over the diff, each reading the producer rather than the caller, defaulting a finding PLAUSIBLE, and reproducing it before acting on it.
