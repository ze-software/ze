# BGP Implementation Competitive Analysis for ZeBGP

**Generated:** 2025-12-22
**Purpose:** Identify USPs, architectural innovations, and features ZeBGP could adopt from 12 open-source BGP implementations.

---

## Executive Summary

This analysis examined 12 BGP implementations across 4 categories to identify competitive advantages and adoption opportunities for ZeBGP.

### Key Findings

| Category | Top Performer | Key Learning for ZeBGP |
|----------|--------------|----------------------|
| **Performance** | BIRD | Attribute deduplication, trie-based RIB, single-table mode |
| **API Design** | GoBGP | gRPC as option, watcher pattern, OpenConfig compatibility |
| **Security** | OpenBGPD | RFC 9687 Send Hold Timer, RPKI integration, privilege separation |
| **Minimalism** | CoreBGP | Plugin architecture, functional options pattern |
| **Cloud-Native** | MetalLB | CRD-based config, BFD integration, Kubernetes patterns |
| **Scalability** | RustyBGP | Prefix-based table sharding, multi-threaded architecture |

### Priority Recommendations for ZeBGP

| Priority | Feature | Source | Impact |
|----------|---------|--------|--------|
| 🔴 P1 | Attribute deduplication | BIRD | 3x memory reduction |
| 🔴 P1 | Trie-based RIB | BIRD/bio-rd | Faster prefix lookup |
| 🔴 P1 | Graceful Restart state machine | FRR/OpenBGPD | Production requirement |
| 🔴 P1 | RPKI/ROA integration | All | Security requirement |
| 🟡 P2 | RFC 9687 Send Hold Timer | OpenBGPD | Prevents blackholing |
| 🟡 P2 | Built-in healthcheck | ExaBGP | #1 use case |
| 🟡 P2 | Optional gRPC API | GoBGP | Programmatic access |
| 🟡 P2 | Table sharding | RustyBGP | Multi-core performance |
| 🟢 P3 | BFD integration | MetalLB/FRR | Sub-second failover |
| 🟢 P3 | CRD-based config | MetalLB | Kubernetes-native |

---

## 1. Full Routing Daemons

### 1.1 BIRD - Performance Champion

**Market Position:** De facto standard for IXP route servers (DE-CIX, AMS-IX, LINX)

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **Attribute Deduplication** | Hash-based rta/ea storage with reference counting | **MUST ADOPT** - 3x memory savings |
| **Trie-based RIB** | Same structure as kernel FIB, optimal for LPM | **MUST ADOPT** |
| **Single-table Mode** | Shared RIB across peers - 10x faster | **SHOULD ADOPT** - config option |
| **Filter DSL** | Bytecode-compiled, no loops, functions | Consider similar approach |
| **Multithreading (v3.0)** | 6-8x speedup on multi-core | Already have reactor pattern |

**Performance Metrics:**
- Memory: 30MB for full BGP table (vs 87-167MB for Quagga)
- Convergence: ~25% faster than FRRouting
- Optimal for: <500 neighbors with many prefixes

**Key Files to Study:**
- Filter language: https://bird.network.cz/doc/bird-5.html
- Attribute system: https://en.blog.nic.cz/2021/03/23/bird-journey-to-threads-chapter-1-the-route-and-its-attributes/

---

### 1.2 FRRouting - Feature Complete

**Market Position:** Most popular open-source routing suite (NVIDIA/Cumulus, SONiC)

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **Complete Protocol Suite** | BGP + OSPF + IS-IS + LDP + BFD | N/A - ZeBGP is BGP-only |
| **YANG/gRPC Northbound** | Modern management interface | Consider gRPC option |
| **Topotests Framework** | Comprehensive topology testing | Adopt similar pattern |
| **Route Maps** | Prefix-tree optimized policy | Already have filter system |
| **VRF Support** | Multi-tenant routing | Future consideration |

**Key Features to Adopt:**
1. **Graceful Restart** - RFC 4724 + RFC 8538 state machine
2. **RPKI/ROV** - RTR client and validation policy
3. **ADD-PATH** - RFC 7911 for route reflectors
4. **Extended Messages** - RFC 8654 for large updates
5. **watchfrr Pattern** - Process health monitoring

**Performance:**
- Similar to BIRD in most scenarios
- Higher memory usage (~2x BIRD)
- Struggles at >150 neighbors

---

### 1.3 OpenBGPD - Security Focus

