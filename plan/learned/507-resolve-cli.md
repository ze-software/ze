# 507 -- Resolve CLI Commands

## Context

The resolve component (`internal/component/resolve/`) consolidated DNS, Cymru, PeeringDB,
and IRR resolvers but had no CLI exposure. Operators needed a way to query these services
directly for debugging -- checking ASN names, prefix counts, AS-SET memberships, and DNS
records without a running daemon.

## Decisions

- Offline command (`cmd/ze/resolve/`) over online RPC because resolution queries are
  stateless and don't need daemon context. Same pattern as `ze data`, `ze interface`.
- One handler file per service (`cmd_dns.go`, `cmd_cymru.go`, `cmd_peeringdb.go`,
  `cmd_irr.go`) over a single dispatch file because each has its own flags and output format.
- Override flags (`--server`, `--dns-server`, `--url`) on every subcommand over hardcoded
  defaults because operators use local mirrors (PeeringDB), regional servers (IRR: rr.ntt.net,
  whois.ripe.net), and custom DNS. Without flags, the tool is untestable in CI.
- Context only for PeeringDB/IRR (30s timeout) over a universal context because DNS
  methods don't accept context.Context -- they use internal timeouts.
- Go unit tests with fake servers over .ci-only tests because the .ci test runner doesn't
  capture stderr for standalone foreground commands, and `captureRun` with `os.Pipe` gives
  full stdout/stderr assertion.

## Consequences

- All 4 resolvers testable end-to-end in CI with fake servers (DNS UDP, HTTP, TCP whois).
  No network dependency.
- `ze-test cymru` and `ze-test irr` fake servers added for future .ci tests if the test
  runner gains standalone stderr capture.
- Adding new resolve operations (e.g., `ze resolve dns mx`) is one function + one switch case.
- The `--dns-server` flag on cymru is a different name than `--server` on dns/irr. This is
  intentional: cymru's DNS is an implementation detail (it uses DNS for TXT queries), not the
  primary protocol. Operators think "which DNS server" not "which cymru server."

## Gotchas

- The auto-linter (`goimports`) strips aliased imports (`mdns "github.com/miekg/dns"`) between
  Edit calls if the using code doesn't compile yet. Fix: Write the entire file at once instead
  of incremental edits when adding aliased imports.
- The .ci parse test runner requires a `stdin=config` block even for commands that don't read
  config. Without it, the parser fails with "no config content found."
- The .ci test runner doesn't capture stderr for foreground commands that don't use `:stdin=config`
  in the `cmd=` line. Tests expecting `stderr:contains=` silently fail. Workaround: only assert
  exit codes in .ci tests; assert stderr content in Go unit tests.
- `flag.Parse` stops at the first non-flag argument. `ze resolve peeringdb max-prefix --url ...`
  does NOT work -- the `--url` must come before `max-prefix`. This is standard Go behavior but
  could confuse users. The help text shows flags before operations.

## Files

- `cmd/ze/resolve/main.go` -- dispatch + help
- `cmd/ze/resolve/cmd_dns.go` -- DNS handler with `--server`
- `cmd/ze/resolve/cmd_cymru.go` -- Cymru handler with `--dns-server`
- `cmd/ze/resolve/cmd_peeringdb.go` -- PeeringDB handler with `--url`
- `cmd/ze/resolve/cmd_irr.go` -- IRR handler with `--server`
- `cmd/ze/resolve/resolve_test.go` -- 13 end-to-end tests with fake servers
- `cmd/ze-test/cymru.go` -- fake Cymru DNS server
- `cmd/ze-test/irr.go` -- fake IRR whois server
- `cmd/ze-test/peeringdb.go` -- extended with `irr_as_set` field
- `cmd/ze/main.go` -- dispatch + help entry
- `test/parse/resolve-*.ci` -- 6 functional tests
- `docs/architecture/resolve.md` -- CLI section + cache table fix
- `docs/guide/command-reference.md` -- resolve section
