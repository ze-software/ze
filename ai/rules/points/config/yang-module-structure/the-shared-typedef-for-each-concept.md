---
kind: table
level:
stage:
---
| Concept | Use | Do NOT use |
|---------|-----|------------|
| IPv4 / IPv6 / either address | `zt:ipv4-address` / `zt:ipv6-address` / `zt:ip-address` | raw `type string`; `type string; ze:validate "ipv4-address"` |
| IPv4 / IPv6 / either prefix | `zt:prefix-ipv4` / `zt:prefix-ipv6` / `zt:ip-prefix` | `type string; ze:validate "ipv4-prefix\|ipv6-prefix"` |
| ASN, port | `zt:asn` / `zt:asn2`, `zt:port` / `zt:listener-port` | inline `uint32`/`uint16` with a copied range |
| Community / RD / address-family | `zt:community`, `zt:route-distinguisher`, `zt:address-family` | per-module patterns for the same shape |
| MAC address | `zt:mac-address` (add it to `ze-types` if absent) | per-plugin `ze:validate "mac-address"` |
| Duration / dimensioned value | an unsigned integer leaf with a YANG `units` statement (see Units below) | `type string` for a duration; the unit only implied in the description |
