# ExaBGP Migration

Ze is ExaBGP's successor. These documents map ExaBGP concepts to Ze and
describe the migration and compatibility tooling.

| Document | Purpose |
|----------|---------|
| `exabgp-migration.md` | How to migrate: `ze config migrate` and the `ze exabgp plugin` bridge |
| `exabgp-code-map.md` | File-by-file map from the ExaBGP Python codebase to Ze packages |
| `exabgp-comparison-report.md` | Feature-by-feature comparison of ExaBGP and Ze |
| `exabgp-differences.md` | Intentional behavioural and syntax differences |

The migration is external tooling only: the engine itself has zero ExaBGP
format awareness. See `../../internal/exabgp/` for the implementation and
`exabgp-migration.md` for the operator-facing guide.
