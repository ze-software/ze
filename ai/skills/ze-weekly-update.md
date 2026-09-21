---
name: ze-weekly-update
description: Write and publish the Ze weekly update
---

# Weekly Update

Write the Zeledon weekly update, update the gh-pages site, and post the approved message to Discord `ze-news`.

Use this when Thomas asks for the weekly update, Zeledon update, Discord update, `ze-news` post, or weekly changelog.

See also: `/ze-status` for current work context, `/ze-doc-update` for broader docs sync, `/ze-commit` for preparing a commit after publishing.

## Hard gates

- Do not post to Discord until Thomas explicitly approves the exact text that will be posted.
- Do not publish unverified claims. Read the source, commits, docs, or generated pages before saying a feature shipped.
- Do not mention internal process in the public update: specs, acceptance criteria, review gates, agent sessions, learned summaries, commit-count bragging, or implementation bureaucracy.
- Write as Zeledon, not as Thomas. Use the project voice. If Thomas must be named, use third person.
- No em dashes. Use commas, periods, colons, or parentheses.
- The whole update fits in 3 Discord messages, 4 at the very most. Check with `./le weekly source <file>` before showing it to Thomas. Over budget means too many items, so cut items (`website/changes/discord/STYLE.md`, "How long").
- A fix gets one line. A new command, field, config leaf, counter, or default keeps its full spelling (`website/changes/discord/STYLE.md`, "How much detail").
- No repository vocabulary and no raw wire bytes in the post (`website/changes/discord/STYLE.md`, "Hard rules").
- Do not hand-edit generated site pages. Edit the source data or Markdown, then run the generator.
- Release progress remains an inventory preview until Thomas resolves the recorded bucket and ownership decisions. Do not publish unresolved classifications.
- Running this workflow for implementation verification authorizes only a dry run. It never authorizes a weekly post or Discord send.

## Required references

Read these before drafting:

1. `website/changes/discord/STYLE.md`.
2. The latest one or two files in `website/changes/discord/`.
3. `website/AI.md`, especially `Weekly Update Checklist`.
4. `website/data/topics.json` for allowed update tags.
5. The latest `website/changes/posts/*.md` post, to keep format and coverage continuity.

Website sources are in `website/`; the publishable artifact is generated into `../gh-pages`.

## Phase 1: Establish the week

1. Find the newest archived Discord post in `website/changes/discord/` and the newest website post in `website/changes/posts/`.
2. Determine the new `covers:` range from the previous post's end date unless Thomas gives a different range.
3. Collect the pinned release comparison below before drafting.
4. Gather what shipped during the range:
   - inspect `git log` for the range,
   - read the touched source, docs, specs, or tests needed to understand user-visible behavior,
   - verify the behavior at its producing function in the selected `release-to` tree,
   - put design or planning work only under `Coming up`.
5. Group the week by user-facing theme. Fold small commits into one capability when they serve one story.
6. A commit message describes the moment it was written. A quoted number can describe a defect before its fix. Read the producer before claiming delivery, including for removed release work items.

### Collect release progress

Resolve the current branch tip once with `git rev-parse --verify 'HEAD^{commit}'`.
Use that full ID for every history lookup in this draft. Pending filesystem
edits enter the report only after a commit and a new comparison.

1. Read the previous website source post's `release-to`. When present, use that
   commit as `release-from`. The Discord archive preserves the posted body, but
   does not copy these revision fields.
2. Resolve `release-to` at 23:59:59 UTC on the final day of `covers:`.
   Use the current branch's first-parent history from the pinned tip.
   For an owner-authorized in-progress update, record the exact UTC cutoff in
   the draft body and use it instead. Never invent a cutoff or use a future day.
3. If no previous `release-to` exists, resolve `release-from` at 00:00:00 UTC
   on the first day of `covers:` using that same history.
4. To resolve each time boundary, read
   `git log --first-parent --format='%H %cI' <pinned-tip>`.
   Select the first entry in first-parent order whose committer timestamp is
   at or before the boundary, after conversion to UTC. Record its full ID.
   Do not use author dates, relative dates, or side-branch commits.
5. Resolve a previous revision with `git rev-parse --verify '<release-to>^{commit}'`.
   An invalid revision, missing boundary, or unreadable tree is an error.
   If shallow history prevents resolution, stop and name the missing history
   to fetch. Never substitute `HEAD`, the current filesystem, or zero counts.
