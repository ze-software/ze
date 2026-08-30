---
kind: directive
level: MUST NOT
stage:
---
Turning on verbose **logging** changes output, not protocol behavior, so it MUST NOT
be modeled as a `debug` command. Model it as configuration
(`set ... log level debug`). The `debug` verb is reserved for perturbing protocol or
dataplane state.
