# Leftovers saved before the claude host shut down

Inert copies, not installed code. Nothing here was linted or tested.

**Update:** the Codex session code then landed (76bf9c345b..7af01375e9), and with it
the doc hunks and the peer image helper below. Only the first `functional-tests.md`
hunk, `more-todo`, `my-diff.patch` and `plan/roadmap.md` remain uncommitted; the
other copies here are now history.

The claude host carried uncommitted work that no commit or snapshot preserved.
Everything that stood on its own was committed (`715fa2833e..56fb983dba`). These
pieces could not land as code, so they are saved here as text.

| File | What it is | Why it is not committed as code |
|------|------------|---------------------------------|
| `tracked-hunks.patch.txt` | The remaining hunks of `ai/INDEX.md`, `interop.md`, `runner-architecture.md`, `documentation-testing.md`, `gh-pages.md` and `functional-tests.md`, against 56fb983dba | Their anchors name code that exists only in the Codex snapshot (`../33-tooling-one-off-reboot-assets/`): `runCatalogProcess`, `committedPluginRegistry`, `CleanupOwnedOrphans`, `run_identity.go`, `check_gobgp_reload*.go`. The first `functional-tests.md` hunk ("The driver reports setup...") has no producer anywhere |
| `internal_le_interoplab_peer_image.go.txt`, `..._test.go.txt` | A peer image helper and its test | Nothing at HEAD calls it; its only consumer is in the Codex snapshot |
| `more-todo.txt` | Three priority notes: command-surface parity, a plugin smoke run, a structure check-in | A note, not repository content |
| `my-diff.patch.txt` | A stale diff of interop, flowspec and GoBGP work | It applies neither forward nor reverse to the tree, and overlaps the Codex files |
| `plan_roadmap.md.txt` | Generated release roadmap output from 2026-10-23, showing 0 items | Stale, and a plan file may not sit at the root of `plan/` |

Restore the hunks only together with the Codex snapshot work they describe:
apply `tracked-hunks.patch.txt` with `git apply --3way` after that code is
back, and resolve each conflict against the current file. Never overwrite a
file whose bytes match neither side.
