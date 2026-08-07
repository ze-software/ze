---
kind: note
level:
stage:
---
Feeder 3 is an **in-process** guard, not a daemon-boot audit: built-ins are 100%
YANG-derived (a handler with no YANG path is skipped, `LoadBuiltinsWithAliases`) so
they are a strict subset of the static gate's tree, and plugin commands are rejected
at registration by Feeder 2. The merged `system command list` surface therefore
contains only conforming commands by construction: a boot-and-dump audit would add
no catch value while depending on an all-plugins config that cannot exist (startup is
config-path-gated). The guard instead locks the two runtime sources against
regression cheaply and deterministically.
