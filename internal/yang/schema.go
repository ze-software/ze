package yang

import (
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// SchemaAdapter provides completion hints from YANG schema.
type SchemaAdapter struct {
	loader *Loader
}

// NewSchemaAdapter creates a new schema adapter from a loaded YANG model.
func NewSchemaAdapter(loader *Loader) *SchemaAdapter {
	return &SchemaAdapter{loader: loader}
}

// Children returns the child names of a container or list at the given path.
// Path format: "bgp" or "bgp.peer" (dot-separated).
func (s *SchemaAdapter) Children(path string) []string {
	entry := s.findEntry(path)
	if entry == nil || entry.Dir == nil {
		return nil
	}

	children := make([]string, 0, len(entry.Dir))
	for name := range entry.Dir {
		children = append(children, name)
	}
	sort.Strings(children)
	return children
}

// IsMandatory returns true if the field at path is mandatory.
func (s *SchemaAdapter) IsMandatory(path string) bool {
	entry := s.findEntry(path)
	if entry == nil {
		return false
	}
	return entry.Mandatory == yang.TSTrue
}

// TypeHint returns a type hint string for completion display.
func (s *SchemaAdapter) TypeHint(path string) string {
	entry := s.findEntry(path)
	if entry == nil || entry.Type == nil {
		return ""
	}

	return s.yangTypeToHint(entry.Type)
}

// EnumValues returns the enum values if the field is an enumeration.
func (s *SchemaAdapter) EnumValues(path string) []string {
	entry := s.findEntry(path)
	if entry == nil || entry.Type == nil {
		return nil
	}

	if entry.Type.Kind != yang.Yenum {
		return nil
	}

	if entry.Type.Enum == nil {
		return nil
	}

	return entry.Type.Enum.Names()
}

// Description returns the YANG description for a path.
func (s *SchemaAdapter) Description(path string) string {
	entry := s.findEntry(path)
	if entry == nil {
		return ""
	}
	return entry.Description
}

// IsContainer returns true if the path points to a container.
func (s *SchemaAdapter) IsContainer(path string) bool {
	entry := s.findEntry(path)
	if entry == nil {
		return false
	}
	// Container has Dir but is not a list
	return entry.Dir != nil && !entry.IsList()
}

// IsList returns true if the path points to a list.
func (s *SchemaAdapter) IsList(path string) bool {
	entry := s.findEntry(path)
	if entry == nil {
		return false
	}
	return entry.IsList()
}

// findEntry finds the YANG entry for a dot-separated path.
func (s *SchemaAdapter) findEntry(path string) *yang.Entry {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	// Map first part to module name
	moduleName := MapPrefixToModule(parts[0])

	entry := s.loader.GetEntry(moduleName)
	if entry == nil {
		entry = s.loader.GetEntry(parts[0])
	}
	if entry == nil {
		return nil
	}

	// Navigate through path parts
	current := entry
	for _, part := range parts {
		if current.Dir == nil {
			return nil
		}
		child, ok := current.Dir[part]
		if !ok {
			return nil
		}
		current = child
	}

	return current
}

// yangTypeToHint converts a YANG type to a completion hint string.
func (s *SchemaAdapter) yangTypeToHint(yangType *yang.YangType) string {
	//nolint:exhaustive // default handles unknown types
	switch yangType.Kind {
	case yang.Ystring:
		return "string"
	case yang.Yuint8:
		return "uint8"
	case yang.Yuint16:
		return "uint16"
	case yang.Yuint32:
		return "uint32"
	case yang.Yuint64:
		return "uint64"
	case yang.Yint8:
		return "int8"
	case yang.Yint16:
		return "int16"
	case yang.Yint32:
		return "int32"
	case yang.Yint64:
		return "int64"
	case yang.Ybool:
		return "boolean"
	case yang.Yenum:
		return "enum"
	case yang.Yunion:
		return "union"
	default:
		return "value"
	}
}
