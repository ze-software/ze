---
kind: table
level:
stage:
---
| Protocol | Maps cleanly | Exceptions |
|----------|--------------|------------|
| BFD | everything | none -- the reference |
| IS-IS | `packet`, `transport`, root engine, `adjacency` (RFC term), `types`, `yang`, `cli`; `circuit`/`lsdb`/`spf`/`redistribute` domain modules | none |
| OSPF | `packet`, `transport`, root engine, `neighbor` (RFC term), `types`, `yang`, `cli`; `iface`/`lsdb`/`spf`/`sr`/`redistribute` domain; `v3` version dir; `wire` = raw handoff type (glossary sense of `wire`) | none |
| IKE | `engine`, `transport`, `yang`, `cmd`; `crypto`/`eap`/`ipsec`/`dataplane` domain | `wire` is a full codec where the skeleton says `packet` (predates the glossary; kept) |
| BGP | `fsm` (RFC term), `types`, `yang`, `cli`; many platform modules | platform archetype, pre-SDK: `message`+`wireu` for the codec, `reactor`+`server` for the runtime -- documented as historical in the glossary; not a template for new work |
| LDP, RSVP-TE | single-package + `yang/` | below the subpackage threshold; skeleton N/A until they grow |
