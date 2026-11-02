// Design: docs/architecture/testing/ci-format.md -- record-answer test plugin
// Related: register.go -- registers the module under the zetest build
//
// Package yang provides the command tree of the record-plugin test process
// (`le mock record-plugin`). The engine judges a plugin command's arguments
// only at a path its own compiled model declares, so the model a functional
// test DUT links must carry the plugin's argument-bearing command.
package yang

import _ "embed"

// ZeRecordPluginCmdYANG declares `show record-plugin argument <id>`.
//
//go:embed ze-record-plugin-cmd.yang
var ZeRecordPluginCmdYANG string
