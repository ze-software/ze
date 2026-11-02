# A question about a config is answered by matching its text

Tooling or a test reads a config file, Ze's or a peer daemon's, with a text
match. The match answers a question about spelling: it misses the same setting
written another way, and it fires on a comment or on another section.

Captured: 2026-11-02 rule config/directives/answer-a-question-about-a-config-from-its-structure

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | - | interop lab network selection, `needsIPv6` (`internal/le/interoplab`) | It matched the text `address-family ipv6`, so a ze.conf written as `address-family { ipv6 { } }` (ospfv3-gr-frr and three others) ran on an IPv4-only network; FRR never saw a neighbor | fixed: bc9f7c9bea follows block paths (`zeRunsOSPFv3`); `TestOSPFv3ConfigSelectsIPv6Network` |
| 2026-11-02 | - | strongSwan lab flush check (`test/interop-ipsec/parity_test.go`) | The first form of the check accepted `flush_line` anywhere in the drop-in, so a setting on another logger, at charon level or in a comment passed for the stderr logger the checkers read | fixed: 0953167f23 reads the section path (`labFlushesEachLine`); `TestLabFlushCheckRequiresTheStderrLogger` |
