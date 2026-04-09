// Design: docs/architecture/core-design.md -- loop detection filter YANG schema
//
// Package schema provides the YANG schema for the loop detection filter.
package schema

import _ "embed"

//go:embed ze-loop-detection.yang
var ZeLoopDetectionYANG string
