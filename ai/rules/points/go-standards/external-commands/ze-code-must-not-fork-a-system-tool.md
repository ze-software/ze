---
kind: directive
level: MUST NOT
stage:
---
**Ze code MUST NOT run an external binary.** No `exec.Command`, no `exec.CommandContext`, no shell. Ze is a network operating system: it runs on an appliance image carrying no shell utilities, inside a network namespace carrying nothing, and on a router nobody restarts. A fork buys three defects and one convenience. It depends on a binary the environment does not carry. It answers with a second implementation's opinion about the kernel, where Ze holds its own. And it reports nothing exactly when the environment is minimal and an operator most needs an answer.
