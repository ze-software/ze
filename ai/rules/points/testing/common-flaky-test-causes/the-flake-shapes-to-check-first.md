---
kind: note
level:
stage:
---
Seven flake shapes have been seen here: locked-write with unlocked-read,
subscribe-before-broadcast, gate-handler queue state, barrier FIFO order,
cleanup-drains-work, a fixed port behind an SO_REUSEPORT gate, and colliding
test-fake pool IDs. Check each one against your test before investigating a new
race or isolation flake.
