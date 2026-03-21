# 380 -- SSH Server

## Objective

Add an SSH server component to Ze that serves a read-only session over SSH using Charm's Wish library, with username/password authentication via bcrypt.

## Decisions

- SSH server is a `ze.Subsystem` (Start/Stop/Reload lifecycle managed by Engine)
- YANG schema `ze-ssh-conf` follows the hub pattern: embed.go + register.go + init()
- Password hashes stored as bcrypt in config; field named `Hash` to avoid gosec G117
- SessionModel uses pointer receivers to avoid gocritic hugeParam (6440 bytes from embedded bubbles components)
- No hot-reload for v1 -- config changes require restart

## Patterns

- Schema registration: embed YANG file via `//go:embed`, register in `init()` via `yang.RegisterModule()`
- Schema loading: add `loader.GetEntry("ze-ssh-conf")` block in `YANGSchemaWithPlugins()`
- Blank import in `plugin/all/all.go` triggers init() for schema registration
- Wish middleware chain: bubbletea.Middleware (per-session tea.Program) + activeterm.Middleware (PTY)
- Password auth callback: `wish.WithPasswordAuth(func(ctx ssh.Context, pass string) bool)`

## Gotchas

- gosec G117 flags any exported struct field matching "password" pattern -- use "Hash" instead
- gocritic hugeParam: textinput.Model + viewport.Model make structs large; use pointer receivers
- goimports auto-removes imports on save -- must add import + usage in same edit
- `errors.Is()` required for error comparison (errorlint)
- SECURITY: omitting `authentication {}` caused Wish to set `NoClientAuth = true` -- always register a password handler that rejects all when no users configured
- SECURITY: `AuthenticateUser` had timing side-channel -- unknown users returned instantly vs bcrypt ~100ms for known users, enabling username enumeration. Fix: always run bcrypt against a dummy hash for unknown users
- `ListenAndServe()` in goroutine meant bind failures were invisible to `Start()` caller -- use synchronous `net.Listen` then `srv.Serve(listener)` in goroutine
- No double-start guard leaked first server -- protect `s.wish` with `sync.Mutex`, check nil before Start
- `max-sessions` was stored but never enforced -- added middleware with `atomic.Int32` session counter
- `NewServer` returned `(*Server, error)` but error was always nil -- validate HostKeyPath non-empty
- YANG `idle-timeout` description said "0 = disabled" but code applied 600s default for 0 -- fixed description to say "default 600"

## Files

- `internal/component/ssh/ssh.go` -- Server struct, Start/Stop, Wish setup
- `internal/component/ssh/auth.go` -- CheckPassword, AuthenticateUser (bcrypt)
- `internal/component/ssh/session.go` -- SessionModel (per-SSH-session tea.Model)
- `internal/component/ssh/schema/ze-ssh-conf.yang` -- YANG schema
- `internal/component/ssh/schema/embed.go` -- embedded YANG
- `internal/component/ssh/schema/register.go` -- init() registration
- `internal/component/config/yang_schema.go` -- added ze-ssh-conf loading
- `internal/component/plugin/all/all.go` -- added blank import for ssh/schema
