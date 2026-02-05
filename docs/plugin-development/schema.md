# YANG Schema Authoring

Plugins declare their configuration schema using YANG (RFC 7950). This guide covers the basics for plugin authors.

## Minimal Schema

```yang
module my-plugin {
    namespace "urn:my-company:my-plugin";
    prefix mp;

    container my-plugin {
        leaf enabled {
            type boolean;
            default true;
        }
    }
}
```

## Module Structure

```yang
module name {
    namespace "urn:...";  // Unique namespace URI
    prefix prefix;        // Short prefix for references

    // Content: containers, lists, leafs
}
```

## Common Types

| YANG Type | Go Type | Example |
|-----------|---------|---------|
| `string` | `string` | `"hello"` |
| `boolean` | `bool` | `true`/`false` |
| `uint8/16/32/64` | `uintN` | `42` |
| `int8/16/32/64` | `intN` | `-42` |

## Constraints

### Range (Numbers)

```yang
leaf port {
    type uint16 {
        range "1..65535";
    }
}

leaf hold-time {
    type uint16 {
        range "0 | 3..65535";  // 0 or 3+
    }
}
```

### Length (Strings)

```yang
leaf name {
    type string {
        length "1..64";
    }
}
```

### Pattern (Regex)

```yang
leaf ipv4-address {
    type string {
        pattern '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+';
    }
}
```

### Enumeration

```yang
leaf action {
    type enumeration {
        enum allow;
        enum deny;
        enum log;
    }
}
```

## Containers and Lists

### Container (Single Instance)

```yang
container settings {
    leaf timeout { type uint32; }
    leaf retries { type uint8; }
}
```

Config:
```
settings {
    timeout 30;
    retries 3;
}
```

### List (Multiple Instances)

```yang
list endpoint {
    key "name";

    leaf name { type string; }
    leaf url { type string; }
    leaf enabled { type boolean; default true; }
}
```

Config:
```
endpoint api {
    url "https://api.example.com";
}
endpoint backup {
    url "https://backup.example.com";
    enabled false;
}
```

## Mandatory and Default

```yang
leaf required-field {
    type string;
    mandatory true;  // Must be provided
}

leaf optional-field {
    type uint32;
    default 100;     // Used if not provided
}
```

## References (leafref)

Reference values from other parts of config:

```yang
// In ze-bgp-conf module
list peer-group {
    key "name";
    leaf name { type string; }
}

// In your module - reference peer-group
leaf group {
    type leafref {
        path "/bgp/peer-group/name";
    }
}
```

ZeBGP validates that referenced values exist.

## Best Practices

1. **Use meaningful namespaces** - Include your organization
2. **Add descriptions** - Help users understand fields
3. **Set sensible defaults** - Reduce required config
4. **Validate early** - Use YANG constraints, not just code
5. **Test with ze plugin validate** - Catch errors early

## Example: Complete Schema

```yang
module acme-monitor {
    namespace "urn:acme:monitor";
    prefix acme;

    description "ACME endpoint monitoring plugin";

    container acme-monitor {
        description "Monitor configuration";

        leaf endpoint {
            type string;
            mandatory true;
            description "HTTPS endpoint to monitor";
        }

        leaf interval {
            type uint32 {
                range "10..3600";
            }
            default 60;
            description "Check interval in seconds";
        }

        list alert {
            key "name";
            description "Alert destinations";

            leaf name {
                type string {
                    length "1..64";
                }
            }

            leaf email {
                type string;
            }
        }
    }
}
```
