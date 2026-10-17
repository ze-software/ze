---
kind: directive
level: MUST
stage:
---
**A command that acts on a set (clear, delete, flush, reset, withdraw, update, refresh, restart, disconnect) MUST NOT act on every member when its selector is omitted: the everything form MUST be an explicit selector word, `all`, or the typed wildcard the family already uses (`peer *`).** The bare prefix that stops before the selector MUST be refused, and the refusal MUST name the explicit forms (`clear dns cache` answers with `clear dns cache all` and the narrower selectors), so a typo, an early Enter or a tab completion never reaches the all-effect. The old bare form MUST NOT survive as an alias. The selector is mandatory in both declarations `docs/architecture/cli/command-verbs.md`, "Action verbs and their selector", names: the YANG leaf and `RequiresSelector`.
