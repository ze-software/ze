# 705 -- cpe-1-pppoe-client

Spec: spec-cpe-1-pppoe-client.md
Date: 2026-05-15

## What this was

PPPoE client interface kind for ze's CPE/home-router use case. Ze already had a
PPPoE access concentrator (server) in `internal/component/pppoe/` and a full PPP
session manager in `internal/component/ppp/`. This spec added the client side:
dial an ISP's AC over a physical Ethernet interface, negotiate LCP/auth/NCP, and
present the resulting PPP session as a routable interface with server-assigned
addresses.

Motivated by replacing VyOS on a home router where the WAN uplink is PPPoE DSL.

## Architecture decisions

1. **Import cycle forced a new package.** `pppoe` imports `iface` (for
   `GetBackend`), so `iface` cannot import `pppoe`. Solved with `PPPoEDialer`
   interface defined in iface, implemented by `internal/component/pppoeclient/`.
   The pppoeclient package imports both `pppoe` (wire format) and `ppp` (FSM,
   DevPPPSetup). Registration via `init()` + blank import in `hub/main.go`.

2. **Client-mode PPP, not extending the PPP Driver.** The existing PPP Driver
   (`ppp.Driver`) is server/LNS-oriented: it sends auth challenges, assigns IPs
   from a pool, and uses external auth/IP event channels. Client-mode reverses
   all of that. Rather than adding client-mode branches to every server-side
   handler, the client drives the PPP FSM directly using the ppp package's
   exported pure functions (`LCPDoTransition`, `ParseFrame`, `WriteFrame`,
   `BuildLocalConfigRequest`, `BuildLCPEchoReply`, etc.). Zero risk to the
   existing L2TP/BNG PPP code path.

3. **Single reader goroutine for the session lifetime.** The negotiation phase
   (LCP+auth+NCP) creates a reader goroutine that feeds a `chan readFrame`. After
   negotiation, the same channel is passed to `keepaliveLoop`. No second reader,
   no fd race. This mirrors the PPP server's `readFrames` pattern but across
   phase boundaries.

4. **Reconcile pattern matches DHCP.** `reconcilePPPoEClients` in `register.go`
   follows the same shape as `reconcileDHCP`: desired-vs-active map diffing,
   config-change detection restarts affected clients, shutdown loop stops all.

## What surprised us

1. **`default:` in select is a hook violation.** The project's `block-silent-ignore`
   hook rejects empty `default:` cases in select statements. The non-blocking read
   pattern (`select { case <-stop: ...; default: read() }`) required restructuring
   to use a reader-goroutine + channel pattern instead.

2. **`strconv.FormatInt` is banned in production code.** The `block-sprintf-new`
   hook blocks it (use `textbuf` instead). MAC address formatting had to use
   `net.HardwareAddr.String()`.

3. **YANG `ze:os "linux"` prunes leaves on macOS.** The functional parse test
   needs `option=skip-os:value=darwin` because the YANG walker removes the
   `pppoe-client` list entirely on non-Linux, making `ze config validate` reject
   it as unknown.

## Mistakes and corrections (review findings)

| Finding | Severity | Root cause |
|---------|----------|------------|
| No shutdown cleanup for PPPoE clients | BLOCKER | Copied DHCP reconcile but forgot the shutdown loop |
| ChanFD/UnitFD leaked in Cleanup closure | BLOCKER | PPPoE server closes them via PPP Driver; client has no Driver |
| Two reader goroutines on same channel fd | BLOCKER | negotiateSession started a reader; keepaliveLoop started another |
| Echo failure off-by-one (4 failures, not 3) | ISSUE | `>` vs `>=` with post-increment |
| No config-change detection on reload | ISSUE | DHCP reconcile detects param changes; PPPoE did not |
| source-interface not validated | ISSUE | Other interface kinds validate their key name |
| Discovery socket read was blocking | ISSUE | SO_RCVTIMEO needed for stop-signal responsiveness |
| BuildPADR missing Relay-Session-Id echo | NOTE | RFC 2516 Section 5.3 MUST; only matters with relay agents |

All found and fixed during `/ze-review` passes before commit.

## Files

| Area | Files | Lines |
|------|-------|-------|
| YANG schema | `iface/yang/ze-iface-conf.yang` | +74 |
| Config parsing | `iface/config.go` | +91 |
| Client lifecycle | `iface/pppoe_client.go` | ~320 |
| Apply wiring | `iface/register.go` | +22 |
| Discovery dialer | `pppoeclient/dialer.go` | ~250 |
| PPP session | `pppoeclient/session.go` | ~550 |
| Auth/IPCP helpers | `pppoeclient/auth.go` | ~80 |
| Tests | `iface/pppoe_client_test.go`, `pppoeclient/auth_test.go` | ~500 |
| Wire format | `pppoe/discovery.go` | +33 |
| Kernel wrappers | `pppoe/kernel_linux.go`, `socket_other.go`, `ppp/ops.go` | +62 |
| Functional test | `test/parse/iface-netlink-accepts-pppoe-client.ci` | 20 |
| Total | 17 files | ~2400 |
