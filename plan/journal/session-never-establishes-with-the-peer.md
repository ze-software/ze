# A BGP session that never reaches Established with the peer under test

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-26 | spec-bgp-one-off-defects | `bgp-labeled-withdraw-compatibility-frr`, Ze-kernel Docker lab, daemon built from HEAD e4f37d6cae's Go files | FRR answers every connection with NOTIFICATION FSM Error (5/0) while Ze is in OPENSENT ("NotifMsg received in OPENSENT"), so no session forms and assertion 0 times out. The same lab run brought a GoBGP session up. Blocks the interop proof owed for a306bd35f2 (queued withdrawal framing) | open: cause not traced; read FRR's log for which message it found unexpected |
