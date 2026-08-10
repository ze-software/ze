---
kind: directive
level: MUST
stage:
---
**Runtime vs offline tree:** the runtime completion tree DOES inject plugin
`CommandRegistry` entries after startup (`internal/component/cli/client/inject.go`
`injectPluginCommands`), so plugin commands complete in the live CLI. The static
offline tree (`BuildCommandTree`, used when no daemon is reachable, and
`ze help command`) still sees only YANG-backed commands; a plugin whose commands
MUST complete offline SHOULD ship a `-cmd` YANG module.
