# Dynamic Watchdog API Plan

**Status:** ✅ DONE (2025-12-27)

## Implementation Summary

### Completed Features

Config-based watchdogs:
```
route 77.77.77.77 next-hop 1.2.3.4 watchdog dnsr withdraw;
```

API group control:
```
announce watchdog <name>
withdraw watchdog <name>
```

WatchdogManager with pool-based route control:
- Pool-based route grouping
- Unified state management
- Test coverage added

### Test Status

- ✅ Test `ao` (watchdog) passes
- ✅ WatchdogManager unit tests pass

---

## Future Enhancements (Deferred)

If dynamic watchdog route creation via API is needed later:
- See `plan/watchdog-pool.md` for pool-based design
- Consider group-first namespace syntax
- AFI/SAFI handling needs decision

---

**Completed:** 2025-12-27
