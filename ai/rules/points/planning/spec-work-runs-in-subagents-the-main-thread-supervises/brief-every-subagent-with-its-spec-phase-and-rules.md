---
kind: directive
level:
stage:
---
**Give every subagent the spec path, the phase it is in, and the rules that govern it.** A subagent inherits no session state: name `plan/<spec>.md`, the `ai/rules/` files that apply, and what its report must contain. It cannot ask the user -- do not hand it work that needs an answer from them. It CAN resolve symbols: by the LSP tool where its registry carries one, by `gopls` from Bash where it does not (`ai/rules/context-economy.md`).
