# Command Equivalents Critical Review

Date: 2026-07-04

## Scope

This review covers the command-equivalents website work and the DNS cache command split:

- `../gh-pages/tools/render-command-equivalents.py`
- `../gh-pages/tools/build.py`
- `../gh-pages/data/command-equivalents.json`
- `../gh-pages/data/cli-commands.json`
- `../gh-pages/command-equivalents/`
- `../gh-pages/assets/site.css`
- `internal/component/resolve/cmd/dns.go`
- `internal/component/resolve/cmd/show_dns.go`
- `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang`
- command docs and generated website mirrors

## Verdict

The direction is correct, but the first pass is not merge-ready. The generated page can drift from the YANG-backed CLI catalog, the syntax renderer can publish wrong arguments, and the DNS split needs inventory, docs, and test coverage updates.

## Findings

### Blockers

1. **Command Equivalents can build from stale CLI data.**
   `render-command-equivalents.py` reads `data/cli-commands.json` directly. If the `cli` build step is skipped, new YANG commands do not appear. The current generated page still shows aggregate DNS cache commands instead of the split stats, list, and record forms.

2. **Generated syntax is not authoritative.**
   The renderer builds syntax from `path` plus `args`. Some commands have incomplete or mismatched arg metadata, so generated pages can contradict their own `Usage:` text. Examples include `create interface address` and `request log level`.

3. **Split DNS handlers allow action override through args.**
   `handleDNSCacheStats` and `handleClearDNSCacheStats` prepend the split action and then call the old generic parser. A caller can pass another action token and override the split operation.

4. **Wire-method inventory is stale.**
   New split RPC methods are registered in code but absent from `internal/component/plugin/all/testdata/wire-methods.snapshot`.

### Issues

5. **DNS vendor mapping still names the old aggregate command.**
   `data/command-equivalents.json` maps DNS lookup/cache intent to `show dns cache`, not the new split show commands.

6. **API command documentation is stale.**
   `docs/architecture/api/commands.md` still lists `ze-show:dns-cache` instead of the split DNS cache methods.

7. **Functional coverage is incomplete.**
   `test/plugin/dns-cache-show.ci` dispatches `show dns cache stats` only. It should also dispatch `show dns cache list` and `show dns cache record <name>`.

8. **Vendor-only gaps are counted but not rendered.**
   The page reports vendor-only gap notes, but entries such as LLDP and NAT are not shown or searchable.

9. **Project dropdown mobile layout can break.**
   The desktop `#nav-panel-project` ID rule has higher specificity than the mobile `.nav-dropdown-panel` reset.

10. **Maintenance docs omit the CLI refresh step.**
    `../gh-pages/AI.md` tells maintainers to rebuild command-equivalents without first refreshing the CLI catalog.

11. **Source anchors reference a non-existent generator symbol.**
    Docs mention `complete_entries`, but the generator uses `build_rows`.

12. **The vendor command dataset is curated, not exhaustive.**
    This should be stated in the page until a vendor-command catalog layer exists.

## Resolution plan

1. Make `command-equivalents` regenerate the CLI catalog or depend on the `cli` step.
2. Add authoritative syntax generation to the CLI catalog and consume that field in the command-equivalents renderer.
3. Split DNS cache handlers so each method executes only its own operation and rejects unexpected args.
4. Update wire-method snapshot, API docs, command docs, and generated website output.
5. Add regression tests for handler override protection and end-to-end show cache list/record dispatch.
6. Render vendor-only gap notes.
7. Fix mobile Project dropdown CSS.
8. Update maintenance docs and source anchors.
9. Rebuild affected website artifacts and run targeted verification.

## Resolution status

All findings above were resolved in the follow-up implementation:

- `render-command-equivalents.py` now loads commands through the CLI catalog loader, so a command-equivalents build refreshes `data/cli-commands.json` from `bin/ze help command --json` when the binary is available.
- `render-cli-catalog.py` rejects a `bin/ze` built with the `zetest` tag before writing public command docs, preventing test-only command leakage.
- `render-cli-catalog.py` annotates cached commands with a `syntax` field derived from explicit `Usage:` text. Command Equivalents uses that syntax before falling back to structured args.
- DNS cache handlers now execute only their exact split operation and reject unexpected action-like args.
- DNS cache `record` responses return an empty entries list instead of JSON `null` when no records match.
- The wire-method snapshot includes the split DNS cache RPCs.
- `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, and `docs/guide/command-catalogue.md` were updated.
- The generated Command Equivalents page shows split DNS cache rows, source-backed syntax, and vendor-only gaps.
- The page now states that vendor commands are curated migration hints, not exhaustive vendor CLI catalogs.
- The Project dropdown has a mobile-specific override.
- Regression tests cover DNS override rejection, show stats/list/record handler shapes, the wire-method snapshot, and the show DNS cache functional path.
