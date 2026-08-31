# Week of 2026-08-24

Standards closure was the plan for the week. The build and test tooling took it instead: the Makefile and 256 shell and Python scripts are gone, replaced by Go, and that move is still in progress. Around it, output formatting moved off flags and onto the pipe operators, and a TACACS+ authentication bypass was closed.

## 🖥️ CLI

New:

- Every command renders its answer through the pipe operators, so the rendering flags are gone. `--json` is removed from `config dump`, `config show`, `config validate`, `config diff`, `config fix`, `config completion`, `schema list`, `schema handlers`, `schema methods`, `schema events`, `yang tree`, `yang completion`, `tacacs show` and `interface scan`. `--yaml` is removed from `interface scan`, and `--format` from `config migrate`. Write `ze show config dump | json` instead. There is no alias and no deprecation period.
- `ze --plugins` is now `show plugins`. It returns one structured payload, so `| json`, `| yaml`, `| table`, `| count` and `| match` all work over it.
- `announce` is three commands, one per form: `announce unicast`, `announce blackhole` and `announce flowspec`. What an operator types does not change.
- `ze help <path>` prints a usage line built from the command model, naming the arguments a command actually takes. `ze help command "<path>" --json` now reports `usage`, `grammar`, `operators`, `answer-shape`, `address-fields` and `pipe-aliases`.
- The prompt color says what the session is doing: blue in operational mode, green in configuration mode, magenta after a command fails.

Fixed:

- A pipe operator a command cannot support is refused by name before the command runs, instead of being answered wrongly.

## 🛰️ Routing and subscribers

New:

- A per-neighbor `propagate-srv6-prefix-sid` leaf controls whether the BGP Prefix-SID attribute crosses an external session. It previously left the autonomous system with nothing asking it to (RFC 8669 Section 8).

Fixed:

- Two ranges on one FlowSpec field emitted two components of the same type. GoBGP answers that with a session reset, so an ordinary filter flapped the peering on every commit that touched it. The ranges now merge into one component (RFC 8955 Section 4.2).
- Every PPPoE subscriber session leaked its pool address on teardown, so the pool ran to exhaustion and only a restart recovered it.
- Changing `refresh-period` on a running box advertised one value and refreshed at another, so a neighbor's cleanup timer expired live LSPs. The advertised period, the refresh cadence and the state lifetime now come from one value (RFC 2205 Section 3.7).
- VPN, EVPN, MVPN, MUP, FlowSpec, VPLS and BGP-LS produced no best-path changes at all, so BMP mirroring, redistribution and telemetry saw nothing for those families.

## 🔒 Access, tunnels and configuration

New:

- Rotating a RADIUS or TACACS+ shared secret, address or timeout now takes effect on reload. It was accepted into the configuration and reached nothing until the daemon restarted, so SSH kept authenticating against the retired secret.
- Managed mode serves a named certificate from the PKI store, and the client can pin it by SHA-256 fingerprint including on first boot. `ze.managed.tls.insecure` is no longer needed for an authenticated deployment.

Fixed:

- A TACACS+ reply that claimed to be unencrypted was parsed as cleartext even with a shared secret configured. An off-path packet carrying PASS could therefore log a user in with no proof of the secret. Such a reply is refused now and the connection is dropped (RFC 8907 Section 10.5.2).
- If the interface named in the IPsec configuration had no address at boot, the box came up with no IPsec at all. That included peers carrying their own `local-address`. Startup now serves every peer it can bind, and a reload still refuses.
- A firewall `term` list reached the dataplane in random order on a first-match-wins path, and the BGP filter lists refused any entry past the first. The operator's order now travels with the list.

## 📚 Standards programme

Ze is being checked against every RFC it implements, one MUST at a time. The work has started rather than finished.

Of 4,748 requirements, 3,071 are MUST-level and 2,976 are checked. 68 MUSTs still owe work. Six of the 171 documents have been read end to end.

The 68 was 51 last week, and all 17 of the difference come from one document newly written up: RFC 7627, whose MUSTs have no tests yet. No requirement that was already checked was reclassified. Closing a gap moves a requirement between two states that both count as owing nothing, which is why a week of closures leaves that number flat.

A green run proves the written list, not its completeness. The remaining 165 documents still need end-to-end reading.

What that turned up this week:

- `local-as no-prepend` and `replace-as` put identical AS_PATHs on the wire. No prepend governs what is learned from the peer and relayed inward. Replace-as governs what is sent to it, and it alone drops the globally configured ASN outbound (RFC 7705 Section 3.3).
- With `accept-mode false`, a non-owner Active VRRP router still answered traffic addressed to the virtual address (RFC 9568 Section 6.4.3).
- Ze signed IKEv2 authentication with a hash the peer never advertised, and treated an empty algorithm list as permission to use any (RFC 7427 Section 4).

## 🔭 Coming up

Next week finishes the tooling move, brings the website back up to date, and spends the steadier tooling on defects. Standards closure follows: close what the checking found, then add tests for the 68 MUSTs that still owe work, then read the remaining 165 documents end to end. SHOULD-level work waits behind all of it.
