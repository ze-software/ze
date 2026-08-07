---
kind: note
level:
stage:
rationale: plan/learned/608-concurrent-test-patterns.md
---
Flake-shape catalogue (locked-write/unlocked-read, subscribe-before-broadcast,
gate-handler queue state, barrier FIFO, cleanup-drains-work, fixed-port
SO_REUSEPORT gate, test-fake pool IDs): `plan/learned/608-concurrent-test-patterns.md`.
Read it before investigating a new race or isolation flake.
