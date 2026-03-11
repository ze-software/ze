// Design: docs/architecture/core-design.md — external format translation

package migration

import (
	_ "embed"
	"errors"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
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
	// ExaBGP uses backslash-newline for line continuation. The tokenizer's
	// auto-semicolon insertion would otherwise treat the backslash as a word
	// and insert a semicolon at the line break, prematurely terminating
	// multi-line flex entries (vpls, mup routes).
	input = strings.ReplaceAll(input, "\\\n", " ")

	schema := ExaBGPSchema()
	if schema == nil {
		return nil, ErrSchemaLoad
	}
	p := config.NewParser(schema)
	return p.Parse(input)
}
