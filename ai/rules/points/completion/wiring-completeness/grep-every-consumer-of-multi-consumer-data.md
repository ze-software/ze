---
kind: directive
level: MUST
stage:
---
**For multi-consumer data (route attributes, config fields, bus events) you MUST grep every consumer: UI templates, graph rendering, functional tests, CLI formatters.** Changing the producer without updating its consumers is incomplete, never done.