**Market Position:** BSD-licensed, backed by IXP operators (DE-CIX, AMS-IX, LINX, Netnod)

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **RFC 9687 Send Hold Timer** | First implementation, prevents blackholing | **SHOULD ADOPT** - novel security |
| **Three-Process Model** | Parent (root) + SE + RDE with chroot | Inform process separation |
| **RPKI Integration** | Tight coupling with rpki-client | Support rpki-client output format |
| **Clean Config Syntax** | Macros, prefix-sets, condensed syntax | Inform config DSL |
| **ASPA Support** | draft-ietf-sidrops-aspa-* | Future consideration |

**Security Features:**
- chroot() jail for SE and RDE
- Privilege dropping for non-parent processes
- pledge() and unveil() on OpenBSD

**Recommendation:** Implement RFC 9687 Send Hold Timer - unique differentiator

---

## 2. Programmatic Libraries

### 2.1 GoBGP - API-First Go Implementation

**Market Position:** Most popular Go BGP implementation (DigitalOcean, Kubernetes CNI)

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **gRPC API Primary** | CLI wraps API - ensures completeness | Optional gRPC alongside text API |
| **OpenConfig Schema** | Configuration follows IETF YANG models | Consider compatibility |
| **Watcher Pattern** | Event streaming for route changes | Formalize observer API |
| **Dynamic Neighbors** | Accept from prefix ranges | Useful for cloud deployments |
| **Embeddable** | Use as Go library | Already achieved |

**Performance Warning:**
- 6-40x slower than BIRD/FRR
- Uses all CPU cores inefficiently
- Memory bloat with large tables

**What NOT to Copy:**
- Excessive goroutines
- Unbounded channels (memory pressure)
- Central event loop bottleneck

---

### 2.2 bio-routing - Memory Efficient

**Market Position:** Production at EXARING (AS51324), created to solve GoBGP memory issues

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **Dual-Mode** | Library AND daemon | Already achieved |
| **VRF First-Class** | Design from ground up | Consider early |
| **State Interface FSM** | `run() -> (state, string)` | Clean pattern to adopt |
| **Path Hidden Reasons** | Explicit filtering reasons | Aids debugging |
| **Filter Chain Architecture** | Modular composition | Inform policy design |

**Key Patterns:**
```go
// State interface pattern
type state interface {
    run() (state, string)
}

// Hidden reasons
const (
    HiddenReasonNone = iota
    HiddenReasonFiltered
    HiddenReasonASLoop
    // ...
)
```

---

### 2.3 CoreBGP - Minimalist Reference

**Market Position:** Building block, not complete solution

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **Does Nothing Extra** | No RIB, no UPDATE generation | N/A - ZeBGP needs RIB |
| **Plugin Interface** | Clean lifecycle hooks | Inform peer handler design |
| **Generic UpdateDecoder** | Type-safe composable decoders | Consider for parsing |
| **Functional Options** | `WithHoldTime()`, `WithPassive()` | Adopt for peer config |
| **RFC 7606 Error Types** | Behavior encoded in types | Adopt error handling pattern |

**Code Pattern to Adopt:**
```go
// Functional options
func NewPeer(addr, localAS, peerAS,
    WithHoldTime(90),
    WithPassive(),
    WithMD5Password("secret"),
)

// Error types with behavior
type UpdateError interface {
    error
    AsSessionReset() *Notification
}
```

---

### 2.4 RustyBGP - Sharded Architecture

**Market Position:** Experimental, proves multicore can help with right architecture

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **Table Sharding** | Prefix-hash to dedicated workers | **SHOULD ADOPT** |
| **Peer/Table Separation** | 50% threads each | Inform goroutine allocation |
| **Channel-Based Routing** | Async between peer and table | Enhance update distribution |
| **GoBGP API Compatible** | Same gRPC, CLI works | N/A |
| **Formal Methods** | Panic-free deserialization proofs | Use fuzzing instead |

**Sharding Pattern for ZeBGP:**
```go
const numShards = 16
type ShardedRIB struct {
    shards [numShards]struct {
        routes map[string]*Route
        mu     sync.RWMutex
    }
}

func (r *ShardedRIB) shard(prefix []byte) int {
    return int(hash(prefix) % numShards)
}
```

**Performance:**
- 4.4x less memory than GoBGP
- 5.5x less CPU than GoBGP
- 3x faster convergence

---

### 2.5 ExaBGP - Parent Project

**Market Position:** "BGP Swiss Army Knife" - NOT a routing daemon

