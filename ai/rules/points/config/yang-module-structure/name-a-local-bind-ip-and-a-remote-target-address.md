---
kind: directive
level: MUST
stage:
---
- `ip` is a local literal address (`zt:ip-address`); `address` is a remote host that MAY be a name. The two field names encode that difference on purpose. `host` MUST NOT be used, and `ip` MUST NOT be used for a remote target.
- A combined `"host:port"` / `"address:port"` string MUST NOT be used (structured data, `evidence.md`). It MUST be split into the two fields.
- `port` MUST NOT be a bare `uint16` and MUST NOT be an inline `uint16 { range ... }`; the typedef MUST be used.
- The pair MAY be hand-modelled only for a documented exception (BGP peer-local `union`-with-`auto`); see Listeners above.
