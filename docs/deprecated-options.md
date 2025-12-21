# Deprecated Configuration Options

This document lists configuration syntax that has been deprecated and the migration path for each.

## v2 Syntax (Deprecated)

The following v2 syntax is deprecated and should be migrated to v3:

### neighbor keyword

**Deprecated:**
```
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
```

**Current (v3):**
```
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
```

**Migration:** `zebgp config migrate --in-place config.conf`

---

### Root-level peer globs

**Deprecated:**
```
peer * {
    hold-time 90;
}

peer 192.168.*.* {
    hold-time 180;
}
```

**Current (v3):**
```
template {
    match * {
        hold-time 90;
    }
    match 192.168.*.* {
        hold-time 180;
    }
}
```

**Why:** Glob patterns now live in `template { match }` blocks, keeping peer definitions for actual peers only.

---

### template { neighbor }

**Deprecated:**
```
template {
    neighbor ibgp-rr {
        peer-as 65000;
    }
}
```

**Current (v3):**
```
template {
    group ibgp-rr {
        peer-as 65000;
    }
}
```

**Why:** `neighbor` renamed to `group` for clarity - these are named template groups, not neighbors.

---

## Unsupported Features

These ExaBGP features are parsed but ignored:

### multi-session capability

```
capability {
    multi-session;  # Ignored - ExaBGP-specific
}
```

ZeBGP uses standard BGP session handling.

### operational capability

```
capability {
    operational;  # Ignored - ExaBGP-specific
}
```

### operational block

```
peer 192.0.2.1 {
    operational {
        # All operational messages ignored
    }
}
```

ExaBGP operational messages (ASM, ADM, RPCQ, etc.) are not supported.

---

## Migration Commands

```bash
# Check what needs migration
zebgp config check config.conf

# Preview changes
zebgp config migrate --dry-run config.conf

# Apply migration
zebgp config migrate --in-place config.conf
```

See [config-migration.md](config-migration.md) for full details.
