// Design: docs/architecture/config/yang-config-design.md -- native value restrictions.
// Overview: yang_schema.go -- YANG-to-schema conversion and scalar leaf metadata.
// Related: leaf_value.go -- memberLeaf hands these facts to ValidateLeafValue.

package config

import (
	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// yangValueLeaf reads one native type into the value restrictions a scalar or
// multi-valued node carries. Keeping this producer shared prevents a leaf-list
// from retaining a constraint's message while dropping the restriction itself.
func yangValueLeaf(entry *gyang.Entry, path string) *LeafNode {
	node := Leaf(yangTypeToValueType(entry.Type))
	node.Enums, node.EnumHelp = enumFactsFromEntry(entry, path)
	node.Format = yang.LeafValueFormat(entry, entry.Type)
	node.Ranges = numericRangesFromType(entry.Type, path)
	node.Lengths = lengthRangesFromType(entry.Type)
	node.Patterns = patternsFromType(entry.Type)
	node.Constraint = yang.LeafConstraintMessages(entry, entry.Type)
	node.Union = unionMembersFromType(entry, entry.Type)
	markDecimal64(node, entry.Type)
	return node
}

func yangToMultiLeaf(entry *gyang.Entry, path string) *MultiLeafNode {
	value := yangValueLeaf(entry, path)
	return &MultiLeafNode{
		Type: value.Type, Enums: value.Enums, EnumHelp: value.EnumHelp,
		Format: value.Format, Patterns: value.Patterns,
		Ranges: value.Ranges, Lengths: value.Lengths, Union: value.Union,
		FractionDigits: value.FractionDigits, Constraint: value.Constraint,
		ReplaceWhole: hasReplaceWholeExtension(entry),
	}
}

func yangToBracketLeafList(entry *gyang.Entry, path string) *BracketLeafListNode {
	value := yangValueLeaf(entry, path)
	return &BracketLeafListNode{
		Type: value.Type, Enums: value.Enums, EnumHelp: value.EnumHelp,
		Format: value.Format, Patterns: value.Patterns,
		Ranges: value.Ranges, Lengths: value.Lengths, Union: value.Union,
		FractionDigits: value.FractionDigits, Constraint: value.Constraint,
		Ordered:      getOrderedExtension(entry),
		ReplaceWhole: hasReplaceWholeExtension(entry),
	}
}

func yangToValueOrArray(entry *gyang.Entry, path string) *ValueOrArrayNode {
	value := yangValueLeaf(entry, path)
	return &ValueOrArrayNode{
		Type: value.Type, ValidValues: value.Enums, EnumHelp: value.EnumHelp,
		Format: value.Format, Patterns: value.Patterns,
		Ranges: value.Ranges, Lengths: value.Lengths, Union: value.Union,
		FractionDigits: value.FractionDigits, Constraint: value.Constraint,
		Ordered:      getOrderedExtension(entry),
		ReplaceWhole: hasReplaceWholeExtension(entry),
	}
}
