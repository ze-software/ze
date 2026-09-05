---
kind: directive
level: MUST
stage:
---
**The verb MUST be chosen by the command's effect on live state, never by how diagnostic it feels: does running it change what the router does, emits, or forwards?** No, it only reports: `show` for one snapshot or `monitor` for the same read streamed, however deep the introspection goes. Yes, as a normal operational action: the existing action verbs `request`, `clear`, `create`, `set`, `delete`, `update`, `cache`. Yes, and the operator supplies the message that leaves the router for a destination the operator names: `send <protocol> <selector> <form>`, where the protocol keyword comes first and types the value after it. Yes, as a deliberate diagnostic PERTURBATION (inject, force, corrupt, drop, toggle a fault mode): `debug`, double-gated by authz and a fail-closed runtime enablement. An operational `add`, `del` or `remove` MUST NOT be invented for an object that already lives in the config YANG tree: a change to a tree node stays in engine path form under `set`/`delete` and MUST NOT be mirrored as an RPC to make the grammar look regular.
