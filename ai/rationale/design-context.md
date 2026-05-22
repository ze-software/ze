# Rationale: Design Context

Ze has strong opinions that contradict standard Go patterns. DirectBridge
instead of gRPC. EventBus instead of channels. YANG instead of struct tags.
Registration via init instead of constructor injection. An AI's trained
instincts are actively wrong here.

The incident log demonstrates the cost of designing without context:

- Session as-router (2026-04-13) made 7 wrong recommendations by starting
  to design before loading context. Each recommendation matched industry
  best practice but contradicted Ze's existing architecture.

- Session l2tp-8a-auth-pool (2026-04-21) proposed a new direct-call
  mechanism between core and plugins, not discovering that DirectBridge
  already provides typed function calls. The entire design discussion was
  wasted because one grep would have found the existing solution.

Reading context first costs minutes. Reworking a wrong design costs the
entire session. The tiered reading list (Tier 1: always, Tier 2: by
artifact, Tier 3: by area) is structured so the minimum read for any
design decision is 3-4 files, not the entire documentation corpus.

The anti-patterns table exists because "industry standard is X" is the
most common rationalization for ignoring Ze's existing patterns. The
mechanical check ("did I grep how ze already handles similar?") catches
this at the earliest possible moment.
