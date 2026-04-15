# Plan: Complete Event Ownership Migration

## Context

`core/events/` still owns 60+ event type constants that belong to other components.
Namespace names are bare string literals with no compile-time safety.

## Target: per-component `events/` sub-package

Each component/plugin gets an `events/` sub-package. It is a leaf: no imports
from the parent, no dependencies. Anyone can import it without pulling in the
component's implementation or creating cycles.

```
internal/component/iface/events/events.go
internal/plugins/sysctl/events/events.go
internal/component/bgp/events/events.go
internal/component/bgp/plugins/rib/events/events.go
internal/component/config/transaction/events/events.go
internal/plugins/sysrib/events/events.go
internal/plugins/fibkernel/events/events.go
internal/plugins/ntp/events/events.go
internal/component/vpp/events/events.go
```

`core/events/` keeps ONLY:
- `RegisterNamespace`, `RegisterEventType`, `IsValidEvent`, etc. (machinery)
- `DirectionReceived`, `DirectionSent`, `DirectionBoth` (framework constants)

## Consumer imports

```go
// iface can import sysctl/events without importing sysctl itself
import sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"

eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, payload)
```

No dependency direction issues. No string literals. Full compile-time safety.

## Naming convention

Constants drop the component prefix (the package provides context):

| Before (core/events) | After (component events/) |
|----------------------|---------------------------|
| `events.EventInterfaceCreated` | `ifaceevents.EventCreated` |
| `events.EventSysctlDefault` | `sysctlevents.EventDefault` |
| `events.EventVPPConnected` | `vppevents.EventConnected` |
| `events.EventUpdate` | `bgpevents.EventUpdate` |
| `events.EventConfigVerify` | `txevents.EventVerify` |

## Registration

Each component's register.go imports its own events sub-package:

```go
// plugins/sysctl/register.go
import (
    "codeberg.org/thomas-mangin/ze/internal/core/events"
    sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"
)

func init() {
    _ = events.RegisterNamespace(sysctlevents.Namespace,
        sysctlevents.EventDefault, sysctlevents.EventSet, ...)
}
```

## Steps

1. Delete the events.go files I just created in the parent packages (wrong location)
2. Create 9 `events/events.go` sub-packages (constants + Namespace)
3. Update each component's register.go to import its own events sub-package
4. Update all consumers to import from owning component's events sub-package
5. Delete domain files from core/events/ (bgp.go, interface.go, sysctl.go, etc.)
6. Update events_test.go TestMain to import from sub-packages
7. Use `scripts/dev/replace.py` for bulk replacements
8. `make ze-verify`

## Verification

```
go vet ./internal/...
make ze-verify
```
