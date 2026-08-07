---
kind: directive
level:
stage:
---
**Implementation is delegated ONE agent per implementation phase, not one agent per spec.** Give each agent the spec path, the phase it owns, and the per-spec state file; it writes its handoff there when the phase is green, and the next agent reads that file instead of re-deriving the phase before it. Measured: implementation agents ran 144 API calls each at 294k mean context, more of both than any other phase, because context grows with turns inside one agent.
