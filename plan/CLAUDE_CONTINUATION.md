# Claude Continuation State

**Last Updated:** 2025-12-21

---

## CURRENT PRIORITY

**ExaBGP Interop Testing** - Investigating remaining test failures

---

## ACTIVE WORK

### Self-Check Tests

**Status:** ~27/37 passing

**Recent Fixes:**
1. ASN4 capability default → `true` (was `false`)
2. Template inheritance for local-as, peer-as, hold-time, family, capability
3. Process group killing with `syscall.Kill(-pid, SIGKILL)`
4. Concurrency limit (4 workers) to prevent resource exhaustion

**Remaining Failures (message mismatch):**
- `flow-redirect` - Redirect action encoding
- `extended-nexthop` - Extended next-hop handling
- `prefix-sid` - Prefix-SID attribute (code 40)
- `parity` - Unknown
- `srv6-mup-v3` - SRv6 MUP encoding
- `srv6-mup` - SRv6 MUP encoding
- `watchdog` - Watchdog feature
- `path-information` - Path-ID/add-path
- `split` - Message splitting
- `vpn` - VPN routes

---

## RECENT COMMITS

- `ee3a8c8` Fix all 42 golangci-lint issues
- `d209e49` Add RFC 9136 and update commit protocol to include lint
- `8c7173b` Complete ExaBGP alignment: Phase 8-9 (error subcodes, config)

---

## TEST STATUS

✅ **All unit tests pass** (`make test`)
✅ **Lint clean** (`make lint` - 0 issues)

---

## KEY FILES

| Purpose | File |
|---------|------|
| Self-check tool | `cmd/self-check/main.go` |
| Config parser | `pkg/config/bgp.go` |
| This file | `plan/CLAUDE_CONTINUATION.md` |
| Protocols | `.claude/ESSENTIAL_PROTOCOLS.md` |

---

## NOTES

- All code changes require TDD (test first, show failure, implement, show pass)
- Plans go in `plan/`, protocols go in `.claude/`
- Check ExaBGP reference before implementing BGP features
- **ALWAYS run `make test && make lint` before requesting a commit**
