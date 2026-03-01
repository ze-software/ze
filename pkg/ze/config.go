// Design: docs/plan/spec-arch-0-system-boundaries.md — ConfigProvider interface

package ze

// ConfigProvider is the central authority for configuration.
//
// It loads config from files, validates against YANG schemas aggregated
// from plugins, serves config subtrees to subsystems and plugins, and
// supports save with backup. All config consumers use this interface:
// BGP subsystem, plugins, CLI editor (ze config edit), future web UI.
type ConfigProvider interface {
	// Load reads and parses a config file.
	Load(path string) error

	// Get returns the config subtree for a root name (e.g., "bgp").
	Get(root string) (map[string]any, error)

	// Validate checks the current config against the merged YANG schema.
	Validate() []error

	// Save writes the current config to a file with automatic backup.
	Save(path string) error

	// Watch returns a channel that receives notifications when
	// the config for a given root changes (e.g., on SIGHUP reload).
	Watch(root string) <-chan ConfigChange

	// Schema returns the merged YANG schema from all registered plugins.
	Schema() SchemaTree

	// RegisterSchema adds a plugin's YANG schema to the merged schema.
	RegisterSchema(name, yang string) error
}

// ConfigChange describes a configuration change notification.
type ConfigChange struct {
	// Root is the config root that changed (e.g., "bgp").
	Root string

	// Tree is the new config subtree after the change.
	Tree map[string]any
}

// SchemaTree is a validated YANG schema tree.
type SchemaTree struct {
	// Modules lists the YANG module names in this schema.
	Modules []string
}
