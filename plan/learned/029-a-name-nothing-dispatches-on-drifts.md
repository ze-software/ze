# Learned: a name nothing dispatches on drifts until a gate reads it

`spec-rpc-published-name-does-not-reach-its-handler` found 117 wire methods that
`ze schema methods`, `ze help ai --json` and the MCP `ze_reference` tool
published and no handler answered. Three declarations could name one method:
the rpc statement (through `WireModule`, which derived a prefix from the module
file name), the `ze:command` node, and the handler's `RPCRegistration`. The
command worked whenever the node and the handler agreed, because the dispatcher
keys on the YANG path, so the third name reached only documentation and nothing
ever failed on it.

## What changed

- The `ze:command` node is the one declaration of a wire method. It points at
  the rpc that documents it with `ze:rpc`; the plugin IPC rpcs and the
  notifications, which no node reaches, declare theirs with `ze:method`.
  `yang.PublishedRPCs` (`internal/component/config/yang/rpc_publish.go`) reads
  both, and `WireModule` is deleted.
- The prefix is derived from the package that registers the handler
  (`OwnerPrefix`, `internal/component/plugin/server/rpc_register.go`), so a
  clash between two subsystems cannot be written. The command-contract gate
  (`./le doc yang-contract command-contract`) fails on a published method with
  no handler, a handler with no node, an orphan local handler, and a foreign
  prefix.
- A second builtin holder of a command name or wire method is refused at
  registration and at server start (`ErrCommandHeld`, `ErrWireMethodHeld`).

## What outlives the spec

1. **A surface with no consumer inside the process needs a gate that compares
   it with the one that has a consumer.** Documentation that nothing dispatches
   on cannot break a command, so it cannot be caught by using the product.
2. **A rename can leave a hand-listed guard vacuous.** After the prefix rename,
   three tokens in the central show/clear schema guards matched nothing. A guard
   over a text scan states how many statements it judged and refuses a
   statement it could not read; where the population can legitimately be
   empty (the central clear schema holds no command), the read check is what
   keeps the zero honest.
3. **A commit that compiles only beside another session's uncommitted edit is
   not landed work.** The third closure attempt found HEAD did not build
   `cmd/ze`; closure evidence is gathered in a clean clone of HEAD, never in
   the shared tree.
