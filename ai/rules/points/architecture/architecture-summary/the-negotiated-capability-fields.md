---
kind: table
level:
stage:
---
| Field | Type | Effect |
|-------|------|--------|
| ASN4 | bool | 4-byte ASN in AS_PATH |
| AddPath | map[Family]Mode | Path-ID prefix in NLRI |
| ExtendedMsg | bool | 65535 byte messages |
| ExtendedNextHop | map[Family]AFI | Per-family NH mapping |
| GracefulRestart | *GR | RFC 4724 state |
| RouteRefresh | bool | RFC 2918 |
