---
kind: directive
level:
stage:
---
- Go 1.21+ features (slog, generics)
- `golangci-lint` must pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context as first param: `context.Context`
- Never strip `ctx context.Context` parameters from function signatures. "Clean unused context" means remove dead `import "context"` lines only. Parameters stay even if the current body doesn't use ctx (propagation, cancellation, future use).
- Fail-early: propagate parse/config errors immediately, never silently default
