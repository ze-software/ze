# Measure: do news fragments carry the weekly update's facts

The weekly update needs five facts no commit message carries: what an
operator hit before the change (with what they see after), the earlier
ze-news claim it corrects, the configurations a security fix exposed, a bug
reported upstream, and who outside the project asked for it. News fragments
(`website/changes/fragments/README.md`) are meant to carry them. This file
records, for the next two weekly updates, where each fact of each published
item came from, so the owner can judge whether fragments replace the research
or whether their rules need tightening.

`/ze-weekly-update` appends one table per update once it is published
(`ai/skills/ze-weekly-update.md`, Phase 1, step 9). Nothing here is published.

## Table shape

Each update adds a section headed by its range, whatever its length:

`## <release-from>..<release-to>`

followed by one row per item published in that update:

| Item | Before | Corrects | Exposure | Upstream | Asked by | Fragment |
|------|--------|----------|----------|----------|----------|----------|

- **Item:** the published item, in a few words.
- **Before** through **Asked by:** one of `fragment` (a fragment carried the
  fact and it passed verification at the producer), `research` (no fragment
  carried it, so the writer found it), `failed` (a fragment carried it and
  verification refuted it), or `n/a` (the item has no such fact). Before
  covers the Before and After headings together.
- **Fragment:** the fragment file's name, or `none` when the item had none.

Rows of `research` for a fact a fragment marked `none`, and items with no
fragment, are the signal that the fragment rules need tightening.
