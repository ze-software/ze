---
kind: directive
level:
stage:
---
- `ip` is a local literal address (`zt:ip-address`); `address` is a remote host that may be a name. The two field names encode that difference on purpose. Do not use `host`, and do not use `ip` for a remote target.
- A combined `"host:port"` / `"address:port"` string is banned (structured data, `evidence.md`). Split it into the two fields.
- `port` is never a bare `uint16` and never an inline `uint16 { range ... }`; use the typedef.
- Hand-model the pair only for a documented exception (BGP peer-local `union`-with-`auto`); see Listeners above.
