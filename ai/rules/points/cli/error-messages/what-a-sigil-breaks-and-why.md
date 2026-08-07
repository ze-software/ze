---
kind: table
level:
stage:
---
| What breaks | Why |
|-------------|-----|
| The value | `\| grep <lsp-id>` stops matching, and a parsed field carries a character that is not part of the identifier. The text form and the JSON form then disagree about what the identifier IS |
| The token | `*` is already an INPUT token in Ze: the selector wildcard for "all" (`peer *`, `clear bgp rib in *`, `192.168.*.*`), documented in `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md` and `docs/guide/route-injection.md`. One character pointing in two directions |
