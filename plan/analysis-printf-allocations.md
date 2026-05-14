# Analysis: Printf-Family Allocation Reduction

| Field | Value |
|-------|-------|
| Date | 2026-05-14 |
| Scope | All `fmt.Sprintf`, `fmt.Fprintf`, `fmt.Errorf`, `fmt.Printf/Println` in production + test code |

## Totals

| Function | Production | Test | Total |
|----------|-----------|------|-------|
| `fmt.Sprintf` | 1,302 | ~150 | ~1,452 |
| `fmt.Fprintf` | 1,586 | ~40 | ~1,626 |
| `fmt.Errorf` | 4,407 | ~80 | ~4,487 |
| `fmt.Printf/Println` | 178 | ~16 | ~194 |
| **Total** | **~7,473** | **~286** | **~7,759** |

## Existing Zero-Alloc Pattern (reference)

The codebase already has a proven pattern in `attribute/text_append.go`:
- `AppendText(buf []byte) []byte` methods on every attribute type
- Primitives: `strconv.AppendUint`, `netip.Addr.AppendTo`, `hex.AppendEncode`, literal `append(buf, "..."...)`
- No `fmt.Sprintf`, no `strings.Builder`, no `strconv.FormatUint` in that file
- Benchmarked in `text_append_bench_test.go`
- Also used in `format/text.go` (hex digit table), `core/family/family.go` (`AppendTo`)
- `reactor/filter_format.go` has explicit "No fmt.Sprintf" contract

The strategy is to extend this pattern systematically.

---

## P0: Hot Path (per-UPDATE / per-route)

These fire on every BGP UPDATE or route evaluation. Full table = 900K+ routes.

### bestpath.go: ComparePair discards allocated reason strings

**Single highest-impact item.** `ComparePair` (line 274) calls `comparePairWithReason` which allocates ~10 `fmt.Sprintf` reason strings per comparison, then immediately discards them:

```go
func ComparePair(a, b *Candidate) int {
    result, _, _ := comparePairWithReason(a, b)  // reason string thrown away
    return result
}
```

With 900K routes and 2 candidates each, that is ~9M wasted string allocations at startup.

**Fix:** Split into two functions:
- `comparePair(a, b) (int, BestStep)` with no string allocation (for the hot path)
- `comparePairWithReason(a, b) (int, BestStep, string)` only called from `SelectBestExplain`

| Line | Sprintf | Why it allocates |
|------|---------|-----------------|
| 290 | `"stale-level %d vs %d (threshold %d)"` | 3 ints |
| 298 | `"stale-level %d vs %d"` | 2 ints |
| 308 | `"local-preference %d vs %d"` | 2 ints |
| 318 | `"as-path-length %d vs %d"` | 2 ints |
| 328 | `"origin %d vs %d"` | 2 ints |
| 340 | `"med %d vs %d (same neighbor AS %d)"` | 3 ints |
| 354 | `"ebgp-over-ibgp (%q vs %q)"` | 2 strings |
| 368 | `"router-id %s vs %s"` | 2 strings |
| 376 | `"peer-address %s vs %s"` | 2 strings |

### rib_nlri.go: NLRI formatting per prefix

Called for every NLRI in every UPDATE.

| Line | Pattern | Fix |
|------|---------|-----|
| 153 | `Sprintf("%s/%d", ip.String(), prefixLen)` | `ip.AppendTo(buf)` + `'/'` + `strconv.AppendInt` |
| 166 | `Sprintf("hex:%x", nlriBytes)` | `append(buf, "hex:"...)` + `hex.AppendEncode` |
| 169 | `Sprintf("%s [pathID=%d]", prefix, pathID)` | concat + `strconv.Itoa` |
| 188 | `Sprintf("afi-%d", fam.AFI)` | `"afi-"` + `strconv.AppendUint` |
| 207 | `Sprintf("safi-%d", fam.SAFI)` | `"safi-"` + `strconv.AppendUint` |
| 218 | `Sprintf("%d.%d.%d.%d", ...)` IPv4 | `appendDottedDecimal` helper (exists as `appendClusterID` in text_append.go:80) |
| 221 | `Sprintf("%x:%x:...%x", ...)` IPv6 | `hex.AppendEncode` per segment |
| 231 | `Sprintf("%x", data)` fallback | `hex.AppendEncode` |
| 236 | `Sprintf("%d.%d.%d.%d", ...)` routerID | same as IPv4 |

### Route key formatting (per route insert/lookup)

| File:Line | Pattern | Fix |
|-----------|---------|-----|
| `event.go:44` | `Sprintf("%s:%d", prefix, pathID)` | `append` + `strconv.AppendUint` |
| `route.go:73` | `Sprintf("%s:%s:%d", family, prefix, pathID)` | same |
| `peersettings.go:154` | `Sprintf("%s#%d", key, pathID)` | `key + "#" + strconv.Itoa(pathID)` |

### Cache commands (per UPDATE flowing through system)

