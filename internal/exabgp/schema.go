package exabgp

import "codeberg.org/thomas-mangin/ze/internal/config"

// ExaBGPSchema returns a schema for parsing ExaBGP configuration files.
// This schema includes ExaBGP-specific constructs like:
//   - `api { processes [...]; }` inside neighbor (ExaBGP uses this, not `process`).
//   - `family { ipv4 unicast; }` with space separator.
func ExaBGPSchema() *config.Schema {
	schema := config.NewSchema()

	// Global settings.
	schema.Define("router-id", config.Leaf(config.TypeIPv4))
	schema.Define("local-as", config.Leaf(config.TypeUint32))

	// Process definitions (top-level plugins).
	schema.Define("process", config.List(config.TypeString,
		config.Field("run", config.MultiLeaf(config.TypeString)),
		config.Field("encoder", config.Leaf(config.TypeString)),
		config.Field("respawn", config.Leaf(config.TypeBool)),
	))

	// Neighbor definition (ExaBGP uses `neighbor`, not `peer`).
	schema.Define("neighbor", config.List(config.TypeIP, exabgpNeighborFields()...))

	// Template definitions.
	schema.Define("template", config.Container(
		config.Field("neighbor", config.List(config.TypeString, exabgpNeighborFields()...)),
	))

	return schema
}

// exabgpNeighborFields returns field definitions for ExaBGP neighbor block.
func exabgpNeighborFields() []config.FieldDef {
	return []config.FieldDef{
		// Basic peer identification.
		config.Field("description", config.Leaf(config.TypeString)),
		config.Field("router-id", config.Leaf(config.TypeIPv4)),
		config.Field("local-address", config.Leaf(config.TypeString)),
		config.Field("local-as", config.Leaf(config.TypeUint32)),
		config.Field("peer-as", config.Leaf(config.TypeUint32)),
		config.Field("hold-time", config.LeafWithDefault(config.TypeUint16, "90")),
		config.Field("passive", config.LeafWithDefault(config.TypeBool, "false")),
		config.Field("group-updates", config.LeafWithDefault(config.TypeBool, "true")),

		// Family (ExaBGP uses space: "ipv4 unicast").
		config.Field("family", config.Freeform()),

		// Capabilities (ExaBGP uses Flex for most).
		config.Field("capability", config.Container(
			config.Field("asn4", config.Flex()),
			config.Field("route-refresh", config.Flex()),
			config.Field("graceful-restart", config.Flex(
				config.Field("restart-time", config.LeafWithDefault(config.TypeUint16, "120")),
			)),
			config.Field("add-path", config.Flex(
				config.Field("send", config.LeafWithDefault(config.TypeBool, "false")),
				config.Field("receive", config.LeafWithDefault(config.TypeBool, "false")),
			)),
			config.Field("extended-message", config.Flex()),
			config.Field("nexthop", config.Flex()),
			config.Field("multi-session", config.Flex()),
			config.Field("operational", config.Flex()),
			config.Field("aigp", config.Flex()),
		)),

		// API block (ExaBGP-specific, renamed to `process` in ZeBGP).
		config.Field("api", config.Container(
			config.Field("processes", config.ArrayLeaf(config.TypeString)),
			config.Field("processes-match", config.ArrayLeaf(config.TypeString)),
			config.Field("neighbor-changes", config.Flex()),
			config.Field("receive", config.Freeform()),
			config.Field("send", config.Freeform()),
		)),

		// Process bindings (ZeBGP-style, may exist in migrated configs).
		config.Field("process", config.List(config.TypeString,
			config.Field("processes", config.ArrayLeaf(config.TypeString)),
			config.Field("receive", config.Freeform()),
			config.Field("send", config.Freeform()),
		)),

		// Static routes (freeform: "route PREFIX next-hop IP;").
		config.Field("static", config.Freeform()),

		// Announce block (static announcements).
		config.Field("announce", config.Freeform()),

		// L2VPN.
		config.Field("l2vpn", config.Freeform()),

		// Flow.
		config.Field("flow", config.Freeform()),

		// Nexthop encoding (RFC 8950).
		// ExaBGP syntax: nexthop { ipv4 unicast ipv6; ipv6 unicast ipv4; }
		config.Field("nexthop", config.Freeform()),
	}
}

// ParseExaBGPConfig parses an ExaBGP configuration string.
func ParseExaBGPConfig(input string) (*config.Tree, error) {
	p := config.NewParser(ExaBGPSchema())
	return p.Parse(input)
}