6. Run the shared report with the resolved full IDs:

   ```sh
   ./le spec roadmap compare from <release-from> to <release-to>
   ```

   Use the common `| json`, `| yaml`, or `| table` renderer when needed.
   Use `./le spec roadmap list revision <release-to>` for the same end snapshot.
   Read both endpoint inventories and their diagnostics. Do not scan specs to
   build a separate news inventory.
7. Read `immediate` and `pre-release` separately and together as the required
   total, then read the root nice-to-have group. Include every declared state.
   Skeleton, blocked, deferred, malformed, and `verification` items remain open.
   Keep diagnostics visible and retain the inventory-preview qualification.
8. Distinguish additions, removals, bucket moves, and status transitions.
   Explain changes in required totals with those categories.
   A move can also have a status transition. Neither fact establishes delivery.

This is an endpoint comparison of the remaining queue. An item added and
removed entirely between the endpoints is absent from the comparison.
Continue commit and source research for delivered behavior within the interval.
Counts measure release work items. They cannot establish effort, a completion
percentage, release readiness, or a release date. An empty queue proves none
of those claims.

### The RFC MUST programme is a standing item

Ze is being checked against every RFC it implements, one MUST at a time. Every
weekly update says so: that the work has started rather than finished, where it
stands, and the best two or three things it turned up that week. Owner
instruction, 2026-08-10. One section, and it lives under the same budget as
every other section.

Read the counts live. Never copy them from a commit message or a previous post.

| Fact | Source |
|------|--------|
| Total requirements, MUST-level, how many are checked | the header of `ai/RFC-REQUIREMENTS.md` |
| MUSTs still owing a test | the "Coverage by RFC" line in the same file |
| Documents whose requirement list has been checked against the RFC, and those not | `./le rfc extraction-status` |

State the limit honestly: a green run proves everything on the list, and does
not yet prove the list is complete. That is why the end-to-end reading is on
the roadmap.

**MUST comes before SHOULD, and `Coming up` keeps that order.** Close what the
checking found, then the MUSTs still owing a test, then the documents not yet
read end to end. SHOULD waits behind all of it.

### `Coming up` is chosen by Thomas, never composed

The forward-looking section MUST NOT be written from the queue comparison, from
spec names, or from what the week looked like from the outside. A spec file
existing says nothing about whether anybody is working on it, and its declared
status is the only status there is.

Build the candidate list, then ASK:

1. Take the additions and the status transitions from the comparison, plus every
   item the selected revision declares `design`, `ready` or `in-progress`.
2. Write one line per candidate: the user-facing capability, the declared
   status, and the file it comes from. Keep it to what moved this week or is
   declared `in-progress`.
3. Put that list to Thomas with `AskUserQuestion` and let him pick what appears.
   He can pick nothing, and then the section is omitted.
4. Write only what he picked, and use no status verb the declared status
   supports. `design` is "has a design" and never "is being written". `skeleton`
   is a captured idea and does not belong in the section at all.

A number belongs there only when it was counted off the list. Measured
2026-09-20: five audit items were published as "seven audits, one per area" over
a list of five, because the spec names ran 3 to 7 and the writer filled the gap
rather than counting what was in front of him.

## Phase 2: Draft the public post

1. Create or update `website/changes/posts/<covers-start>.md`.
2. Use this front matter:

```yaml
---
covers: <YYYY-MM-DD> .. <YYYY-MM-DD>
tags: <comma-separated allowed tags>
ze-stat-snapshot: true
release-from: <full resolved commit ID>
release-to: <full resolved commit ID>
---
```

Keep `ze-stat-snapshot: true` and both release revision IDs in front matter.
Weekly counts are historical facts. Write their values into the saved body
before approval. Never use live roadmap tokens or refresh historical prose
during site generation, sending, resuming, or archive handling.

