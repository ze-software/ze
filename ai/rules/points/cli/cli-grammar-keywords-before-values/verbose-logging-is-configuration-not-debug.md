---
kind: note
level:
stage:
---
Not `debug`: turning on verbose **logging** changes output, not protocol
behaviour -- model it as configuration (`set ... log level debug`), never as a
`debug` command. The `debug` verb is reserved for perturbing protocol/dataplane
state.
