# A message the peer waits for is never sent

The session starts and nothing errors, but a message the protocol or the test
expects next never reaches the wire, so the run ends on a timeout that names
no cause.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | route-policy-0-umbrella (found, not caused) | `test/plugin/rpki-aspa-*`, all 7, QEMU netns on `claude` | Each times out at 25s with `received messages: 0` while the test peer waits for Ze's EOR (message 1); the mock RTR and peer start and Ze logs no ERROR. Red alike at 2e08f27eb5 and at 7c45216f1e (before route policy, booted on its own 7.2.9 kernel) | open: cause not traced; read Ze's debug log for whether the session reaches Established and why no EOR follows (the peers carry no export policy, so check RFC 8212 default reject's effect on EOR), fix it, then rerun the 7 |
| 2026-11-01 | - (found bumping the rustbgpd perf pin) | `le perf send --passive-listen`, `internal/perf/benchmark.go` | rustbgpd (the only passive DUT) sends its OPEN, gets none back and resets with NOTIFICATION 5/0 after 30 s, so its perf run fails the warmup. Passive mode skips `doHandshake` on the receiver and `receiveRaw` drops non-UPDATEs. Since 1427dc96cb, untested | open: fix written, unrun; close once the Mac runs `TestRunPassiveListenHandshakesReceiver` green and `./le perf run dut rustbgpd` measures rustbgpd |
| 2026-11-01 | bgp-rfc8212-ebgp-default-reject (closed; found by filter-wire-0-umbrella bisect, not caused) | `test/reload/bgp-role-export-reload.ci` | First red at 30ff1fce91 (D-2 import invalidation, operation-based reload; parent 8b062b45a8 green, both run twice): after the reload sets peer-a's export to `[ default customer ]`, B never receives U2's announce (nor a withdraw), so `await-file b-first` times out. bgp-level `final accept` is set, so not RFC 8212 | open: trace why U2 misses B since 30ff1fce91 (reconcile-peers residual or a policy-epoch fence), fix the producer, keep the test |
