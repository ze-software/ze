# 1094 -- followup-web-cli-ux

## Context

Umbrella of user-facing web/CLI follow-ups closed over four sessions. This final
session landed the last three groups: AC-10 (a shared error-capturing render
writer so `ze help | head` exits non-zero instead of silently truncating),
AC-5 (a web-route registry replacing hardcoded L2TP/IS-IS/OSPF/gokrazy routes in
the hub), and AC-6/AC-7 (French i18n proving locale, a 390px mobile CSS
breakpoint, and `.wb` harness directives for login/multi-user, locale, and
viewport). The environment cannot launch Chrome (missing `libatk`), so every
browser-tier `.wb` proof is env-blocked and the ACs are proven at the Go tier.

## Decisions

- **renderWriter owns its print methods (`Str`/`Line`) over `fmt.Fprintln(rw,...)`.**
  Session 3's design assumed `fmt.Fprintln`/`fmt.Fprint` to a custom writer (hook-allowed).
  But `errcheck` flags every unchecked `fmt.Fprintln(customWriter,...)`, which would force
  ~250 `//nolint` comments. Giving `RenderWriter` return-less `Str`/`Line` methods that
  capture the error internally keeps call sites free of both the banned `fmt.Fprintf` AND
  any error check.
- **Byte-identity verified by sorted-set diff, not line diff.** `ze help ai` output is
  non-deterministic (map iteration in `aihelp.Services`), so a plain diff shows spurious
  changes. Proof = compare sorted line multisets with the known-nondeterministic lines
  excluded; confirmed the same binary varies run-to-run to isolate that from the conversion.
- **WebRoute contract = `{Pattern, Wrap, Build}` plus optional `Enabled`/`Portal`.** The
  minimal settled shape covers l2tp/isis/ospf; gokrazy needs an env-gate and a portal-menu
  entry, so two optional fields model it rather than a bespoke path or a hub special-case.
- **All four features register from the web package (dedicated `register_*.go`).** Chose this
  over gokrazy self-registering from its own component behind `ze_web`, because the web
  package is always compiled (no composition-root / `make generate` change) and the wrap
  helpers live in the hub (web carries only pattern+kind+builder, keeping web free of hub
  internals). gokrazy's register file imports `zegokrazy`+`env`.
- **i18n = catalog + English fallback, wired as a `t` template FuncMap helper.** `Translate`
  falls back locale-catalog -> englishBase -> key text (never blank). Scoped to the login
  page (the proving locale's `.wb` page) rather than a full template sweep.
- **`.wb` server-auth is decided by pre-parsing the file.** A test declaring `option=auth`
  drops `--insecure-web` and the harness bcrypt-seeds the declared admin; tests without it
  keep the fast single-implicit-admin path unchanged.

## Consequences

- CLI render paths (`ze help*`, usage pages, version) now return a non-zero exit on a broken
  pipe; the contract lives in `helpfmt.RenderWriter`. New render code should route through it.
- Adding/removing an in-tree web feature's routes is a `register_*.go` edit, never a
  `service_web.go` edit. `http.ServeMux` panics on duplicate patterns, so a stray leftover
  hardcoded route surfaces immediately at server start.
- The i18n pipeline exists but only the login page is translated; extending coverage is
  adding keys to `englishBase`/`catalogs` and `{{t .Locale "key"}}` in templates.
- `.wb` tests can now assert mobile layout (`option=viewport`), locale (`option=locale`), and
  authenticated flows (`option=auth` + `action=login`); the runner drives real
  `agent-browser set viewport`/`set headers` commands.

## Gotchas

- `gosec` G101/G117 false-positives on translation keys named `login.password` and on a
  `WBAuthUser.Password` field, and on a `passwordHash :=` local -- all need `//nolint:gosec`.
  `misspell` flags the correct French word "Connexion" (British/American dict) -- `//nolint:misspell`.
- The write-edit hook re-scans an Edit's `new_string`, so an edit that merely *includes* an
  existing `switch args[0]` line is blocked by the switch-dispatch guard; target the case
  bodies instead. `fmt.Fprintf(os.Stderr,...)` is hook-exempt but `fmt.Fprintf(customWriter,...)`
  is banned.
- `c_ignored_errors` fires on `_test.go` too: `_, _ = w.Write(...)` in a test is blocked; use
  `if _, err := ...; err != nil { t.Fatal }` or a return-less path.
- The bash hook blocks the literal string `git build`... `go build` without `-o bin/`, `/tmp`
  paths, and `| tail` pipes -- capture to a file and Read instead.
- `agent-browser` is present but Chrome cannot launch (`libatk-1.0.so.0`, exit 127); the
  `installFakeAgentBrowser` test seam (logs commands to a file) is how the runner's
  browser-command plumbing is unit-tested without a real browser.
- `helpfmt` exists twice: `internal/core/helpfmt` (canonical) and `cmd/ze/internal/helpfmt`
  (a re-export shim). New types (RenderWriter) live in core and are re-exported by the shim.

## Files

- `internal/core/helpfmt/renderwriter.go` (+test), `internal/core/helpfmt/helpfmt.go` (WriteTo seed)
- `cmd/ze/internal/helpfmt/helpfmt.go` (RenderWriter re-export)
- `cmd/ze/help_ai.go`, `cmd/ze/help_command.go`, `cmd/ze/dispatch.go`, `cmd/ze/ze_core_dispatch.go`, `cmd/ze/render_exit_test.go`
- `internal/component/cli/editor.go` (prompt conversion)
- `internal/component/web/webroute.go`, `register_l2tp.go`, `register_isis.go`, `register_ospf.go`, `register_gokrazy.go`, `webroute_test.go`
- `internal/component/web/i18n.go` (+test), `render.go` (FuncMap + LoginData.Locale), `templates/page/login.html`, `assets/style.css`, `mobile_test.go`
- `cmd/ze/hub/service_web.go` (route iteration, login locale)
- `internal/component/web/testing/parser.go`, `runner.go`, `directives_test.go`
- `internal/test/cli/cmd_web.go`, `cmd_web_auth_test.go`
- `test/web/web-i18n-fr.wb`, `web-mobile-layout.wb`, `web-rbac-admin.wb`, `web-rbac-readonly.wb`
