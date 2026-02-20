// Design: docs/architecture/core-design.md — external format translation

package exabgp

import (
	_ "embed"
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/yang"
)

// ErrSchemaLoad is returned when the ExaBGP schema fails to load.
var ErrSchemaLoad = errors.New("failed to load ExaBGP schema")

//go:embed exabgp.yang
var exabgpYANG string

// ExaBGPSchema returns a schema for parsing ExaBGP configuration files.
// Uses YANG-defined schema from exabgp.yang.
func ExaBGPSchema() *config.Schema {
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil
	}
	if err := loader.AddModuleFromText("exabgp.yang", exabgpYANG); err != nil {
		return nil
	}
	if err := loader.Resolve(); err != nil {
		return nil
	}

	schema := config.NewSchema()

	// Load exabgp module
	entry := loader.GetEntry("exabgp")
	if entry != nil {
		config.LoadYANGModule(schema, entry)
	}

	return schema
}

// ParseExaBGPConfig parses an ExaBGP configuration string.
func ParseExaBGPConfig(input string) (*config.Tree, error) {
	schema := ExaBGPSchema()
	if schema == nil {
		return nil, ErrSchemaLoad
	}
	p := config.NewParser(schema)
	return p.Parse(input)
}
