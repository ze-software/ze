# Spec: nexthop-capability-move

## Task
Move the `nexthop` configuration block from peer level into the `capability` block.

## Rationale

The nexthop block configures RFC 8950 Extended Next Hop capability families. Logically, this belongs inside the capability block since:
1. It directly controls a BGP capability (Extended Next Hop, Code 5)
2. The capability is built from these family entries
3. Grouping with other capabilities improves config clarity

## Before/After

```
# BEFORE: nexthop at peer level
peer 10.0.0.1 {
    capability {
        nexthop enable;      # flag only
    }
    nexthop {                # separate block
        ipv4/unicast ipv6;
    }
}

# AFTER: nexthop inside capability
peer 10.0.0.1 {
    capability {
        nexthop {            # full block with families
            ipv4/unicast ipv6;
        }
    }
}
```

## Files Modified

### Schema & Parsing
| File | Change |
|------|--------|
| `internal/config/bgp.go:207` | Changed `Field("nexthop", Flex())` → `Field("nexthop", Freeform())` in capability |
| `internal/config/bgp.go:223` | Removed `Field("nexthop", Freeform())` from peer level |
| `internal/config/bgp.go:1023-1027` | Moved nexthop parsing into capability block in `applyTreeSettings` |
| `internal/config/bgp.go:1287-1291` | Moved nexthop parsing into capability block in `parsePeerConfig` |

### Migration
| File | Change |
|------|--------|
| `internal/exabgp/migrate.go:226-232` | `migrateCapability`: Now moves nexthop block into capability (was setting flag) |
| `internal/exabgp/migrate.go:252-258` | `copyContainers`: Removed nexthop copying (now in capability) |

### Tests
| File | Change |
|------|--------|
| `internal/exabgp/migrate_test.go` | Updated 6 test functions to check for nexthop inside capability |

### Config Files
| File | Change |
|------|--------|
| `etc/ze/bgp/api-open.conf` | Moved nexthop into capability |
| `etc/ze/bgp/conf-prefix-sid-srv6.conf` | Moved nexthop into capability |
| `etc/ze/bgp/extended-nexthop.conf` | Moved nexthop into capability |
| `test/data/encode/extended-nexthop.conf` | Moved nexthop into capability |
| `test/data/encode/prefix-sid-srv6.conf` | Moved nexthop into capability |
| `test/data/migrate/nexthop/expected.conf` | Updated expected migration output |
| `test/data/plugin/open.conf` | Moved nexthop into capability |

## Implementation Details

### Config Schema
The nexthop field type changed from `Flex()` (flag/value) to `Freeform()` (block with arbitrary entries) inside the capability container. This allows parsing family entries like `ipv4/unicast ipv6;`.

### Parsing Logic
Both `applyTreeSettings` and `parsePeerConfig` were updated to get the nexthop block from `cap.GetContainer("nexthop")` instead of `tree.GetContainer("nexthop")`, where `cap` is the capability container.

### Migration
ExaBGP configs have nexthop at neighbor level. Migration now:
1. Reads nexthop block from source neighbor
2. Converts family syntax (`ipv4 unicast ipv6` → `ipv4/unicast ipv6`)
3. Places converted block inside capability container

### Serialization
No changes needed - recursive serialization naturally handles nexthop inside capability.

## Checklist

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Config files updated to new format
- [x] Migration test expectations updated
- [x] Test assertions updated

## Implementation Summary

### What Was Implemented
- Schema change: nexthop moved from peer level to capability block
- Parsing change: nexthop now parsed from capability container
- Migration change: ExaBGP nexthop blocks placed inside capability
- All config files and tests updated

### Design Insights
- Freeform parsing handles the family syntax naturally
- Recursive serialization requires no changes
- Migration logic cleanly separates ExaBGP reading (peer level) from ZeBGP output (capability level)