3. Choose tags from `website/data/topics.json`. If the week needs a genuinely new topic, add it to `data/topics.json` with the right category. Do not force a near miss.
4. Decide what the week is about before writing, and leave the rest out. A full week yields far more than fits, so `STYLE.md` ("How long") governs what survives: 3 sections is normal, 5 is the ceiling, and a section carrying one bullet is a sentence in the wrong shape.
5. Write the body in Zeledon style:
   - `**📅 Ze Weekly Update**` header,
   - one short framing sentence,
   - themed sections with bold emoji headers,
   - bullets for multiple items,
   - `**🔭 Coming up**` only for work Thomas selected, in the words his
     declared status supports ("`Coming up` is chosen by Thomas", above).
   - a short release-progress paragraph with remaining release work items and
     relevant scope changes,
   - the public roadmap link: https://ze-software.net/project/roadmap/.
   Translate selected items into user-facing capabilities. Keep spec filenames,
   bucket names, and workflow terms out of the body. Planned capabilities stay
   under `Coming up`. Explain that the queue comparison can miss items added
   and removed between its endpoints. Until classification decisions are
   resolved, label the draft paragraph as an inventory preview and withhold
   publication. This paragraph and the standing RFC section share the existing
   message budget.
6. Dry-run the post and read the message count. Over 4, go back to step 4 and cut items. Do not compress the prose instead.
7. Run a self-review against the hard gates. Grep for what a grep can find rather than re-reading:
   - no em dashes,
   - no first person,
   - no internal process language,
   - no repo vocabulary (`extracted`, `enrolled`, `gated`, `polarity`, `ratchet`, `walk`, `carrier`, `artifact`, `tier`),
   - no raw hex or wire bytes,
   - no unverified shipped claims,
   - no hype,
   - fixes at one line, new surfaces named in full,
   - no sentence past about 30 words.
8. Show Thomas the exact dry-run messages and wait for approval before posting.

If Thomas asks only for a draft, stop after the draft. Do not post, archive, or regenerate the site unless asked.

## Phase 3: Publish after approval

After Thomas approves the exact dry-run messages, keep the source body and
revision fields fixed. A changed body requires a new preview and approval.

1. Run a dry run first:

```sh
le weekly source website/changes/posts/<covers-start>.md
```

2. Compare the chunk count and exact text with the approved preview. Existing date stamping can change a header as time passes. If any text differs, obtain approval again. Never recollect release counts here.
3. If Thomas wants a Discord preview, post to test only:

```sh
le weekly source website/changes/posts/<covers-start>.md channel ze-test confirm
```

4. Post to `ze-news` only after approval:

```sh
./le weekly source website/changes/posts/<covers-start>.md confirm
```

The posting tool refuses incomplete weeks unless `force` is used. Do not use `force` unless Thomas explicitly asks for an in-progress week to be posted. The tool archives the exact posted text to `website/changes/discord/<covers-start>-weekly.md` after a successful post.

5. **A run that stops partway has already put messages in the channel.** Read what it printed. It names the chunk that failed and the arguments that finish the post:

```sh
./le weekly source website/changes/posts/<covers-start>.md resume-from <N> confirm
```

Never answer a partial send by running the command again without `resume-from`. The archive is written only after the last chunk lands, so nothing records the week as posted, and a fresh run sends every chunk that already arrived a second time.

## Phase 4: Update the website

In `website/`, apply the checklist from `AI.md`:

1. Confirm the post has valid `tags:` front matter.
2. Check `data/features.json` and `data/milestones.json` for drift.
3. Check `docs/comparison.md` and `website/compare/bgp.md` for comparison drift (`website/compare/comparison.md` is a dispatch page, not the mirror).
4. Check whether a new lab page or `data/nav.json` Labs entry is needed.
5. Check whether `performance/index.html` needs fresh headline benchmark stats.
6. Run:

```sh
./le site build
./le site check
```

7. Verify these outputs exist and reference the new week:
   - `changes/<covers-start>/index.html`,
   - `changes/index.html`,
   - `changes/feed.xml`,
   - `index.html` homepage `Latest updates` cards, when the new week is within the rendered latest set.
8. Do not assume the homepage card count. Read `website/data/whats-new.json` and the staged `../gh-pages/index.html` before claiming the homepage is correct.
9. Use `./le site check` for the native artifact and page-mirror checks.

## Phase 5: Report

Report only grounded facts:

- the `covers:` range,
- the weekly source file path,
- the Discord archive path,
- whether `ze-news` was posted,
- which site files or data files changed,
- the `./le site build` and `./le site check` results,
- the pinned `release-from` and `release-to`, and any authorized UTC cutoff,
- any intentionally skipped drift item, with the reason.

Do not say the update is done unless the Discord post, archive, generated site, homepage card, feed, and native site checks are all accounted for.