| USP | Details | ZeBGP MUST Preserve |
|-----|---------|---------------------|
| **STDIN/STDOUT API** | Any language integration | ✅ Core requirement |
| **JSON Encoder** | Structured data | ✅ Default encoder |
| **Text Encoder** | Legacy compatibility | ✅ Backward compat |
| **Healthcheck Module** | #1 use case | ✅ Built-in module |
| **FlowSpec DSL** | Human-readable rules | ✅ Match config syntax |
| **55+ RFC Support** | Comprehensive protocol | Continue expanding |

**Pain Points ZeBGP MUST Solve:**
| Issue | ExaBGP | ZeBGP Target |
|-------|--------|--------------|
| Performance | Python overhead | Go 10-100x faster |
| Neighbor limit | ~29 before crash | 1000s of peers |
| Memory | Leaks with many peers | Proper management |
| Convergence | 5-15 seconds | <1 second |
| Multi-core | Python GIL | Native goroutines |
| Session limit | 1024 (select) | Unlimited (epoll) |

**Notable Users:** AMS-IX, BBC, Cloudflare, Facebook/Meta, Microsoft, Cisco

---

## 3. Kubernetes/Container

### 3.1 MetalLB - Kubernetes Load Balancer

**Market Position:** De facto bare metal K8s load balancer

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **CRD-Based Config** | IPAddressPool, BGPPeer, BFDProfile | Future K8s operator |
| **Speaker/Controller** | Separation of concerns | Inform daemon design |
| **BFD Profiles** | Reusable BFD configuration | Add BFD support |
| **Selector-Based** | Node/peer label filtering | Useful pattern |
| **FRR Backend** | Advanced features via FRR | ZeBGP could be alternative |

**CRD Design Pattern:**
```yaml
apiVersion: metallb.io/v1beta1
kind: BGPPeer
spec:
  myASN: 64500
  peerASN: 64501
  peerAddress: 10.0.0.1
  bfdProfile: fast-detect
  nodeSelectors:
    - matchLabels:
        rack: rack-1
```

**ZeBGP Opportunity:** Native BFD + IPv6 without FRR sidecar

---

### 3.2 Kube-Router - Embedded GoBGP

**Market Position:** All-in-one K8s networking (kube-proxy + CNI + network policy)

| USP | Details | ZeBGP Adoption |
|-----|---------|----------------|
| **Annotation-Based Config** | No separate CRDs needed | Simpler approach |
| **GoBGP Embedding** | Library pattern | Reference implementation |
| **Auto Node Mesh** | iBGP between all nodes | Default cluster mode |
| **Route Reflector** | Via node annotations | Add RR support |
| **Service IP Advertisement** | ClusterIP, ExternalIP, LB | K8s integration pattern |

**GoBGP Integration Pattern:**
```go
// 1. Create server
bgpServer = gobgp.NewBgpServer()
go bgpServer.Serve()

// 2. Start with config
bgpServer.StartBgp(ctx, &StartBgpRequest{...})

// 3. Watch events
bgpServer.WatchEvent(ctx, req, func(event) {
    // Handle state changes
})

// 4. Advertise routes
bgpServer.AddPath(ctx, &AddPathRequest{...})
```

---

## 4. Performance Benchmarks Summary

### Convergence Speed (10 peers, 100K routes each)

| Implementation | Time | Relative |
|---------------|------|----------|
| **BIRD** | 3-4s | Baseline |
| **FRRouting** | 3-4s | ~Same |
| **RustyBGP** | ~10s | 2-3x slower |
| **GoBGP** | 24s | 6-8x slower |
| **OpenBGPD** | Minutes | Very slow |

### Memory Usage (100K routes)

| Implementation | Memory | Notes |
|---------------|--------|-------|
| **BIRD** | 15-110 MB | Most efficient |
| **RustyBGP** | ~32 MB | Very efficient |
| **FRRouting** | 90-250 MB | Moderate |
| **GoBGP** | 140-280 MB | 2x more |
| **ExaBGP** | ~2.1 GB | ~21 KB/route |

### CPU Usage Pattern

| Implementation | Pattern | Notes |
|---------------|---------|-------|
| **BIRD** | Single-threaded | 1 core max |
| **FRRouting** | Single-threaded | 1 core max |
| **RustyBGP** | Multi-threaded | Efficient multi-core |
| **GoBGP** | Multi-threaded | All cores, inefficient |

---

## 5. Feature Support Matrix

| Feature | BIRD | FRR | GoBGP | OpenBGPD | ExaBGP | ZeBGP |
|---------|------|-----|-------|----------|--------|-------|
| Large Communities | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| FlowSpec | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| EVPN | ⚠️ | ✅ | ✅ | ❌ | ❌ | ✅ |
| BGP-LS | ❌ | ⚠️ | ✅ | ❌ | ✅ | ✅ |
| RPKI/ROA | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Add-Path | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| Graceful Restart | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| BFD | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| gRPC API | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |

