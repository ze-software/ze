# Doc Update

Sync the wiki (`../wiki`) and in-tree docs (`docs/`) with all user-facing changes since the last documented commit.

See also: `/ze-commit` (commit after updating)

## Steps

1. **Read the sync point:** Read `../wiki/.source-commit`. This is the last main-repo commit that the wiki reflects.
2. **Enumerate the full delta:** Run `git log <source-commit>..HEAD --oneline --no-merges`. Filter to user-facing changes (features, fixes, refactors that change behavior, config, CLI, API). Exclude: AI/rules churn, spec lifecycle, test-only fixes, doc-only commits that already updated in-tree docs.
3. **Categorize and map:** For each user-facing commit, identify which wiki pages and in-tree docs are affected. Present a table to the user:

```
## Doc Sync: <source-commit-short>..<HEAD-short> (<N> user-facing commits)

| # | Commit | Summary | Wiki Pages | In-Tree Docs |
|---|--------|---------|------------|--------------|
| 1 | abc1234 | feat: DHCP server plugin | plugins.md, feature-inventory.md | docs/guide/dhcp.md |
| 2 | def5678 | feat: keepalive timer | peers.md, bgp.md | docs/guide/bgp.md |
...
```

4. **Confirm scope:** The table IS the plan. Proceed immediately unless the user redirects.
5. **Update each page:** For each wiki page that needs changes:
   - Read the current page
   - Apply the relevant changes (new sections, updated tables, corrected descriptions)
   - Keep the page's existing style and structure
   - Do NOT rewrite unchanged sections
6. **Update in-tree docs:** Same for `docs/` files if they need updates.
7. **Update `_Sidebar.md`:** If new pages were created, add them to the sidebar in the appropriate section.
8. **Update `feature-inventory.md`:** If features were added or changed, update the inventory.
9. **Bump `.source-commit`:** Write the current HEAD hash to `../wiki/.source-commit`. This is the LAST step, after all pages are updated.

## Rules

- **Enumerate before editing.** The table in step 3 must be complete before any page is touched. This prevents scope blindness (anchoring on one familiar feature and ignoring the rest).
- **`.source-commit` is bumped LAST.** Never bump it before all pages are updated. Bumping it claims all intermediate commits are documented.
- **One commit = one row.** Do not merge related commits into one row. Each commit may affect different pages.
- **Preserve existing content.** Only add or modify sections relevant to the delta. Do not rewrite pages from scratch.
- **No invented information.** If unsure what a commit changed, read the commit (`git show <hash> --stat`, then source files). Do not guess from the commit message alone.
- **Config changes require reading the YANG schema or config code.** Do not document config options from memory or commit messages alone.
- **New features require reading the source.** At minimum: the register.go, the main implementation file, and any functional tests.
- **Match the wiki's tone.** Pre-alpha banner, short paragraphs, tables for reference, code blocks for config examples.

## Scope Decisions

| Change Type | Document? | Where |
|-------------|-----------|-------|
| New feature (feat:) | Yes | feature-inventory.md + relevant topic page |
| Config syntax change | Yes | topic page config table + configuration-reference.md if it exists |
| CLI command change | Yes | command-reference.md + topic page |
| Bug fix with user-visible behavior change | Yes | topic page (if the fix changes documented behavior) |
| Performance improvement | Only if user-visible (new config option, new metric) | performance.md or topic page |
| Refactor (no behavior change) | No | -- |
| Internal fix (test, CI, tooling) | No | -- |
| API/RPC change | Yes | rest-api.md or grpc-api.md |

## Error Cases

| Problem | Action |
|---------|--------|
| `.source-commit` hash not found in git log | Ask user. The wiki may be ahead of or behind the repo. |
| Commit touches code you cannot read (deleted file, binary) | Use the commit message and diff stat. Note uncertainty. |
| Wiki page does not exist for a new feature | Create it. Add to `_Sidebar.md`. |
| Feature was added then modified in the same delta | Document the final state only. |
