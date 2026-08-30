---
kind: directive
level: MUST
stage:
---
**The action on the matching row MUST be run for the surface the change touched,
after each edit.**

| You changed | Run this |
|-------------|----------|
| A `.go` file | `./le job run label unit-pkg command go test <that package>`, or the component group covering it (`./le test-unit bgp`, `core`, `plugins`, `config`, or `cli`). Then run `./le verify lint run` (`ai/rules/commands.md`) |
| A `.go` change that alters what the daemon PUTS ON THE WIRE, installs, or shows | The owning functional action as well: `./le functional plugin`, `encode`, `decode`, `parse`, `reload`, `ui`, or `web`. Unit tests of the package are not evidence about the rail |
| Reactor concurrency (`reactor/session*.go`, `forward_pool*.go`, `peer.go`) | `./le job run label reactor-race command go test -race -count=20 ./internal/component/bgp/reactor/...` (`ai/rules/testing.md`) |
| A `.ci` or `.et` test | Its native suite action: `./le functional plugin`, `parse`, `encode`, `editor`, or `web`. Draft first in `test/draft/` |
| Linux-only code (`//go:build linux`) | `./le qemu all-tests`, or `./le qemu netns-test suites <names>` for focused kernel-dependent `.ci` suites (`ai/rules/platform-linux.md`) |
| `rfc/short/*.md`, an `RFC requirement:` tag, `rfc/extraction/*` | `./le rfc check` |
| `docs/**`, `ai/**`, `plan/**` | `./le doc check verify`, and `./le doc wiring` for the changed-file gates |
| `ai/rules/points/**` | `./le rules render-update`, `./le rules condensed-update`, then `./le rules lint` |
| A `*.yang` file or a `ze:command` | `./le doc check verify`, `./le cli-grammar` |
| A plugin `register.go` or generated composition root | `./le plugin imports write`, then `./le plugin imports check` |
| A new package's placement | `./le tier check` |
| Anything, once the commit script has run and it carried Go | `./le repository tracked-build check`, the only check that compiles what git holds |
| A native tool package under `internal/le/` | `./le test-unit`; the permanent package tests compile and call the Go implementation directly |
| Several of the above, and you want breadth | `./le verify worktree` |
