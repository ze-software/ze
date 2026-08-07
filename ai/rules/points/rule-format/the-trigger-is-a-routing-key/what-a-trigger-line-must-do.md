---
kind: table
level: MUST
stage:
---
| Requirement | Why |
|-------------|-----|
| Start with a temporal opener (`when`, `whenever`, `before`, `after`, `while`, `during`, `if`, `once`, `unless`, `upon`, `on`, `at`, `any/every/each time`, `prior to`, `as soon as`) or a gerund (`writing`, `adding`, `reviewing`, `naming`, `closing`, ...) | A uniform opening makes the column scannable, and both forms force a situation rather than an assertion |
| Name what the author is DOING or what has HAPPENED, never what they must do | "All CLI commands MUST follow these patterns" matches every task and therefore routes nothing. "adding or changing a CLI subcommand, flag, or exit code" routes |
| One complete clause, one line | A trigger that ends on a comma, a dangling `by`/`with`/`the`, or an unbalanced `**` was copied out of a wrapped bold body line. Three such triggers shipped into the digest unnoticed |
| Do not restate the directive | The directive belongs under `## Directives`, where the digest picks it up. Duplicating it in the trigger costs tokens in every session and routes nothing |
