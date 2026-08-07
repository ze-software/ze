---
kind: note
level:
stage:
---
External plugins can declare named route filters at stage 1 via `declare-registration`.
This is runtime IPC, not compile-time registration. Filter fields are stored in
`PluginRegistration` (`internal/component/plugin/registration.go`), not `Registration`.
