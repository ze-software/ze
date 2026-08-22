---
kind: note
level:
stage:
---
External plugins can name a CLI pipe alias for their own commands at stage 1 via
`declare-registration`. An alias is the word an operator types after the pipe
character, and it stands for an operator chain. The engine parses the expansion
once, at registration, and the daemon resolves it. `RegisterPluginAliases`
(`internal/component/command/alias.go`) reports a bad declaration as an error
rather than a panic, because the strings arrived over a socket.