Legend: ✅ Full | ⚠️ Partial | ❌ None

---

## 6. Recommended Roadmap for ZeBGP

### Phase 1: Core Performance (P1)

1. **Attribute Deduplication** (BIRD pattern)
   - Hash-based storage for path attributes
   - Reference counting for sharing
   - Target: 3x memory reduction

2. **Trie-based RIB** (BIRD/bio-rd pattern)
   - Replace map-based lookup
   - Optimal for longest-prefix match
   - Consider: `github.com/kentik/patricia`

3. **Graceful Restart State Machine** (FRR/OpenBGPD)
   - Stale route marking/deletion
   - Restart timer management
   - End-of-RIB synchronization

4. **RPKI/ROA Integration**
   - RTR protocol client (RFC 6810)
   - Validation state tracking
   - Policy hooks for valid/invalid/unknown

### Phase 2: Security & Stability (P2)

5. **RFC 9687 Send Hold Timer** (OpenBGPD)
   - Detect blocked outbound connections
   - Prevent blackholing at IXPs
   - Unique differentiator

6. **Built-in Healthcheck** (ExaBGP)
   - Zero-config health monitoring
   - `--cmd`, `--interval`, `--rise`, `--fall`
   - Most-used ExaBGP feature

7. **Table Sharding** (RustyBGP pattern)
   - Prefix-hash to worker goroutines
   - Per-shard RIB without cross-locking
   - Target: Linear scaling with cores

### Phase 3: API & Integration (P3)

8. **Optional gRPC API** (GoBGP pattern)
   - Alongside text API, not replacing
   - WatchEvent for streaming
   - Multi-language client support

9. **BFD Integration** (MetalLB/FRR)
   - Sub-second failure detection
   - BFDProfile abstraction
   - External daemon or library

10. **Kubernetes CRD Operator** (MetalLB pattern)
    - BGPPeer, RouteAdvertisement CRDs
    - Selector-based targeting
    - Service IP advertisement

---

## 7. Competitive Positioning

### ZeBGP Advantages

| Advantage | vs GoBGP | vs ExaBGP | vs BIRD |
|-----------|----------|-----------|---------|
| ExaBGP API compat | ✅ Unique | N/A | ✅ Unique |
| Single binary | ✅ Same | ✅ Better | ✅ Better |
| Modern Go codebase | ✅ Same | ✅ Better | ✅ Better |
| Container-native | ✅ Same | ✅ Better | ✅ Better |
| Performance | 🎯 Target | ✅ Better | 🎯 Target |
| Memory efficiency | 🎯 Target | ✅ Better | 🎯 Target |

### ZeBGP Target Market

1. **ExaBGP Users** - Drop-in replacement with better performance
2. **SDN/Automation** - Programmatic BGP control
3. **Kubernetes** - Cloud-native BGP load balancing
4. **DDoS Mitigation** - FlowSpec injection
5. **Health-based Routing** - Anycast with service health

---

## Sources

### Primary Benchmark Sources
- [Comparing Open Source BGP Stacks](https://elegantnetwork.github.io/posts/comparing-open-source-bgp-stacks/)
- [BGP Stacks with Internet Routes](https://elegantnetwork.github.io/posts/comparing-open-source-bgp-internet-routes/)
- [bgperf2](https://github.com/netenglabs/bgperf2)

### Implementation Documentation
- [BIRD Documentation](https://bird.network.cz/)
- [FRRouting BGP](https://docs.frrouting.org/en/latest/bgp.html)
- [GoBGP GitHub](https://github.com/osrg/gobgp)
- [OpenBGPD](https://www.openbgpd.org/)
- [ExaBGP Wiki](https://github.com/Exa-Networks/exabgp/wiki)
- [bio-routing](https://github.com/bio-routing/bio-rd)
- [CoreBGP](https://github.com/jwhited/corebgp)
- [RustyBGP](https://github.com/osrg/rustybgp)
- [MetalLB](https://metallb.universe.tf/)
- [Kube-Router](https://www.kube-router.io/)

### Industry Analysis
- [2025 Guide to Open-Source Routing Daemons](https://bizety.com/2025/10/09/2025-guide-to-open-source-routing-daemons-frr-bird-and-exabgp/)
- [RPKI ROV Deployment Milestone](https://manrs.org/2024/05/rpki-rov-deployment-reaches-major-milestone/)
