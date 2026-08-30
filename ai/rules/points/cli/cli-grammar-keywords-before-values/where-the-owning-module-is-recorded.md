---
kind: directive
level: MUST
stage:
---
The owning module MUST be identified from source before the grammar is edited, and it
is recorded in one of these places:

- the existing `ze:command` path
- the module `register.go` / `embed.go`
- the handler `RPCRegistration`
