---
spec: rfc-release-layer-map
topic: RFC Compliance
---

## Before

The public RFC compliance and use-case profile pages showed requirement counts
and dependency tables per RFC, but readers had no interactive way to inspect
how RFCs depend on one another, filter around a proposed capability boundary
such as base IPv4 BGP, separate MUST, SHOULD, and MAY requirements while
keeping the full mandatory test results intact, or trace which planned work
items relate to a given RFC and which remain unmapped.

## After

A new interactive RFC dependency map is live at `/quality/rfc-map/` (with a
Markdown mirror and a static no-JavaScript fallback), linked from the
Evaluate -> Evidence navigation menu and the quality, RFC compliance,
use-case profile, and roadmap sidebars. Readers can browse proposed
capability groups, existing use-case profiles, technologies, or all recorded
standards; inspect citation, update, and obsolescence edges and cycle
components without silently expanding scope; filter requirements by exact
normative keyword (`MUST`, `SHOULD`, `MAY`, or mandatory family) while
keeping canonical mandatory test results from the per-RFC evidence records
unchanged; see gradient-tinted completion cards matching the recorded result
bar; inspect linked and unmapped work items alongside input SHA-256 hashes
and explicit unknown-age provenance; and navigate with keyboard focus
preserved and no horizontal overflow on narrow screens.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
