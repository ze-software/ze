---
kind: table
level:
stage:
---
| You changed | Run this |
|-------------|----------|
| A `.go` file | `make ze-test-pkg PKG=<that package>`, or the group target covering it (`ze-test-bgp`, `ze-test-core`, `ze-test-plugins`, `ze-test-config`, `ze-test-cli`, `ze-test-rest`). Then `make ze-lint-changed` (`ai/rules/commands.md`) |
| Reactor concurrency (`reactor/session*.go`, `forward_pool*.go`, `peer.go`) | `make ze-race-reactor` (`ai/rules/testing.md`) |
| A `.ci` or `.et` test | its suite target: `make ze-plugin-test`, `ze-parse-test`, `ze-encode-test`, `ze-editor-test`, `ze-web-test`. Draft first in `test/draft/` |
| Linux-only code (`//go:build linux`) | `make ze-qemu-integration-test`, or `make ze-qemu-needs-linux-test` for a `needs-linux` `.ci` (`ai/rules/platform-linux.md`) |
| `rfc/short/*.md`, an `RFC requirement:` tag, `rfc/extraction/*` | `make ze-rfc-check` |
| `docs/**`, `ai/**`, `plan/**` | `make ze-doc-test`, and `make ze-verify-wiring-docs` for the changed-file gates |
| `ai/rules/*.md` | `make ze-rules-condensed` then `make ze-rules-lint`, and commit all three digests with the rule |
| A `*.yang` file or a `ze:command` | `make ze-doc-test`, `make ze-cli-grammar-check` |
| A plugin `register.go`, or anything generated | `make generate`, `make ze-plugin-imports-check` |
| A new package's placement | `make ze-tier-check` |
| Anything, once the commit script has run and it carried Go | `make ze-tracked-build-check` -- the only check that compiles what git holds |
| A `scripts/dev/*.py` tool | its sibling `*_test.py` directly (python needs no build cache), then `make ze-test-pkg PKG=./scripts/dev` |
| Several of the above, and you want breadth | `make ze-verify-changed` |
