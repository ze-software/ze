# 553 -- cmd-4 Docs Sweep

## Context

cmd-4 phase 1 (`plan/learned/548`) and phase 2 (`plan/learned/552`) shipped
the `bgp-filter-prefix` plugin in full, including per-prefix NLRI rewriting
for the modify path. The feature worked end-to-end but was not documented
anywhere user-facing: `docs/features.md`, `docs/guide/plugins.md`,
`docs/guide/configuration.md`, and `docs/comparison.md` still read as if
prefix-list filtering did not exist. The original cmd-4 handover deferred
the doc sweep on the theory that cmd-5 (AS-path filter), cmd-6 (community
match), and cmd-7 (route modify) would add their own filter types, so one
combined sweep after all four landed would touch each file only once. That
reasoning still holds for cmd-5/6/7 -- but with cmd-4 sitting in main and
no public documentation, operators have no way to know the feature exists.

## Decisions

- **Ship the cmd-4 subset of the sweep now; leave cmd-5/6/7 pages open.**
  Touching the four doc files for cmd-4 adds ~150 lines total and leaves
  the files ready for cmd-5/6/7 to slot their own subsections in without
  structural edits. The alternative (keeping the feature entirely
  undocumented until cmd-7 lands) makes `ze` look less capable than it is
  and gives operators no syntax reference.
- **`docs/features.md`: inline bullet only.** The feature list is already a
  single-line-per-feature table. Extended cmd-4 to the existing Plugins
  line by adding `prefix-list filters` and a source anchor. No new row.
  This keeps features.md honest without restructuring -- cmd-5/6/7 will
  append the same way.
- **`docs/guide/plugins.md`: new "Prefix-List Filter (`bgp-filter-prefix`)"
  subsection under Redistribution Filters.** Covers the three chain-ref
  forms (`<plugin>:<filter>`, `<filter-type>:<filter>`, bare name), the
  five-row action matrix (single accept/reject, multi accept/reject, mixed
  modify), and the IPv4-only caveat for the mixed modify path. Positions
  it so cmd-5/6/7 can add their own subsections as siblings without
  shuffling headings.
- **`docs/guide/configuration.md`: new "Prefix-List Filter" subsection
  under Redistribution Filters.** Documents the full YANG syntax:
  `bgp { policy { prefix-list NAME { entry <CIDR> { ge; le; action; } } } }`,
  the semantics (first-match-wins, implicit deny, default ge/le), a full
  example including a filter chain reference, and the multi-prefix modify
  behaviour. Source anchors point at the YANG schema and the Go parser so
  future doc edits can verify syntax claims against code.
- **`docs/comparison.md`: flip `Prefix matching (ge/le)` from No to Yes and
  add a policy-intro sentence enumerating the three built-in filter
  plugins.** The old "No" value was from before cmd-4. The enumeration
  makes it clear that ze ships first-class filter plugins, which is the
  distinguishing factor for operators comparing against BIRD / GoBGP /
  OpenBGPd. cmd-5/6/7 will flip additional cells from No to Yes when they
  land.
- **Rejected: adding a standalone `docs/guide/prefix-list.md` page.**
  Would bloat the docs tree for a single filter. The existing
  `docs/guide/redistribution.md` is the right umbrella for filter docs;
  when cmd-5/6/7 land, that page can be promoted to a per-filter-type
  reference.
- **Rejected: backfilling `docs/guide/redistribution.md` with cmd-4 now.**
  redistribution.md is the natural home for detailed examples, but per the
  handover's rationale the per-plugin deep dives belong in a single
  post-cmd-5/6/7 sweep. The cmd-4 subset in plugins.md and
  configuration.md is the minimum to make the feature discoverable.

## Consequences

- Operators searching `docs/features.md` or `docs/comparison.md` for
  prefix filtering now find it. Previously the answer was "look at the
  Go source".
- cmd-5/6/7 specs can now add their own subsections under
  `docs/guide/plugins.md` Redistribution Filters and
  `docs/guide/configuration.md` Redistribution Filters without touching
  the cmd-4 subsection. The structural shape is in place.
- `docs/comparison.md`'s prefix-matching cell now matches reality. Future
  comparisons (AS-path regex, community matching) will flip the same way
  as cmd-5/6/7 land.
- The cmd-5/6/7 docs sweeps remain open deferrals. They are no longer
  blocking cmd-4 visibility, so they can be worked sequentially with each
  filter type.

## Gotchas

- **The existing text in `docs/guide/configuration.md` and
  `docs/guide/plugins.md` is still marked "(planned)" for Redistribution
  Filters.** The "(planned)" header predates cmd-4 and refers to the
  broader redistribution framework vision; cmd-4 only implements a subset
  of that. Left the "(planned)" label in place to avoid overclaiming -- a
  future session that promotes redistribution from planned to shipped
  should rename the headers at the same time.
- **`docs/features.md` has a pre-existing `community filters` item in the
  Plugins line.** Added `prefix-list filters` next to it rather than
  spinning up a new row. If cmd-5/6/7 add "AS-path filters" and
  "community match filters" and "route modify filters", the list grows to
  a comma-separated sentence which is acceptable per the file's table
  conventions. Restructuring into a dedicated Filter Plugins row is
  deferred to the cmd-0 umbrella.
- **The chain-ref form section lists three forms.** The "bare name"
  resolution depends on whether another plugin has registered a filter
  with the same name. If cmd-5/6/7 add `CUSTOMERS`-named filters for
  different plugins, the bare form becomes ambiguous and the engine
  defaults to the filter-type resolver order documented in
  `internal/component/plugin/registry/registry.go`. The docs do not
  re-explain that; the FilterTypes resolver already has its own godoc.
- **Source anchors point at both the plugin code and the YANG schema.**
  Keep both in sync when renaming things. A rename of the YANG list name
  (unlikely) would need the YANG anchor updated; a refactor of
  `parsePrefixLists` in config.go would need the Go anchor updated. Check
  the anchors before editing either.

## Files

- `docs/features.md` -- Plugins row: added `prefix-list filters` and a
  source anchor
- `docs/guide/plugins.md` -- new "Prefix-List Filter (`bgp-filter-prefix`)"
  subsection under Redistribution Filters with action matrix and
  chain-ref forms
- `docs/guide/configuration.md` -- new "Prefix-List Filter" subsection
  under Redistribution Filters with full YANG syntax, entry field table,
  and multi-prefix modify notes
- `docs/comparison.md` -- flipped `Prefix matching (ge/le)` cell from No
  to Yes; added the built-in filter plugin enumeration in the policy intro