| File | Count | Pattern | Fix |
|------|-------|---------|-----|
| `persist/server.go` | ~8 | `Sprintf("cache %d forward %s", msgID, peer)` | `append`-chain + `strconv.AppendUint` |
| `rr/rr.go` | ~3 | `Sprintf("cache %d forward *", msgID)` | same |
| `rs/server.go` | ~3 | `Sprintf("adj-rib-in replay %s %d", ...)` | same |

With full table feed of 900K routes, that is ~12.6M allocations at startup.

### Fingerprint functions (per route grouping)

| File:Line | Args | Fix |
|-----------|------|-----|
| `peer_static_routes.go:284` | 16-arg Sprintf with `\|` separator | Builder + `strconv.AppendInt` |
| `peer_initial_sync.go:514` | 7-arg MVPN fingerprint | Builder + `strconv.AppendInt` |

### nlri/rd.go (per VPN/EVPN route)

| Line | Pattern | Fix |
|------|---------|-----|
| 121 | `Sprintf("0:%d:%d", asn, assigned)` | `"0:" + strconv.AppendUint + ":" + strconv.AppendUint` |
| 126 | `Sprintf("1:%s:%d", ip, assigned)` | `"1:" + ip.AppendTo + ":" + strconv.AppendUint` |
| 131 | `Sprintf("2:%d:%d", asn, assigned)` | same as type 0 |
| 133 | `Sprintf("rd-type%d:%x", ...)` | `"rd-type" + strconv.AppendUint + ":" + hex.AppendEncode` |

### Community formatting (per route, multiplied by community count)

| File:Line | Pattern | Fix |
|-----------|---------|-----|
| `attribute/community.go:98` | `Sprintf("%d:%d", c>>16, c&0xFFFF)` in `Community.String()` | Use existing `appendCommunityText` from `text_append.go:60` |
| `attribute/community.go:264` | `Sprintf("%d:%d:%d", ...)` in `LargeCommunity.String()` | Use existing `LargeCommunity.AppendText` from `text_append.go:43` |
| `rib_attr_format.go:171` | `Sprintf("%d:%d", high, low)` in loop | `strconv.AppendUint` x2 |

---

## P1: Per-Route Display (show/looking-glass/format)

Fires when displaying routes. Thousands of calls on `show routes` for a full table.

### NLRI type String() methods (~40 calls across NLRI plugins)

| Package | Count | Patterns | Fix |
|---------|-------|----------|-----|
| `nlri/evpn` | ~9 | MAC `%02x:` x6/10, `evpn-type%d`, labels `%d` | hex digit table + Builder |
| `nlri/rtc` | 6 | `%d:%d`, `%d.%d.%d.%d:%d`, `rt-type%d:%x` | strconv + Builder |
| `nlri/ls` | ~10 | `%d.%d.%d.%d` IPv4, `%02x` x6/7 | `appendDottedDecimal` / hex table |
| `nlri/flowspec` | 10 | `>%d`, `<%d`, `=%d` etc in loop | strconv with prefix char |
| `nlri/vpn` | 3 | `%X`, `%d`, `path-id=%d` | hex table, strconv |
| `nlri/mup` | 2 | `type(%d)`, `%s rd %s` | strconv, concat |
| `nlri/vpls` | 1 | `rd %s ve-id %d label %d` | Builder |

**Fix pattern:** Add `AppendTo(buf []byte) []byte` method on each type, thin `String()` wrapper using stack-allocated `[64]byte`:

```go
func (t Type) String() string {
    var buf [64]byte
    return string(t.AppendTo(buf[:0]))
}
```

### format.go: Announce/Withdraw command building

| Line | Pattern | Fix |
|------|---------|-----|
| 55 | `Fprintf(&sb, " med %d", *route.MED)` | `sb.WriteString(" med ")` + write `strconv.AppendInt` result |
| 60 | `Fprintf(&sb, " local-preference %d", ...)` | same |
| 133 | `Fprintf(sb, " label %d", ...)` | same |
| 136 | `Fprintf(sb, " path-information %d", ...)` | same |

### message/rfc7606.go (per UPDATE validation, 23 calls)

All follow `Sprintf("RFC 7606 Section X.Y: description %d", val)`.
Fix: `"RFC 7606 Section X.Y: description " + strconv.Itoa(val)`.

### message/open.go (per session)

| Line | Pattern | Fix |
|------|---------|-----|
| 224 | `Sprintf("%d.%d.%d.%d", ...)` router-ID | `appendDottedDecimal` |
| 255 | `Sprintf("OPEN AS%d RouterID=%s HoldTime=%d", ...)` | Builder |

---

## P2: Mechanical Replacements (broad, easy wins)

These are safe, pattern-based replacements applicable everywhere.

### Strategy A: `Sprintf("%d", n)` to `strconv.Itoa` (~132 calls)

~3x faster per call. Trivial sed-like replacement. Concentrated in:
- `web/page_system.go` (12)
- `config/yang/validator.go` (6)
- `lg/` pages (scattered)
- `cmd/show/` (scattered)

