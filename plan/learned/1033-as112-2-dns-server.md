# 1033 - as112-2-dns-server

## Context

Implements the as112 plugin's DNS-serving core: RFC 7534/7535 zone answers
(reverse-DNS sink for RFC 1918/link-local, EMPTY.AS112.ARPA DNAME
redirection), hostname/facility/location TXT identification, allow-from
access control, health checking, and doctor checks -- built on the shared
`internal/core/dnsserver` harness (spec-dns-server-harness) and spec-as112-1's
address-ownership registry. Three review rounds (2 independent agents each)
found 10 real bugs; see also [1032](1032-as112-review-hardening.md) for a
later, separate hardening pass done after this spec reached content-complete.

## Decisions

- Zone-boundary matching via `dns.IsSubDomain(zone, name)`, not raw
  string-suffix comparison: a suffix match treats a sibling name
  (`evil10.in-addr.arpa.`) as in-zone, answering NODATA instead of the
  correct NXDOMAIN -- a real security-relevant correctness bug (Run 1
  BLOCKER 1), not just a style preference.
- YANG listener anchors are `list ... { config false; ze:listener; }`
  directly under `container as112`, not `config false` wrapper containers:
  the wrapper form has zero operator-typeable content and never materializes
  in the parsed config Tree, so the entire cross-service port-conflict
  detection (`RegisterListenerDefault`/`CollectListenersWithDefaults`) was a
  permanent no-op (Run 1 BLOCKER 2) -- undetected because nothing exercised
  the materialized-Tree path in a test, only the YANG schema's own shape.
- `Manager.applied`'s "signature sticks only after a successful bind"
  invariant needs a FAILURE path too, not just a success path: fixing only
  "stick on success" (Run 1) left a good→bad→revert-to-good sequence wedged,
  since the middle failed `Apply` left `applied` unchanged (still the old
  good signature) and a third `Apply` reverting to that exact signature
  short-circuited as a no-op with zero listeners bound (Run 2 BLOCKER 1).
  Fixed by resetting `applied` to a sentinel (`unappliedSig`) on ANY failure
  path, not just leaving it untouched.
- A listener's accept-loop goroutine crashing (not a deliberate `Stop`) must
  invalidate `applied` and flip the liveness gauge, or a later `Apply` with
  the same desired endpoint set silently no-ops forever believing the dead
  listener is still up (Run 2 BLOCKER 2) -- fixed via `Stop`'s
  generation-counter bump compared against `serve`'s bind-time snapshot (see
  also 1032's NotifyStartedFunc fix, a related but distinct race in the SAME
  bind/Stop path found in the later hardening pass).
- `isOnBox`'s on-box carve-out must recognize the plugin's OWN anycast
  addresses, not just loopback: whether the kernel presents 127.0.0.1 or the
  destination anycast address itself as the apparent source of a same-box
  query bound on `lo` is routing/architecture-dependent, and the healthcheck
  probe is deliberately designed to query the real anycast address, not
  loopback -- if the kernel presents the anycast address and `allow-from` is
  configured, the probe would be wrongly denied, causing BGP to withdraw a
  healthy route (Run 3 BLOCKER).

## Consequences

- The address-family-aware health/doctor pattern
  (`defaultHealthTarget`/`wildcardHostsForFamily`) generalizes to any future
  single-stack-capable service: hardcoding one wildcard/target address
  silently breaks the ipv6-only (or ipv4-only) configuration, a class of bug
  that only shows up when someone actually deploys that mode, not in the
  default dual-stack path most testing exercises.
- `internal/core/dnsserver/manager.go`'s bugs (signature-sticking,
  generation-counter crash detection) were found while implementing as112
  but are in shared harness code also used by geodns -- both consumers
  benefit, and both were re-verified green after each fix.
- `parseHealthArgs`'s keyword-form bug (CLI dispatcher doesn't strip keyword
  tokens before an RPC handler sees them) is a recurring shape across this
  codebase's command handlers (established precedent:
  `internal/plugins/diag/cmd/tcp_check.go`'s `parseTCPCheckArgs`) -- any new
  plugin RPC handler accepting a `keyword <value>` argument form needs its
  own parse function following this pattern, the dispatcher will not do it.

## Gotchas

- A shared `internal/core/dnsserver` bug (signature-sticking on failure) can
  hide behind a NARROWER first fix that only handles the immediately-tested
  case (first-attempt failure) -- the good→bad→revert-to-good sequence that
  exposed the residual gap was found by a dedicated verify-agent re-checking
  the Run 1 fix against the code, not by extending the original test.
- `servedZones()` rebuilding a fixed 22-entry table with nested string
  building on every DNS query (hot path) is the kind of allocation-review
  finding that's easy to miss when focused on correctness bugs -- computed
  once into a package-level var instead.
- `requestTotal`'s metric undercounted allow-from-denied queries because the
  denied branch returned before the increment -- a metrics correctness bug
  with no functional/behavioral symptom, only caught by a reviewer
  specifically checking metric consistency across all early-return branches,
  not general code reading.

## Files

- `internal/plugins/as112/zones.go` + `zones_test.go` -- zone matching, SOA,
  DNAME redirection, TXT identification, `servedZones` caching
- `internal/plugins/as112/server.go` + `server_test.go` -- `answerQuery`,
  `isOnBox`, `allowed`, metrics
- `internal/plugins/as112/health.go` + `health_test.go` -- `as112 health`
  command, `parseHealthArgs`, `defaultHealthTarget`
- `internal/plugins/as112/doctor.go` + `doctor_test.go` -- bind-capability
  doctor check, family-aware wildcard probing
- `internal/plugins/as112/config.go` + `config_test.go` -- config parse/validate
- `internal/plugins/as112/state.go`, `metrics.go`, `show.go` -- state
  snapshot, metrics registration, `show as112`
- `internal/plugins/as112/yang/*.{yang,go}` -- YANG schema, listener anchors
- `internal/plugins/as112/integration_linux_test.go`,
  `freebind_integration_linux_test.go` -- real-kernel DNS wire proof
- `internal/core/dnsserver/manager.go` + `manager_test.go` -- `Apply`
  failure-path sentinel, generation-counter crash detection (shared with geodns)
- `internal/core/dnsserver/client.go` + `client_test.go` -- `RemoteAddr` 4-in-6 unmap
- `mk/test-integration.mk` -- `ze-integration-as112-test`
- `internal/core/diagnostic/codes.go` -- `doctor-as112-port-unavailable`
- `test/parse/as112-config.ci`, `test/plugin/as112-{enable,health,disable}.ci`
