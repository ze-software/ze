---
kind: directive
level: MUST
stage:
rationale: plan/journal/documentation-shows-config-the-parser-refuses.md
---
**Every config example in `docs/` MUST parse, and an excerpt MUST parse inside the smallest complete config that carries it.** Build the binary and run `ze config validate` over that config before you publish the example. The one-line form above is one way an example is refused; a retired keyword, a leaf that moved, and a peer named by address are the others.

**Nothing in this repository parses a config example, so a refused one survives until a reader tries it.** `docs/guide/rpki.md` opens with a peer written `peer peer1 { remote { ... } local { ... } }`, and `remote` is not a peer field: `ze-bgp-conf.yang` models it under `container connection` in `grouping peer-fields`, so the parser answers `unknown field in peer: remote`. That example has never parsed, at `origin/main` too, and `route-reflection.md` and `graceful-restart.md` carry the same retired shape.
