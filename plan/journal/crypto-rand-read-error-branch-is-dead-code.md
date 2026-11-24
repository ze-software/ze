| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-09 | ipv6cp-accepts-and-proposes-a-zero-interface-identifier | `generateIPv6CPInterfaceID` (`internal/component/l2tp/ppp/ipv6cp.go`) | Go 1.27 `crypto/rand.Read` never returns an error and crashes on failure, so `if _, err := rand.Read(...); err != nil` is unreachable dead code | fixed: dead `crypto/rand.Read` error branch removed in `generateIPv6CPInterfaceID` (`internal/component/l2tp/ppp/ipv6cp.go`) |
