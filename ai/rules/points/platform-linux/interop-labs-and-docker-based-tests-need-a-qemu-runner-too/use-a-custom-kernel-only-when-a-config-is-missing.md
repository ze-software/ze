---
kind: directive
level: MUST
stage:
---
**Probing the stock Alpine kernel proves nothing about a lab, and its result MUST NOT be recorded as a reason to skip step 3's `--kernel`.** A probe answers a question about Alpine, while the lab's verdict is about the kernel ze ships, so a green probe and a green lab on stock together establish only that Alpine works. A capability the probe found MUST be declared in `gokrazy/kernel/runtime.config` with its symbol in `gokrazy/kernel/runtime.require`, so the lab gets it from the kernel under test and a silent demotion to `=m` fails the build instead of the lab.
