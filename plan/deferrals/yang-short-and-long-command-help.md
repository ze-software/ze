# Deferrals: yang-short-and-long-command-help

Rows deferred from `spec-yang-short-and-long-command-help`, now closed. Each row
names where the work goes, so nothing is recorded without a destination.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-31 | spec-yang-short-and-long-command-help | The short/long split for CONFIG nodes: 1241 leaf and leaf-list descriptions, 443 container and list descriptions, 277 enum descriptions, and the nine config renderers that each guess a length today | The owner scoped this work to commands. The config corpus is larger, spreads wider (fourteen descriptions carry a paragraph break, one runs past 900 characters), and carries an obligation commands do not: `ai/rules/config.md` requires a leaf description to name any environment variable that overrides the leaf, so a split must first decide which half carries that contract | `plan/future/spec-yang-config-leaf-short-and-long-help.md` | deferred |
