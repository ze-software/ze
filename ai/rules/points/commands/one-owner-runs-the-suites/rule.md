---
kind: directive
level: MUST
stage:
---
**Suite runs have ONE owner: the main thread, or one agent it dedicates to
running them. Every other agent MUST report the command it wants run, and stop.**
A suite target, the runner binary, a race run, a QEMU target and a Docker
deployment target all count.
