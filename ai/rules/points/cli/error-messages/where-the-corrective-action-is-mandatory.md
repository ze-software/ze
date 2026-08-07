---
kind: note
level:
stage:
---
Scope of leg 3: it is mandatory on machine-facing surfaces (doctor, startup,
config apply/verify, readiness, plugin load -- the diagnostic-code surfaces
below). For internal errors that get wrapped upward, legs 1 and 2 plus a
wrapped cause (`%w`) are the requirement; add the corrective action whenever
a clear next step exists, but a deep internal error need not invent one.