### Strategy B: IPv4 `%d.%d.%d.%d` to `appendDottedDecimal` (~22 calls)

Extract existing `appendClusterID` (text_append.go:80) to a shared location. Same logic, currently private to attribute package.

### Strategy C: Single `%s` concat (~253 calls)

`Sprintf("prefix %s suffix", s)` to `"prefix " + s + " suffix"`. Go compiler optimizes 2-3 arg string concat into a single allocation (still allocates, but avoids reflect overhead of fmt).

### Strategy D: Hex formatting (~38 hex, ~7 MAC)

- `Sprintf("%02x", b)` to hex digit table lookup (already exists in `format/text.go:27`)
- `Sprintf("%02x:%02x:...", mac...)` to `appendMAC` helper
- `Sprintf("%X", data)` to `hex.EncodeToString` (cold) or `hex.AppendEncode` (hot)

### Strategy E: Colon-separated integers (~8 calls)

`Sprintf("%d:%d", a, b)` to `strconv.Itoa(a) + ":" + strconv.Itoa(b)`.

### Strategy F: `Fprintf(buf, ...)` to WriteString chains (~15 calls)

`fmt.Fprintf(b, "set %s %s\n", ...)` to `b.WriteString("set ")` + `b.WriteString(path)` + etc.

---

## P3: Sentinel Error Candidates

**722 instances** of `fmt.Errorf("constant string")` with no format verbs. Each allocates a new `*errors.errorString` per call.

### Top targets (repeated constants)

| Location | String | Copies |
|----------|--------|--------|
| `export.go` | "export requires encryption passphrase" | 6 identical |
| `session_read.go:61` / `session_coalesce.go:57` | "read buffer exhausted" | 3 (ironic: allocates under memory pressure) |
| Various hub/reactor | "server not ready" | 3 |
| `received_update.go:118` | "EBGP wire buffer exhausted" | 1 (error path but same irony) |

### Pre-allocatable wraps (~30 in wireu/)

Patterns like `fmt.Errorf("rewrite AS_PATH: %w", ErrUpdateTruncated)` appear multiple times with the same context. Since the wrapped error is a package-level sentinel, the wrapping is immutable and can be pre-allocated:

```go
var errRewriteASPathTruncated = fmt.Errorf("rewrite AS_PATH: %w", ErrUpdateTruncated)
```

---

## P4: Test Files

286 printf-family calls. Mostly cold (test setup, assertions). One benchmark concern:

| File:Line | Issue | Fix |
|-----------|-------|-----|
| `rib_bestchange_bench_test.go:39` | `Sprintf("10.0.%d.%d", i/256, i%256)` in 2000-iteration setup loop per `b.N` | Use `netip.AddrFrom4([4]byte{10, 0, byte(i/256), byte(i%256)}).String()` or direct byte construction. Skews heap measurement since benchmark is measuring heap footprint. |
| `rib_bestchange_bench_test.go:28` | `Sprintf("N=%d", n)` for sub-benchmark name | Fine (once per case) |
| `filter_format_bench_test.go:37` | Comment noting `fmt.Errorf` allocation awareness | Already addressed |

The remaining ~280 test calls are assertion messages, sub-test naming (`t.Run(fmt.Sprintf(...))`), and error construction in test helpers. Low priority.

---

## Proposed Shared Helpers

Extend the `AppendTo`/`AppendText` pattern with shared helpers. Location: either widen `attribute/text_append.go` exports or create `internal/core/textutil/`:

```go
// appendDottedDecimal already exists as appendClusterID in text_append.go:80
// Just needs to be exported or moved to a shared package
func AppendDottedDecimal(buf []byte, a, b, c, d byte) []byte

// New helpers following the same pattern
func AppendMAC(buf []byte, mac []byte) []byte        // "xx:xx:xx:xx:xx:xx"
func AppendColonPair(buf []byte, a, b uint32) []byte  // "a:b"
func AppendColonTriple(buf []byte, a, b, c uint32) []byte // "a:b:c"
func AppendHexUpper(buf []byte, data []byte) []byte   // uppercase hex (for %X)
```

---

## Implementation Priority

| Priority | Category | Call sites | Est. allocs eliminated (full table) | Effort |
|----------|----------|-----------|--------------------------------------|--------|
| **P0** | `ComparePair` split | 1 function, ~10 Sprintf | ~9M at startup | Small: split function |
| **P0** | RIB keys, NLRI format, cache commands, fingerprints | ~30 | ~15M at startup | Medium: per-file |
| **P1** | Community.String, NLRI String() methods, format.go, rfc7606 | ~80 | ~5M on full table show | Medium: add AppendTo |
| **P2** | `%d` to Itoa, single-%s concat, hex/MAC/IPv4 helpers | ~460 | Broad per-call reduction | Easy: mechanical |
| **P3** | 722 sentinel errors, pre-allocated wraps | ~750 | Per-error-path savings | Easy: mechanical |
| **P4** | Benchmark test setup | 1 | Measurement accuracy | Trivial |
