---
kind: directive
level: MUST
stage:
---
- Go 1.21+ features (slog, generics)
- `golangci-lint` MUST pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context as first param: `context.Context`
- Code MUST NOT strip `ctx context.Context` parameters from function signatures. "Clean unused context" means remove dead `import "context"` lines only. Parameters stay even if the current body doesn't use ctx (propagation, cancellation, future use).
- Fail-early: code MUST propagate parse/config errors immediately, and MUST NOT silently default
