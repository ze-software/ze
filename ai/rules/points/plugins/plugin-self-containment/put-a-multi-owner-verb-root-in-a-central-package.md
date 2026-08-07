---
kind: note
level:
stage:
---
A verb whose subcommands belong to several owners (e.g. `monitor bgp`,
`monitor vpn ipsec`, `monitor ping`) must NOT declare its root container inside
any one plugin. If it does, deleting that plugin deletes the whole verb. The
root lives in a central, plugin-free package `internal/component/cmd/<verb>`
(a `doc.go` that blank-imports its `yang/` subpackage, mirroring `internal/component/cmd/delete`);
each owner container-merges only its own subtree onto that root. Precedent:
`internal/component/cmd/monitor` holds the `container monitor` root, while
`monitor bgp` stays in the BGP plugin and the other subcommands carve out to
their feature owners. The central package holds NO handlers; subcommand handlers
register from their owners.
