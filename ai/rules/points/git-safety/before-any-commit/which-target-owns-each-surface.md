---
kind: table
level:
stage:
---
| You changed | Run this |
|-------------|----------|
| A `.go` file | `make ze-unit-pkg-test PKG=<that package>`, or the group target covering it (`ze-unit-bgp-test`, `ze-unit-core-test`, `ze-unit-plugins-test`, `ze-unit-config-test`, `ze-unit-cli-test`, `ze-unit-rest-test`). Then `make ze-lint-changed` (`ai/rules/commands.md`) |
| A `.go` change that alters what the daemon PUTS ON THE WIRE, installs, or shows | the FUNCTIONAL suite owning that surface as well: `make ze-functional-plugin-test`, `ze-functional-encode-test`, `ze-functional-decode-test`, `ze-functional-parse-test`, `ze-functional-reload-test`, `ze-functional-ui-test`, `ze-functional-web-test`. The unit tests of the package you edited are not evidence about the rail |
| Reactor concurrency (`reactor/session*.go`, `forward_pool*.go`, `peer.go`) | `make ze-unit-reactor-test-race` (`ai/rules/testing.md`) |
| A `.ci` or `.et` test | its suite target: `make ze-functional-plugin-test`, `ze-functional-parse-test`, `ze-functional-encode-test`, `ze-functional-editor-test`, `ze-functional-web-test`. Draft first in `test/draft/` |
| Linux-only code (`//go:build linux`) | `make ze-qemu-integration-test`, or `make ze-qemu-needs-linux-test` for a `needs-linux` `.ci` (`ai/rules/platform-linux.md`) |
| `rfc/short/*.md`, an `RFC requirement:` tag, `rfc/extraction/*` | `make ze-rfc-check` |
| `docs/**`, `ai/**`, `plan/**` | `make ze-doc-verify`, and `make ze-doc-wiring-check` for the changed-file gates |
| `ai/rules/*.md` | `make ze-rules-condensed-update` then `make ze-rules-lint`, and commit all three digests with the rule |
| A `*.yang` file or a `ze:command` | `make ze-doc-verify`, `make ze-cli-grammar-check` |
| A plugin `register.go`, or anything generated | `make generate`, `make ze-plugin-imports-check` |
| A new package's placement | `make ze-tier-check` |
| Anything, once the commit script has run and it carried Go | `make ze-repository-tracked-build-check` -- the only check that compiles what git holds |
| A `scripts/dev/*.py` tool | its sibling `*_test.py` directly (python needs no build cache), then `make ze-unit-pkg-test PKG=./scripts/dev` |
| Several of the above, and you want breadth | `make ze-precommit-verify-changed` |
